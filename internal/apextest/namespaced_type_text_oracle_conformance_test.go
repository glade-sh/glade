package apextest

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestNamespacedTypeTextOracleNativeRoutes(t *testing.T) {
	const fixture = "testdata/namespaced_type_text_oracle"
	read := func(name string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(fixture, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	var data struct {
		Namespace         string
		Sources, Metadata map[string]string
		Cases             []struct{ ID, TestClass, Source string }
	}
	if err := json.Unmarshal([]byte(read("cases.json")), &data); err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(read("org.tsv")))
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 35 || strings.Join(rows[0], ",") != "apiVersion,route,id,observation" {
		t.Fatalf("incomplete native capture: %v", err)
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		if len(row) != 4 || (row[0] != "62.0" && row[0] != "67.0") {
			t.Fatalf("invalid native row: %v", row)
		}
		api, route, id, expected := row[0], row[1], row[2], row[3]
		key := api + "/" + route + "/" + id
		if seen[key] {
			t.Fatalf("duplicate native row: %s", key)
		}
		seen[key] = true
		t.Run(key, func(t *testing.T) {
			root := t.TempDir()
			config, err := json.Marshal(map[string]any{
				"namespace": data.Namespace, "sourceApiVersion": api,
				"packageDirectories": []map[string]any{{"path": "force-app", "default": true}},
			})
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "sfdx-project.json"), string(config))
			base := filepath.Join(root, "force-app/main/default")
			classAPI := api
			if route == "anonymous" {
				classAPI = "67.0"
			}
			writeClass := func(name, source string) {
				path := filepath.Join(base, "classes", name+".cls")
				writeFile(t, path, source)
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+classAPI+"</apiVersion></ApexClass>")
			}
			for name, source := range data.Sources {
				writeClass(name, source)
			}
			for path, source := range data.Metadata {
				writeFile(t, filepath.Join(base, filepath.FromSlash(path)), source)
			}
			if route == "isTest" {
				class, source, expression := "FieldSetPathTest", "", "String.join(values, ';')"
				if id == "F001" {
					source = read("fieldset.isTest.cls")
				} else {
					for _, cell := range data.Cases {
						if cell.ID == id {
							class, source, expression = cell.TestClass, cell.Source, "String.valueOf(value)"
						}
					}
				}
				terminal := "System.assert(false, 'P|" + id + "|' + " + expression + ");"
				if strings.Count(source, terminal) != 1 {
					t.Fatal("missing captured named transport", id)
				}
				assertion := "System.assertEquals(" + conformanceApexString(expected) + ", " + expression + ", '" + id + "');"
				writeClass(class, strings.Replace(source, terminal, assertion, 1))
				run := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{class}, NoDiskCache: true, Parallelism: 1})
				if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
					t.Fatalf("native <%s>; summary=%#v problem=%q", expected, got, firstRunProblem(run))
				}
				return
			}
			if route != "anonymous" {
				t.Fatal("unknown native route", route)
			}
			file, expression := "fieldset.anonymous.apex", "String.join(values, ';')"
			assertions := "System.assertEquals(" + conformanceApexString(expected) + ", " + expression + ", '" + id + "');"
			if id == "T001" || id == "T002" || id == "T003" {
				file, expression = "type-text.anonymous.apex", "JSON.serialize(observations)"
				if id == "T002" {
					file = "lookup-controls.anonymous.apex"
				}
				if id == "T003" {
					file = "event-controls.anonymous.apex"
					writeFile(t, filepath.Join(base, "objects/TypeTextEventItem__c/TypeTextEventItem__c.object-meta.xml"), read("event-object.xml"))
				}
				var values map[string]any
				if err := json.Unmarshal([]byte(expected), &values); err != nil {
					t.Fatal(err)
				}
				keys := make([]string, 0, len(values))
				for key := range values {
					keys = append(keys, key)
				}
				slices.Sort(keys)
				assertions = ""
				for _, key := range keys {
					literal := fmt.Sprint(values[key])
					if value, ok := values[key].(string); ok {
						literal = conformanceApexString(value)
					}
					assertions += "System.assertEquals(" + literal + ", observations.get(" + conformanceApexString(key) + "), " + conformanceApexString(key) + ");\n"
				}
			}
			source := read(file)
			terminal := "System.assert(false, 'P|" + id + "|' + " + expression + ");"
			if strings.Count(source, terminal) != 1 {
				t.Fatal("missing captured anonymous transport", id)
			}
			source = strings.Replace(source, terminal, assertions, 1)
			index := loadTestIndex(t, root)
			if analysis := sema.AnalyzeAnonymous(index, source, api); analysis.HasErrors() {
				t.Fatal(analysis.Diagnostics)
			}
			program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			if _, err := runner.execute(program, func(machine *vm.VM) { machine.SetCurrentNamespace(data.Namespace) }); err != nil {
				t.Fatal(err)
			}
		})
	}
}
