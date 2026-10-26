package apextest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestJSONCircularReferencesNativeConformance(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "json_circular_references")
	data, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if lines[0] != "api\tshape\toperation\tobservation" || len(lines) != 93 {
		t.Fatal("incomplete native cycle observations")
	}
	source, err := os.ReadFile(filepath.Join(dir, "classes", "JsonCycleObservation.cls"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"."}],"sourceApiVersion":"`+api+`"}`)
			writeFile(t, filepath.Join(root, "classes", "JsonCycleObservation.cls"), string(source))
			writeFile(t, filepath.Join(root, "classes", "JsonCycleObservation.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native source rejected: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			seen := map[string]bool{}
			for _, line := range lines[1:] {
				row := strings.SplitN(line, "\t", 4)
				if len(row) != 4 {
					t.Fatal("invalid native row")
				}
				if row[0] != api {
					continue
				}
				shape, operation := row[1], row[2]
				// Display, clone, equality and hash observations are retained in
				// the oracle. Their behavior belongs to the collection APIs.
				switch operation {
				case "serialize", "pretty", "suppress", "generator", "generatorField":
				default:
					continue
				}
				key := shape + "/" + operation
				if seen[key] {
					t.Fatal("duplicate native row")
				}
				seen[key] = true
				t.Run(key, func(t *testing.T) {
					program, err := vm.CompileAnonymousWithOptions("System.debug(JsonCycleObservation.observe('"+shape+"','"+operation+"'));", vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					result, err := runner.newMachine().Execute(program)
					if err != nil {
						t.Fatal(err)
					}
					var got, want map[string]any
					if len(result.Debug) != 1 || json.Unmarshal([]byte(result.Debug[0]), &got) != nil || json.Unmarshal([]byte(row[3]), &want) != nil {
						t.Fatalf("invalid observation: %v", result.Debug)
					}
					// Pretty whitespace is outside the graph-serialization contract.
					if actual, ok := got["value"].(string); ok {
						if expected, ok := want["value"].(string); ok {
							var a, b any
							if json.Unmarshal([]byte(actual), &a) == nil && json.Unmarshal([]byte(expected), &b) == nil && reflect.DeepEqual(a, b) {
								got["value"] = expected
							}
						}
					}
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("native=%s; Glade=%s", row[3], result.Debug[0])
					}
				})
			}
			t.Run("diagnostics", func(t *testing.T) {
				source, err := os.ReadFile(filepath.Join(dir, "diagnostics.apex"))
				if err != nil {
					t.Fatal(err)
				}
				observations, err := os.ReadFile(filepath.Join(dir, "diagnostics.tsv"))
				if err != nil {
					t.Fatal(err)
				}
				program, err := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				result, err := runner.newMachine().Execute(program)
				if err != nil {
					t.Fatal(err)
				}
				actual := map[string]map[string]any{}
				for _, line := range result.Debug {
					fields := strings.SplitN(strings.TrimPrefix(line, "JSON_CYCLE|"), "|", 3)
					if len(fields) != 3 {
						t.Fatal("invalid diagnostic observation")
					}
					var row map[string]any
					if err := json.Unmarshal([]byte(fields[2]), &row); err != nil {
						t.Fatal(err)
					}
					actual[fields[0]+"/"+fields[1]] = row
				}
				count := 0
				for _, line := range strings.Split(strings.TrimSpace(string(observations)), "\n") {
					fields := strings.SplitN(line, "\t", 4)
					if len(fields) != 4 {
						t.Fatal("invalid diagnostic oracle")
					}
					if fields[0] != api {
						continue
					}
					count++
					t.Run(fields[1]+"/"+fields[2], func(t *testing.T) {
						var want map[string]any
						if err := json.Unmarshal([]byte(fields[3]), &want); err != nil {
							t.Fatal(err)
						}
						if got := actual[fields[1]+"/"+fields[2]]; !reflect.DeepEqual(got, want) {
							t.Fatalf("native=%v; Glade=%v", want, got)
						}
					})
				}
				if count != 6 || len(actual) != 6 {
					t.Fatal("incomplete diagnostic observations")
				}
			})
			if len(seen) != 35 {
				t.Fatalf("missing serializer observations: %d", len(seen))
			}
		})
	}
}
