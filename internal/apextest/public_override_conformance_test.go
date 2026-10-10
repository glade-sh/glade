package apextest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Separate fixture/decoy/test units preserve the captured declaring scopes.
// Only the terminal observation assertion is replaced for local execution.
func TestPublicOverrideOrgConformance(t *testing.T) {
	var data struct {
		APIVersions  []string
		AbsentRoutes []struct{ Route, Reason string }
		Rows         []struct {
			ID, APIVersion, Route, ExpectedCompilation, ExpectedRuntime string
			Sources                                                     map[string]string
		}
	}
	raw, err := os.ReadFile("testdata/conformance/public_override.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0" || len(data.Rows) != 16 {
		t.Fatalf("native matrix shape: APIs=%v rows=%d", data.APIVersions, len(data.Rows))
	}
	if len(data.AbsentRoutes) != 1 || data.AbsentRoutes[0].Route != "anonymous" || !strings.Contains(data.AbsentRoutes[0].Reason, "Inner types are not allowed to have inner types") {
		t.Fatal("missing anonymous route boundary")
	}
	build := func(t *testing.T, api string, sources map[string]string) typesys.Index {
		t.Helper()
		root := t.TempDir()
		var names, paths []string
		for name := range sources {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			path := filepath.Join(root, name+".cls")
			writeFile(t, path, sources[name])
			paths = append(paths, path)
		}
		return typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: paths}, schema.Schema{})
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			seen := map[string]bool{}
			// Each row's same-named fixture replaces the preceding generation.
			// The shared runner links each complete index without mixing scopes.
			baseIndex := build(t, api, nil)
			org := orgFromIndex(baseIndex)
			org.APIVersion = api
			base := newConformanceRunner(t, baseIndex, conformanceRunnerOptions{LinkProject: true, Org: &org})
			for _, row := range data.Rows {
				if row.APIVersion != api {
					continue
				}
				if seen[row.ID] || row.Route != "isTest" || len(row.Sources) != 3 {
					t.Fatalf("invalid native row: %#v", row)
				}
				seen[row.ID] = true
				t.Run(row.Route+"/"+row.ID, func(t *testing.T) {
					index := build(t, api, row.Sources)
					analysis := sema.Analyze(index)
					actual := "compiled"
					// Preserve the full captured compiler error text and signature.
					var messages []string
					for _, d := range analysis.Diagnostics {
						if d.Severity == diagnostic.Error {
							message := d.NativeMessage
							if message == "" {
								message = d.Message
							}
							messages = append(messages, message)
						}
					}
					if len(messages) != 0 {
						actual = "COMPILE_ERROR\t" + strings.Join(messages, "\n")
					}
					if actual != row.ExpectedCompilation {
						t.Fatalf("native compilation <%s>, local <%s>: %#v", row.ExpectedCompilation, actual, analysis.Diagnostics)
					}
					if analysis.HasErrors() {
						if row.ExpectedRuntime != "" {
							t.Fatal("rejected row has a runtime observation")
						}
						return
					}
					if row.ExpectedRuntime == "" || row.ExpectedRuntime == "?" {
						t.Fatal("missing native runtime value")
					}
					terminal := "System.assert(false,'P|" + row.ID + "|'+String.valueOf(r));"
					if strings.Count(row.Sources["POTest"], terminal) != 1 {
						t.Fatal("missing captured @IsTest observation assertion")
					}
					sources := make(map[string]string, len(row.Sources))
					for name, source := range row.Sources {
						sources[name] = source
					}
					sources["POTest"] = strings.Replace(sources["POTest"], terminal, "System.assertEquals("+conformanceApexString(row.ExpectedRuntime)+",String.valueOf(r));", 1)
					executionIndex := build(t, api, sources)
					if result := sema.Analyze(executionIndex); result.HasErrors() {
						t.Fatalf("local assertion semantics: %#v", result.Diagnostics)
					}
					cases := Discover(executionIndex, Options{SelectedClasses: []string{"POTest"}})
					if len(cases) != 1 || cases[0].MethodName != row.ID {
						t.Fatalf("captured @IsTest method missing: %#v", cases)
					}
					methods, methodErrors := compileTestMethods(cases)
					programs, programErrors := compileTestInvokePrograms(cases)
					key := testCaseKey(cases[0])
					if methodErrors[key] != nil || programErrors[key] != nil {
						t.Fatalf("test lowering: %v %v", methodErrors[key], programErrors[key])
					}
					runner := newConformanceRunner(t, executionIndex, conformanceRunnerOptions{Base: base, LinkProject: true, Org: &org})
					machine := runner.newMachine()
					if err := machine.RegisterMethod(methods[key]); err != nil {
						t.Fatal(err)
					}
					machine.EnableTestContext()
					if _, err := machine.ExecuteInClass(programs[key], cases[0].ClassName); err != nil {
						t.Fatal(err)
					}
				})
			}
			for _, id := range strings.Fields("R001 R002 R003 R004 C001 C002 C003 C004") {
				if !seen[id] {
					t.Fatalf("missing native row %s/%s", api, id)
				}
			}
		})
	}
}
