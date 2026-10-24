package apextest

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestPersonAccountFormulaInputNativeConformance(t *testing.T) {
	for _, route := range []string{"statement", "database", "upsert", "typed_interview"} {
		t.Run(route, func(t *testing.T) { personAccountFormulaInputNativeConformance(t, route) })
	}
}

func personAccountFormulaInputNativeConformance(t *testing.T, mutationRoute string) {
	const fixture = "testdata/conformance/person_account_formula_inputs/"
	raw, err := os.ReadFile(fixture + "cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases struct {
		Files     map[string]string
		Anonymous string
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(fixture + "org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	wants := map[string]map[string]any{}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
		row := strings.SplitN(line, "\t", 4)
		if len(row) != 4 {
			t.Fatal(line)
		}
		if row[2] == "static_rejection" {
			continue
		}
		key := row[0] + "/" + row[1]
		if wants[key] == nil {
			wants[key] = map[string]any{}
		}
		var value any
		if err := json.Unmarshal([]byte(row[3]), &value); err != nil {
			t.Fatal(err)
		}
		if _, ok := wants[key][row[2]]; ok {
			t.Fatal("duplicate native row")
		}
		wants[key][row[2]] = value
	}
	if len(wants) != 4 {
		t.Fatalf("native routes=%d", len(wants))
	}
	for _, api := range []string{"62.0", "67.0"} {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
		writeFile(t, filepath.Join(root, "glade.yml"), "org:\n  features: [PersonAccounts]\n")
		for name, body := range cases.Files {
			if strings.HasSuffix(name, ".cls") {
				switch mutationRoute {
				case "typed_interview":
					body = regexp.MustCompile(`Flow\.Interview\.createInterview\('([^']+)',`).ReplaceAllString(body, "new Flow.Interview.$1(")
				case "database":
					body = regexp.MustCompile(`\binsert (\w+);`).ReplaceAllString(body, "Database.insert($1);")
					body = regexp.MustCompile(`\bupdate (\w+);`).ReplaceAllString(body, "Database.update($1);")
				case "upsert":
					body = regexp.MustCompile(`\b(insert|update) (\w+);`).ReplaceAllString(body, "upsert $2;")
				}
			}
			writeFile(t, filepath.Join(root, name), strings.ReplaceAll(body, "62.0", api))
		}
		index := loadTestIndex(t, root)
		for _, route := range []string{"anonymous", "istest"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				want := wants[api+"/"+route]
				if len(want) != 11 {
					t.Fatalf("native rows=%d", len(want))
				}
				if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
					t.Fatalf("native declarations accepted; Glade: %v %v", index.Diagnostics, analysis.Diagnostics)
				}
				var message string
				if route == "anonymous" {
					if analysis := sema.AnalyzeAnonymous(index, cases.Anonymous, api); analysis.HasErrors() {
						t.Fatal(analysis.Diagnostics)
					}
					program, err := vm.CompileAnonymousWithOptions(cases.Anonymous, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					org := orgFromIndex(index)
					org.APIVersion = api
					runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
					_, err = runner.newMachine().Execute(program)
					var observation *vm.RuntimeError
					if !errors.As(err, &observation) || flowRefsExceptionType(observation.Type) != "System.AssertException" {
						t.Fatalf("missing observation: %v", err)
					}
					message = observation.ExceptionMessage()
				} else {
					run := Run(index, Options{SelectedClasses: []string{"PersonFormulaInputProbeTest"}, SelectedMethod: "observeFields", NoDiskCache: true, Parallelism: 1})
					if run.Summary().Total != 1 || len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
						t.Fatalf("unexpected selection: %+v", run.Summary())
					}
					result := run.Suites[0].Cases[0]
					if result.Problem == nil || flowRefsExceptionType(result.Problem.Type) != "System.AssertException" {
						t.Fatalf("missing observation: %+v", result)
					}
					message = result.Problem.Message
				}
				_, payload, found := strings.Cut(message, "PERSON_FORMULA|")
				var got map[string]any
				if !found || json.Unmarshal([]byte(payload), &got) != nil || len(got) != len(want) {
					t.Fatalf("invalid observation: %s", message)
				}
				for id, value := range want {
					t.Run(id, func(t *testing.T) {
						if !reflect.DeepEqual(got[id], value) {
							t.Fatalf("native=%v; Glade=%v", value, got[id])
						}
					})
				}
			})
		}
	}
}
