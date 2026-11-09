package apextest

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// The anonymous SOQL route replays the synthetic native probe; both source API
// versions are recorded in setup_formula_org/org.tsv.
func TestSetupFormulaOrgDefaultAnonymous(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "setup_formula_org")
	body, err := os.ReadFile(filepath.Join(dir, "probe.apex"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			index := loadTestIndex(t, filepath.Join(dir, "project"))
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			program, err := vm.CompileAnonymousWithOptions(string(body), vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := runner.newMachine().Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// Test-runner execution reuses the captured anonymous program in a named
// @IsTest method. The native observation is in setup_formula_org/org.tsv:2-5.
func TestSetupFormulaOrgDefaultProjectIsTest(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "setup_formula_org")
	body, err := os.ReadFile(filepath.Join(dir, "probe.apex"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(dir, "project")
			err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				rel, err := filepath.Rel(source, path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, rel), strings.ReplaceAll(string(data), "62.0", api))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			classes := filepath.Join(root, "force-app", "main", "default", "classes")
			writeFile(t, filepath.Join(classes, "SetupFormulaProbeTest.cls"), "@IsTest private class SetupFormulaProbeTest { @IsTest static void runProbe() { "+string(body)+" } }")
			writeFile(t, filepath.Join(classes, "SetupFormulaProbeTest.cls-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?><ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			index := loadTestIndex(t, root)
			for _, phase := range []string{"cold", "warm"} {
				if phase == "warm" {
					runtimeCacheMu.RLock()
					_, cached := runtimeCache[runtimeKey(index)]
					runtimeCacheMu.RUnlock()
					if !cached {
						t.Fatal("cold run did not retain a project runtime")
					}
				}
				result := Run(index, Options{SelectedClasses: []string{"SetupFormulaProbeTest"}, NoDiskCache: true, Parallelism: 1})
				summary := result.Summary()
				if summary.Total != 1 || !summary.OK() || summary.Passed != 1 {
					t.Fatalf("%s result: %+v suites=%+v", phase, summary, result.Suites)
				}
			}
		})
	}
}

// Each source is an independent native transaction; replay the hierarchy
// precedence observations from setup_formula_org/org.tsv:6-11.
func TestSetupFormulaOrgHierarchyAnonymous(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "setup_formula_org")
	for _, api := range []string{"62.0", "67.0"} {
		for _, name := range []string{"user_over_org", "profile_over_org", "user_over_profile"} {
			t.Run(api+"/"+name, func(t *testing.T) {
				body, err := os.ReadFile(filepath.Join(dir, "precedence", name+".apex"))
				if err != nil {
					t.Fatal(err)
				}
				index := loadTestIndex(t, filepath.Join(dir, "project"))
				org := orgFromIndex(index)
				org.APIVersion = api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				program, err := vm.CompileAnonymousWithOptions(string(body), vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := runner.newMachine().Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestSetupFormulaOrgHierarchyProjectIsTest(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "setup_formula_org")
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(dir, "project")
			err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				rel, err := filepath.Rel(source, path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, rel), strings.ReplaceAll(string(data), "62.0", api))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			var code strings.Builder
			code.WriteString("@IsTest private class SetupHierarchyProbeTest {")
			for _, name := range []string{"user_over_org", "profile_over_org", "user_over_profile"} {
				body, err := os.ReadFile(filepath.Join(dir, "precedence", name+".apex"))
				if err != nil {
					t.Fatal(err)
				}
				code.WriteString(" @IsTest static void " + name + "() { " + string(body) + " }")
			}
			code.WriteString(" }")
			classes := filepath.Join(root, "force-app", "main", "default", "classes")
			writeFile(t, filepath.Join(classes, "SetupHierarchyProbeTest.cls"), code.String())
			writeFile(t, filepath.Join(classes, "SetupHierarchyProbeTest.cls-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?><ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			index := loadTestIndex(t, root)
			for _, phase := range []string{"cold", "warm"} {
				if phase == "warm" {
					runtimeCacheMu.RLock()
					_, cached := runtimeCache[runtimeKey(index)]
					runtimeCacheMu.RUnlock()
					if !cached {
						t.Fatal("cold run did not retain a project runtime")
					}
				}
				run := Run(index, Options{SelectedClasses: []string{"SetupHierarchyProbeTest"}, NoDiskCache: true, Parallelism: 1})
				summary := run.Summary()
				if summary.Total != 3 || !summary.OK() || summary.Passed != 3 {
					t.Fatalf("%s: %+v suites=%+v", phase, summary, run.Suites)
				}
			}
		})
	}
}

// Replay the retained native runAs class with its captured metadata.
func TestSetupFormulaRunAsProjectIsTest(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "setup_formula_org")
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(dir, "project")
			err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				rel, err := filepath.Rel(source, path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, rel), strings.ReplaceAll(string(data), "62.0", api))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			classes := filepath.Join(root, "force-app", "main", "default", "classes")
			for _, name := range []string{"SetupRunAsProbeTest.cls", "SetupRunAsProbeTest.cls-meta.xml"} {
				data, err := os.ReadFile(filepath.Join(dir, "precedence", name))
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(classes, name), strings.ReplaceAll(string(data), "62.0", api))
			}
			run := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{"SetupRunAsProbeTest"}, NoDiskCache: true, Parallelism: 1})
			summary := run.Summary()
			if summary.Total != 1 || !summary.OK() || summary.Passed != 1 {
				t.Fatalf("runAs: %+v suites=%+v", summary, run.Suites)
			}
		})
	}
}

// Replay field-level inheritance captured in separate native transactions.
func TestSetupFormulaFieldInheritanceNativeReplay(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "setup_formula_org")
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range []struct{ name, want string }{{"field_inheritance", "profile"}, {"field_org", "org"}, {"field_user", "user"}} {
			for _, route := range []string{"anonymous", "istest"} {
				t.Run(api+"/"+tc.name+"/"+route, func(t *testing.T) {
					body, err := os.ReadFile(filepath.Join(dir, "precedence", tc.name+".apex"))
					if err != nil {
						t.Fatal(err)
					}
					root := t.TempDir()
					source := filepath.Join(dir, "project")
					err = filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
						if err != nil || entry.IsDir() {
							return err
						}
						data, err := os.ReadFile(path)
						if err != nil {
							return err
						}
						rel, err := filepath.Rel(source, path)
						if err != nil {
							return err
						}
						writeFile(t, filepath.Join(root, rel), strings.ReplaceAll(string(data), "62.0", api))
						return nil
					})
					if err != nil {
						t.Fatal(err)
					}
					if route == "istest" {
						classes := filepath.Join(root, "force-app", "main", "default", "classes")
						writeFile(t, filepath.Join(classes, "SetupFieldProbeTest.cls"), "@IsTest private class SetupFieldProbeTest { @IsTest static void observe() { "+string(body)+" } }")
						writeFile(t, filepath.Join(classes, "SetupFieldProbeTest.cls-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?><ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
					}
					index := loadTestIndex(t, root)
					var message string
					if route == "istest" {
						run := Run(index, Options{SelectedClasses: []string{"SetupFieldProbeTest"}, NoDiskCache: true, Parallelism: 1})
						if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
							t.Fatalf("missing assertion transport: %+v", run.Suites)
						}
						message = run.Suites[0].Cases[0].Problem.Message
					} else {
						org := orgFromIndex(index)
						org.APIVersion = api
						runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
						program, err := vm.CompileAnonymousWithOptions(string(body), vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						_, err = runner.newMachine().Execute(program)
						if err == nil {
							t.Fatal("missing assertion transport")
						}
						message = err.Error()
					}
					if !strings.Contains(message, "OBS|"+tc.want) {
						t.Fatalf("native=%q glade=%s", tc.want, message)
					}
				})
			}
		}
	}
}
