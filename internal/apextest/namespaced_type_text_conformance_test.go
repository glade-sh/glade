package apextest

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func namespacedNativeObservation(t *testing.T, api, route, id string) string {
	t.Helper()
	file, err := os.Open("testdata/conformance/namespaced_type_text/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		if len(row) != 4 {
			t.Fatal("invalid native row", row)
		}
		if row[0] == api && row[1] == route && row[2] == id {
			return row[3]
		}
	}
	t.Fatalf("missing native row %s/%s/%s", api, route, id)
	return ""
}

// R001-R012 cover the named @IsTest route independently at each source API.
func TestNamespacedTypeTextOrgConformance(t *testing.T) {
	var data struct {
		Namespace   string
		APIVersions []string
		Route       string
		Sources     map[string]string
		Metadata    map[string]string
		Cases       []struct {
			ID, Question, TestClass, Source string
			ExpectedRuntime                 map[string]*string
		}
	}
	raw, err := os.ReadFile("testdata/conformance/namespaced_type_text.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data.Namespace != "nsx" || strings.Join(data.APIVersions, ",") != "62.0,67.0" || data.Route != "isTest" || len(data.Cases) != 12 || len(data.Sources) != 4 || len(data.Metadata) != 4 {
		t.Fatal("incomplete namespaced type-text case matrix")
	}
	for i, row := range data.Cases {
		terminal := "System.assert(false, 'P|" + row.ID + "|' + String.valueOf(value));"
		if row.ID != fmt.Sprintf("R%03d", i+1) || row.Question == "" || row.TestClass == "" || strings.Count(row.Source, terminal) != 1 || len(row.ExpectedRuntime) != len(data.APIVersions) {
			t.Fatalf("invalid namespaced type-text case: %s", row.ID)
		}
		for _, api := range data.APIVersions {
			if _, ok := row.ExpectedRuntime[api]; !ok {
				t.Fatalf("missing native expectation slot: %s@%s", row.ID, api)
			}
		}
	}
	for _, api := range data.APIVersions {
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			for _, row := range data.Cases {
				t.Run(row.ID, func(t *testing.T) {
					expected := row.ExpectedRuntime[api]
					if expected == nil {
						t.Fatalf("missing reviewed native twin %s@%s (%s)", row.ID, api, row.Question)
					}
					if *expected != namespacedNativeObservation(t, api, "isTest", row.ID) {
						t.Fatal("JSON expectation differs from captured native row")
					}
					terminal := "System.assert(false, 'P|" + row.ID + "|' + String.valueOf(value));"
					assertion := "System.assertEquals(" + conformanceApexString(*expected) + ", String.valueOf(value), '" + row.ID + "');"
					root := t.TempDir()
					config, err := json.Marshal(map[string]any{
						"namespace": data.Namespace, "sourceApiVersion": api,
						"packageDirectories": []map[string]any{{"path": "force-app", "default": true}},
					})
					if err != nil {
						t.Fatal(err)
					}
					writeFile(t, filepath.Join(root, "sfdx-project.json"), string(config))
					base := filepath.Join(root, "force-app", "main", "default")
					writeClass := func(name, code string) {
						writeFile(t, filepath.Join(base, "classes", name+".cls"), code)
						writeFile(t, filepath.Join(base, "classes", name+".cls-meta.xml"), "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
					}
					for name, code := range data.Sources {
						writeClass(name, code)
					}
					writeClass(row.TestClass, strings.Replace(row.Source, terminal, assertion, 1))
					for path, code := range data.Metadata {
						writeFile(t, filepath.Join(base, filepath.FromSlash(path)), code)
					}
					index := loadTestIndex(t, root)
					if result := sema.Analyze(index); result.HasErrors() {
						t.Fatalf("local assertion semantics: %#v", result.Diagnostics)
					}
					run := Run(index, Options{NoDiskCache: true, Parallelism: 1})
					if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
						t.Fatalf("native <%s>: summary=%#v problem=%q", *expected, got, firstRunProblem(run))
					}
				})
			}
		})
	}
}

// T001 pins the anonymous provider/cast/global-map routes and both map-based
// field-set lookups to native twins, using the complete synthetic project.
func TestNamespacedTypeTextAnonymousProjectNativeRoutes(t *testing.T) {
	var data struct{ Sources, Metadata map[string]string }
	raw, err := os.ReadFile("testdata/conformance/namespaced_type_text.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	for _, cell := range []struct{ id, file string }{{"T001", "anonymous.apex"}, {"T002", "lookup-controls.apex"}, {"T003", "event-controls.apex"}} {
		body, err := os.ReadFile("testdata/conformance/namespaced_type_text/" + cell.file)
		if err != nil {
			t.Fatal(err)
		}
		for _, api := range []string{"62.0", "67.0"} {
			t.Run(api+"/anonymous/"+cell.id, func(t *testing.T) {
				root := t.TempDir()
				writeNamespacedFieldSetProject(t, root, api)
				base := filepath.Join(root, "force-app/main/default")
				for name, code := range data.Sources {
					writeFile(t, filepath.Join(base, "classes", name+".cls"), code)
					writeFile(t, filepath.Join(base, "classes", name+".cls-meta.xml"), "<ApexClass><apiVersion>67.0</apiVersion></ApexClass>")
				}
				for path, code := range data.Metadata {
					writeFile(t, filepath.Join(base, filepath.FromSlash(path)), code)
				}
				if cell.id == "T003" {
					metadata, readErr := os.ReadFile("testdata/conformance/namespaced_type_text/event-object.xml")
					if readErr != nil {
						t.Fatal(readErr)
					}
					writeFile(t, filepath.Join(base, "objects/TypeTextEventItem__c/TypeTextEventItem__c.object-meta.xml"), string(metadata))
				}
				expected := namespacedNativeObservation(t, api, "anonymous", cell.id)
				terminal := "System.assert(false, 'P|" + cell.id + "|' + JSON.serialize(observations));"
				var values map[string]any
				if err := json.Unmarshal([]byte(expected), &values); err != nil {
					t.Fatal(err)
				}
				assertions := ""
				for key, value := range values {
					literal := fmt.Sprint(value)
					if text, ok := value.(string); ok {
						literal = conformanceApexString(text)
					}
					assertions += "System.assertEquals(" + literal + ", observations.get(" + conformanceApexString(key) + "), " + conformanceApexString(key) + ");\n"
				}
				source := strings.Replace(string(body), terminal, assertions, 1)
				index := loadTestIndex(t, root)
				if result := sema.AnalyzeAnonymous(index, source, api); result.HasErrors() {
					t.Fatal(result.Diagnostics)
				}
				program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				org.APIVersion = api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				if _, err := runner.execute(program, func(machine *vm.VM) { machine.SetCurrentNamespace("nsx") }); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
