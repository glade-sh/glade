package apextest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/resource"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

func TestDMLPolicyNativeConformanceAnonymousAndIsTest(t *testing.T) {
	runDMLPolicyNativeConformance(t, "", "DmlPolicyObservationTest")
}

func TestDMLPolicyBulkUpsertAndTriggerRoutes(t *testing.T) {
	runDMLPolicyNativeConformance(t, "routes/", "DmlPolicyRouteObservationTest")
}

func runDMLPolicyNativeConformance(t *testing.T, variant, className string) {
	const fixture = "testdata/conformance/dml_policies"
	read := func(path string) string {
		t.Helper()
		body, err := os.ReadFile(filepath.Join(fixture, path))
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	expressions := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(read(variant+"cases.tsv")), "\n")[1:] {
		key, expr, ok := strings.Cut(line, "\t")
		if !ok || expressions[key] != "" {
			t.Fatalf("invalid case %q", line)
		}
		expressions[key] = expr
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			err := filepath.Walk(filepath.Join(fixture, "project"), func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return err
				}
				rel, _ := filepath.Rel(filepath.Join(fixture, "project"), path)
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				source := strings.ReplaceAll(string(body), "67.0", api)
				source = strings.ReplaceAll(source, "62.0", api)
				writeFile(t, filepath.Join(root, rel), source)
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native declarations accepted: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			loaded, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			if err := resource.ApplyProject(&org, loaded); err != nil {
				t.Fatal(err)
			}
			if err := resource.ApplyProjectQueues(&org, loaded); err != nil {
				t.Fatal(err)
			}
			org.APIVersion = api
			base := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			triggers, triggerErrors := compileProjectTriggers(index)
			if len(triggerErrors) > 0 {
				t.Fatal(triggerErrors)
			}
			for _, trigger := range triggers {
				if err := base.base.RegisterTrigger(trigger); err != nil {
					t.Fatal(err)
				}
			}
			for _, route := range []string{"anonymous", "istest"} {
				t.Run(route, func(t *testing.T) {
					capture := variant + "native/api" + strings.TrimSuffix(api, ".0") + "/" + route + "/"
					facts := read(capture + "RUN-FACTS.txt")
					for _, fact := range []string{"api=" + api, "context=" + route, "native_complete=true", "deployment_success=true", fmt.Sprintf("rows=%d", len(expressions))} {
						if !strings.Contains("\n"+facts, "\n"+fact+"\n") {
							t.Fatalf("missing native fact %q", fact)
						}
					}
					want := map[string]string{}
					for _, line := range strings.Split(strings.TrimSpace(read(capture+"org.tsv")), "\n")[1:] {
						key, value, ok := strings.Cut(line, "\t")
						if !ok || want[key] != "" || expressions[key] == "" {
							t.Fatalf("invalid observation %q", line)
						}
						want[key] = value
					}
					if len(want) != len(expressions) {
						t.Fatalf("incomplete capture: %d/%d", len(want), len(expressions))
					}
					if route == "istest" {
						run := Run(index, Options{SelectedClasses: []string{className}, NoDiskCache: true, Parallelism: 1})
						if run.Summary().Total != len(expressions) {
							t.Fatalf("unexpected project selection: %+v excluded=%+v", run.Summary(), run.ExcludedClasses)
						}
						seen := map[string]bool{}
						for _, suite := range run.Suites {
							for _, test := range suite.Cases {
								if test.Problem == nil {
									t.Errorf("%s: missing native observation", test.Name)
									continue
								}
								_, row, ok := strings.Cut(test.Problem.Message, "OBSERVED|")
								if !ok {
									t.Errorf("%s: %s", test.Name, test.Problem.Message)
									continue
								}
								key, value, ok := strings.Cut(row, "|")
								if !ok || want[key] == "" || seen[key] {
									t.Fatalf("invalid project row %q", row)
								}
								seen[key] = true
								t.Run(key, func(t *testing.T) {
									if value != want[key] {
										t.Errorf("native %q; Glade %q", want[key], value)
									}
								})
							}
						}
						if len(seen) != len(want) {
							t.Errorf("observed %d/%d native project rows", len(seen), len(want))
						}
					} else {
						for key, expected := range want {
							t.Run(key, func(t *testing.T) {
								source := "System.debug('OBSERVED|'+" + expressions[key] + ");"
								program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
								if err != nil {
									t.Fatal(err)
								}
								result, err := base.execute(program, func(machine *vm.VM) { machine.SetCurrentUser(org.Objects["User"].Records[storage.DefaultUserID]) })
								if err != nil {
									t.Fatalf("native %q; Glade error: %v", expected, err)
								}
								got := ""
								for _, line := range result.Debug {
									if _, value, ok := strings.Cut(line, "OBSERVED|"); ok {
										got = value
									}
								}
								if got != expected {
									t.Errorf("native %q; Glade %q", expected, got)
								}
							})
						}
					}
				})
			}
		})
	}
}

func TestDMLPolicyCalloutBoundariesIsTest(t *testing.T) {
	const fixture = "testdata/conformance/dml_policies"
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			err := filepath.Walk(filepath.Join(fixture, "project"), func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return err
				}
				rel, _ := filepath.Rel(filepath.Join(fixture, "project"), path)
				source, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, rel), strings.ReplaceAll(string(source), "67.0", api))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(filepath.Join(fixture, "boundaries", "api"+strings.TrimSuffix(api, ".0"), "org.tsv"))
			if err != nil {
				t.Fatal(err)
			}
			want := map[string]string{}
			for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
				key, value, ok := strings.Cut(line, "\t")
				if !ok {
					t.Fatalf("invalid native row %q", line)
				}
				want[key] = value
			}
			run := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{"PendingCalloutObservationTest"}, NoDiskCache: true, Parallelism: 1})
			if run.Summary().Total != len(want) {
				t.Fatalf("unexpected boundary selection: %+v excluded=%+v", run.Summary(), run.ExcludedClasses)
			}
			seen := map[string]bool{}
			for _, suite := range run.Suites {
				for _, test := range suite.Cases {
					if test.Problem == nil {
						t.Fatal("missing boundary observation")
					}
					_, row, ok := strings.Cut(test.Problem.Message, "OBSERVED|")
					if !ok {
						t.Errorf("%s", test.Problem.Message)
						continue
					}
					key, value, ok := strings.Cut(row, "|")
					if !ok || want[key] == "" || seen[key] {
						t.Fatalf("invalid boundary row %q", row)
					}
					seen[key] = true
					t.Run(key, func(t *testing.T) {
						if value != want[key] {
							t.Errorf("native %q; Glade %q", want[key], value)
						}
					})
				}
			}
			if len(seen) != len(want) {
				t.Errorf("observed %d/%d boundaries", len(seen), len(want))
			}
		})
	}
}
