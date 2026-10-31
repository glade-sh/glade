package apextest

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

// Compare the captured strings themselves: decoding the inner SObject JSON
// would discard the field order and pretty-print whitespace under test.
func TestSObjectJSON2NativeConformance(t *testing.T) {
	runSObjectJSON2NativeConformance(t, "testdata/conformance/sobject_json2", "SObjectJSON2Probe", "SOBJECT_JSON2|", "", "multiParent multiParentReverse multiParentNull polyParents polyParentsReverse polyNullWho typeofAccount typeofOpportunity typeofWho fieldsStandard fieldsAll fieldsCustom toLabel toLabelAlias format formatAlias address addressReverse addressComponents mailingAddress location locationReverse locationComponents unknownField emptyBoundary")
}

func TestSObjectJSON2NamespaceNativeConformance(t *testing.T) {
	runSObjectJSON2NativeConformance(t, "testdata/conformance/sobject_json2_namespace", "SObjectJSON2NamespaceProbe", "SOBJECT_JSON2_NAMESPACE|", "gladens002", "constructed queried empty rejection")
}

func TestSObjectJSON2RouteContextNativeConformance(t *testing.T) {
	runSObjectJSON2NativeConformance(t, "testdata/conformance/sobject_json2_route_context", "SObjectJSON2RouteProbe", "SOBJECT_JSON2_ROUTE|", "gladens002", "context")
}

func TestSObjectJSON2DecoderNativeConformance(t *testing.T) {
	runSObjectJSON2NativeConformance(t, "testdata/conformance/sobject_json2_decoders", "SObjectJSON2DecoderProbe", "SOBJECT_JSON2_DECODER|", "", "currencyNumber currencyFormatted currencyNull address addressEmpty location locationEmpty nullRecord", "concrete_deserialize", "concrete_strict", "concrete_parser", "concrete_parserStrict", "sobject_deserialize", "sobject_strict", "sobject_parser", "sobject_parserStrict")
}

func runSObjectJSON2NativeConformance(t *testing.T, fixture, probe, prefix, namespace, shapes string, operations ...string) {
	language := ""
	if probe == "SObjectJSON3Probe" || probe == "SObjectJSON3Routes" {
		language = "fr"
	}
	runSObjectJSON2NativeConformanceWithLanguage(t, fixture, probe, prefix, namespace, shapes, language, operations...)
}

func runSObjectJSON2NativeConformanceWithLanguage(t *testing.T, fixture, probe, prefix, namespace, shapes, language string, operations ...string) {
	native, runtimeAPI := readSObjectJSON2NativeRows(t, fixture, shapes, operations...)
	releaseRuntimeCachesAfterTest(t)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			if err := os.CopyFS(root, os.DirFS(filepath.Join(fixture, "project"))); err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"namespace":%q,"packageDirectories":[{"path":".","default":true}]}`, api, namespace))
			writeFile(t, filepath.Join(root, "classes/"+probe+".cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			named := strings.ReplaceAll(sobjectJSON2NativeTestClass(native[api]["isTest"]), "SObjectJSON2Probe", probe)
			if language != "" {
				named = strings.Replace(named, "@IsTest static void capturedRows() {", "@IsTest static void capturedRows() { System.runAs(new User(Id='005000000000009', Name='Synthetic', LastName='Synthetic', LocaleSidKey='fr_FR', LanguageLocaleKey='"+language+"')) {", 1)
				named = strings.TrimSuffix(named, "}\n}\n") + "}\n}\n}\n"
			}
			if probe == "SObjectJSON2RouteProbe" {
				named = strings.ReplaceAll(named, probe+".observe()", "new Map<String,Map<String,String>>{'context'=>(Map<String,String>)JSON.deserialize("+probe+".observe(),Map<String,String>.class)}")
			}
			writeFile(t, filepath.Join(root, "classes/"+probe+"Test.cls"), named)
			writeFile(t, filepath.Join(root, "classes/"+probe+"Test.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native project accepted; Glade diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
			}
			t.Run("anonymous", func(t *testing.T) {
				source := probe + ".emit();"
				if namespace != "" {
					source = namespace + "." + source
				}
				if probe == "SObjectJSON2RouteProbe" {
					source = "System.debug(LoggingLevel.ERROR, '" + prefix + "context|' + " + namespace + "." + probe + ".observe());"
				}
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
				result, err := runner.execute(program, func(machine *vm.VM) {
					machine.SetCurrentNamespace(namespace)
					if language != "" {
						machine.SetCurrentUser(storage.Record{ID: storage.ID(storage.DefaultUserID), Object: "User", Fields: map[string]storage.Value{"LocaleSidKey": storage.StringValue("fr_FR"), "LanguageLocaleKey": storage.StringValue(language)}})
					}
				})
				if err != nil {
					t.Fatal(err)
				}
				observed := make(map[string]map[string]string)
				for _, line := range result.Debug {
					if !strings.HasPrefix(line, prefix) {
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
						SelectedClasses:       []string{probe + "Test"},
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

func sobjectJSON2NativeTestClass(shapes sobjectJSONNativeShapes) string {
	var source strings.Builder
	source.WriteString("@IsTest private class SObjectJSON2ProbeTest {\n@IsTest static void capturedRows() {\nMap<String,Map<String,String>> actual = SObjectJSON2Probe.observe();\nList<String> failures = new List<String>();\n")
	fmt.Fprintf(&source, "System.assertEquals(%d, actual.size(), 'shape count');\n", len(shapes))
	for _, shape := range sobjectJSONSortedKeys(shapes) {
		shapeLiteral := conformanceApexString(shape)
		fmt.Fprintf(&source, "System.assert(actual.containsKey(%s), %s);\n", shapeLiteral, shapeLiteral)
		fmt.Fprintf(&source, "System.assertEquals(%d, actual.get(%s).size(), %s);\n", len(shapes[shape]), shapeLiteral, shapeLiteral)
		for _, operation := range sobjectJSONSortedKeys(shapes[shape]) {
			cell := shapes[shape][operation]
			fmt.Fprintf(&source, "if (!%s.equals(actual.get(%s).get(%s))) failures.add(%s + ': ' + actual.get(%s).get(%s));\n", conformanceApexString(cell.value), shapeLiteral, conformanceApexString(operation), conformanceApexString(fmt.Sprintf("%s/%s org.tsv:%d", shape, operation, cell.line)), shapeLiteral, conformanceApexString(operation))
		}
	}
	source.WriteString("System.assertEquals(0, failures.size(), String.join(failures, '\\n'));\n}\n}\n")
	return source.String()
}

func readSObjectJSON2NativeRows(t *testing.T, fixture, shapes string, operations ...string) (map[string]map[string]sobjectJSONNativeShapes, string) {
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
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		if line == 1 {
			if scanner.Text() != "api\troute\tshape\toperation\tvalue" {
				t.Fatal("invalid oracle header")
			}
			continue
		}
		row := strings.SplitN(scanner.Text(), "\t", 5)
		if len(row) != 5 || (row[0] != "62.0" && row[0] != "67.0") || (row[1] != "anonymous" && row[1] != "isTest") {
			t.Fatalf("invalid org.tsv:%d", line)
		}
		var value string
		if err := json.Unmarshal([]byte(row[4]), &value); err != nil {
			t.Fatal(err)
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
			t.Fatalf("duplicate org.tsv:%d", line)
		}
		values[row[3]] = sobjectJSONNativeCell{value: value, line: line}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "isTest"} {
			for _, shape := range strings.Fields(shapes) {
				values := native[api][route][shape]
				if _, rejected := values["query"]; rejected {
					if len(values) != 1 {
						t.Fatal("mixed query outcome")
					}
					continue
				}
				if shape == "context" {
					if len(values) != 9 {
						t.Fatalf("incomplete route context: %s/%s", api, route)
					}
					continue
				}
				expectedOperations := operations
				if len(expectedOperations) == 0 {
					expectedOperations = []string{"serialize", "pretty", "suppress", "prettySuppress", "roundTrip"}
				}
				for _, op := range expectedOperations {
					if shape == "empty" && op == "roundTrip" {
						continue
					}
					if _, ok := values[op]; !ok {
						t.Fatalf("missing %s/%s/%s/%s", api, route, shape, op)
					}
				}
			}
		}
	}
	return native, "68.0"
}
