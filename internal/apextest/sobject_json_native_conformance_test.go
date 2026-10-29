package apextest

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

type sobjectJSONNativeCell struct {
	value string
	line  int
}

type sobjectJSONNativeShapes map[string]map[string]sobjectJSONNativeCell

// Compare the captured strings themselves: decoding the inner SObject JSON
// would discard the field order and pretty-print whitespace under test.
func TestSObjectJSONNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/sobject_json"
	native, runtimeAPI := readSObjectJSONNativeRows(t, fixture)
	releaseRuntimeCachesAfterTest(t)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			if err := os.CopyFS(root, os.DirFS(filepath.Join(fixture, "project"))); err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":".","default":true}]}`, api))
			writeFile(t, filepath.Join(root, "classes/SObjectJSONProbe.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			writeFile(t, filepath.Join(root, "classes/SObjectJSONProbeTest.cls"), sobjectJSONNativeTestClass(native[api]["isTest"]))
			writeFile(t, filepath.Join(root, "classes/SObjectJSONProbeTest.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native project accepted; Glade diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
			}
			t.Run("anonymous", func(t *testing.T) {
				const source = "SObjectJSONProbe.emit();"
				if analysis := sema.AnalyzeAnonymous(index, source, api); analysis.HasErrors() {
					t.Fatal(analysis.Diagnostics)
				}
				program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				org.APIVersion = runtimeAPI
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				result, err := runner.execute(program)
				if err != nil {
					t.Fatal(err)
				}
				observed := make(map[string]map[string]string)
				for _, line := range result.Debug {
					if !strings.HasPrefix(line, "SOBJECT_JSON|") {
						continue
					}
					parts := strings.SplitN(line, "|", 3)
					if len(parts) != 3 || observed[parts[1]] != nil {
						t.Fatalf("invalid or duplicate observation: %s", line)
					}
					var values map[string]string
					if err := json.Unmarshal([]byte(parts[2]), &values); err != nil {
						t.Fatalf("invalid observation: %s: %v", line, err)
					}
					observed[parts[1]] = values
				}
				want := native[api]["anonymous"]
				if len(observed) != len(want) {
					t.Errorf("observed %d shapes, native has %d", len(observed), len(want))
				}
				for _, shape := range sobjectJSONSortedKeys(want) {
					t.Run(shape, func(t *testing.T) {
						if len(observed[shape]) != len(want[shape]) {
							t.Errorf("observed %d operations, native has %d", len(observed[shape]), len(want[shape]))
						}
						for _, operation := range sobjectJSONSortedKeys(want[shape]) {
							t.Run(operation, func(t *testing.T) {
								cell := want[shape][operation]
								got, present := observed[shape][operation]
								if !present || got != cell.value {
									t.Fatalf("org.tsv:%d native=%q; Glade=%q (present=%t)", cell.line, cell.value, got, present)
								}
							})
						}
					})
				}
			})
			// Both executions use the product runner. The second must reuse the
			// first runtime while starting with fresh records and static state.
			InvalidateRuntimeCaches()
			for _, mode := range []string{"cold", "warm"} {
				t.Run("isTest/"+mode, func(t *testing.T) {
					ResetPerfCounters()
					run := Run(index, Options{
						SelectedClasses:       []string{"SObjectJSONProbeTest"},
						RuntimeRESTAPIVersion: runtimeAPI,
						NoDiskCache:           true,
						Parallelism:           1,
						PerfCounters:          true,
					})
					if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 || summary.Failed != 0 || summary.Errors != 0 || summary.Skipped != 0 {
						t.Errorf("named native rows failed: %+v; %s", summary, firstRunProblem(run))
					}
					if phases := SnapshotPerfCounters().Phases; mode == "warm" && (phases.MemoryCacheHits != 1 || phases.CacheMisses != 0) {
						t.Errorf("warm route did not reuse its runtime: %+v", phases)
					}
				})
			}
		})
	}
}

func readSObjectJSONNativeRows(t *testing.T, fixture string) (map[string]map[string]sobjectJSONNativeShapes, string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(fixture, "org.tsv")); err != nil {
		t.Fatalf("missing native observations: %v", err)
	}
	file, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	native := make(map[string]map[string]sobjectJSONNativeShapes)
	urlVersion := regexp.MustCompile(`/services/data/v([0-9]+\.0)/sobjects/`)
	runtimeAPI := ""
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		if line == 1 {
			if scanner.Text() != "api\troute\tshape\toperation\tvalue" {
				t.Fatal("invalid SObject JSON oracle header")
			}
			continue
		}
		row := strings.SplitN(scanner.Text(), "\t", 5)
		if len(row) != 5 || (row[0] != "62.0" && row[0] != "67.0") || (row[1] != "anonymous" && row[1] != "isTest") || row[2] == "" || row[3] == "" {
			t.Fatalf("invalid org.tsv:%d", line)
		}
		var value string
		if err := json.Unmarshal([]byte(row[4]), &value); err != nil {
			t.Fatalf("org.tsv:%d value must be a JSON string: %v", line, err)
		}
		if native[row[0]] == nil {
			native[row[0]] = make(map[string]sobjectJSONNativeShapes)
		}
		if native[row[0]][row[1]] == nil {
			native[row[0]][row[1]] = make(sobjectJSONNativeShapes)
		}
		if native[row[0]][row[1]][row[2]] == nil {
			native[row[0]][row[1]][row[2]] = make(map[string]sobjectJSONNativeCell)
		}
		values := native[row[0]][row[1]][row[2]]
		if _, exists := values[row[3]]; exists {
			t.Fatalf("duplicate org.tsv:%d observation", line)
		}
		values[row[3]] = sobjectJSONNativeCell{value: value, line: line}
		for _, match := range urlVersion.FindAllStringSubmatch(value, -1) {
			if runtimeAPI != "" && runtimeAPI != match[1] {
				t.Fatalf("capture has inconsistent runtime URL versions: %s and %s", runtimeAPI, match[1])
			}
			runtimeAPI = match[1]
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if runtimeAPI == "" {
		t.Fatal("native record URLs do not identify the runtime REST API")
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "isTest"} {
			shapes := native[api][route]
			for _, shape := range strings.Fields("empty constructed reversed putOverwrite nullId assignedId15 assignedId18 inserted queryNameId queryIdName queryFields queryReversed queryImplicitId queryThenWrite constructedParent queryParent queryNullParent queryChildren queryEmptyChildren customConstructed customInserted customQuery customQueryReversed customParent deserialized deserializedReversed emptyList") {
				for _, operation := range []string{"serialize", "pretty", "suppress", "prettySuppress", "roundTrip"} {
					if _, ok := shapes[shape][operation]; !ok {
						t.Fatalf("missing native %s/%s/%s/%s", api, route, shape, operation)
					}
				}
			}
			for _, shape := range []string{"missingAttributes", "unknownField", "strictUnknown", "invalidId", "nullBoundary"} {
				if _, ok := shapes[shape]["deserialize"]; !ok {
					t.Fatalf("missing native %s/%s/%s/deserialize", api, route, shape)
				}
			}
		}
	}
	return native, runtimeAPI
}

func sobjectJSONNativeTestClass(shapes sobjectJSONNativeShapes) string {
	var source strings.Builder
	source.WriteString("@IsTest private class SObjectJSONProbeTest {\n@IsTest static void capturedRows() {\nMap<String,Map<String,String>> actual = SObjectJSONProbe.observe();\n")
	fmt.Fprintf(&source, "System.assertEquals(%d, actual.size(), 'shape count');\n", len(shapes))
	for _, shape := range sobjectJSONSortedKeys(shapes) {
		shapeLiteral := conformanceApexString(shape)
		fmt.Fprintf(&source, "System.assert(actual.containsKey(%s), %s);\n", shapeLiteral, shapeLiteral)
		fmt.Fprintf(&source, "System.assertEquals(%d, actual.get(%s).size(), %s);\n", len(shapes[shape]), shapeLiteral, shapeLiteral)
		for _, operation := range sobjectJSONSortedKeys(shapes[shape]) {
			cell := shapes[shape][operation]
			fmt.Fprintf(&source, "System.assertEquals(%s, actual.get(%s).get(%s), %s);\n", conformanceApexString(cell.value), shapeLiteral, conformanceApexString(operation), conformanceApexString(fmt.Sprintf("%s/%s org.tsv:%d", shape, operation, cell.line)))
		}
	}
	source.WriteString("}\n}\n")
	return source.String()
}

func sobjectJSONSortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
