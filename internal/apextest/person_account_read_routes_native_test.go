package apextest

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func personReadRows(t *testing.T, file string) [][]string {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata/conformance/person_account_read_shapes", file))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func TestPersonAccountReadNativeRows(t *testing.T) {
	rows := personReadRows(t, "org.tsv")
	if len(rows) != 27 {
		t.Fatalf("got %d rows, want 27", len(rows))
	}
	seen := map[string]bool{}
	for _, r := range rows[1:] {
		if len(r) != 6 {
			t.Fatalf("malformed row: %v", r)
		}
		key := r[0] + "/" + r[1] + "/" + r[3]
		if seen[key] {
			t.Fatalf("duplicate %s", key)
		}
		seen[key] = true
		want := "REJECTED"
		if r[0] == "CONTROL" || r[0] == "FEATURE" {
			want = "PASS"
		}
		if r[4] != want {
			t.Errorf("%s status=%s want %s", key, r[4], want)
		}
		field := map[string]string{"READ": "FirstName", "ADJACENT": "LastName", "REJECTION": "DefinitelyAbsent__c"}[r[0]]
		if field != "" && !strings.Contains(r[5], "Variable does not exist: "+field) {
			t.Errorf("%s observation=%s", key, r[5])
		}
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, id := range []string{"READ", "ADJACENT", "CONTROL", "REJECTION"} {
			for _, route := range []string{"anonymous", "named", "istest"} {
				key := id + "/" + api + "/" + route
				if !seen[key] {
					t.Errorf("missing %s", key)
				}
			}
		}
		if !seen["FEATURE/"+api+"/anonymous"] {
			t.Errorf("missing feature control at %s", api)
		}
	}
}

// Native disabled READ/ADJACENT rows reject at compile time on each route.
func TestPersonAccountReadRouteShapes(t *testing.T) {
	cases := personReadRows(t, "cases.tsv")
	if len(cases) != 6 {
		t.Fatalf("unexpected cases: %v", cases)
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, enabled := range []bool{false, true} {
			for _, schema := range []bool{false, true} {
				for _, row := range cases[1:4] {
					id, body := row[0], row[1]
					t.Run(fmt.Sprintf("%s/enabled=%t/schema=%t/%s", api, enabled, schema, id), func(t *testing.T) {
						root := t.TempDir()
						writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
						feature := `{"features":[]}`
						if enabled {
							feature = `{"features":["PersonAccounts"]}`
						}
						writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), feature)
						if schema {
							writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/Account.object-meta.xml"), `<CustomObject><fields><fullName>Name</fullName><type>Text</type></fields></CustomObject>`)
						}
						class := `@IsTest private class ReadShapeTest { @IsTest static void sample() { ` + body + ` } }`
						path := filepath.Join(root, "force-app/main/default/classes/ReadShapeTest.cls")
						writeFile(t, path, class)
						writeFile(t, path+"-meta.xml", `<ApexClass><apiVersion>`+api+`</apiVersion></ApexClass>`)
						namedPath := filepath.Join(root, "force-app/main/default/classes/ReadShape.cls")
						writeFile(t, namedPath, `public class ReadShape { public static void sample() { `+body+` } }`)
						writeFile(t, namedPath+"-meta.xml", `<ApexClass><apiVersion>`+api+`</apiVersion></ApexClass>`)
						idx := loadTestIndex(t, root)
						anon := sema.AnalyzeAnonymous(idx, body, api)
						named := sema.Analyze(idx)
						field := map[string]string{"READ": "FirstName", "ADJACENT": "LastName"}[id]
						if !enabled && field != "" {
							for route, result := range map[string]sema.Result{"anonymous": anon, "project": named} {
								found := false
								for _, diagnostic := range result.Diagnostics {
									if strings.Contains(diagnostic.NativeMessage+" "+diagnostic.Message, "Variable does not exist: "+field) {
										found = true
									}
								}
								if !found {
									t.Errorf("%s missing rejection for %s: %v", route, field, result.Diagnostics)
								}
							}
							run := Run(idx, Options{SelectedClasses: []string{"ReadShapeTest"}, SelectedMethod: "sample", NoDiskCache: true, Parallelism: 1})
							if run.Summary().CompileErrors == 0 {
								t.Errorf("missing project compile rejection: %+v", run.Summary())
							}
							return
						}
						if anon.HasErrors() || named.HasErrors() {
							t.Fatalf("compile shape changed: anonymous=%v project=%v", anon.Diagnostics, named.Diagnostics)
						}
						program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatalf("anonymous compile: %v", err)
						}
						namedProgram, err := vm.CompileAnonymousWithOptions(`ReadShape.sample();`, vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatalf("named invocation compile: %v", err)
						}
						org := orgFromIndex(idx)
						runner := newConformanceRunner(t, idx, conformanceRunnerOptions{LinkProject: true, Org: &org})
						reject := !enabled && field != ""
						for _, state := range []string{"cold", "warm"} {
							_, runtimeErr := runner.execute(program)
							if reject {
								expected := `unknown field "` + field + `" on Account`
								if runtimeErr == nil || !strings.Contains(runtimeErr.Error(), expected) {
									t.Errorf("%s anonymous runtime=%v want %s", state, runtimeErr, expected)
								}
							} else if runtimeErr != nil {
								t.Errorf("%s anonymous runtime=%v", state, runtimeErr)
							}
							_, namedErr := runner.execute(namedProgram)
							if reject {
								expected := `unknown field "` + field + `" on Account`
								if namedErr == nil || !strings.Contains(namedErr.Error(), expected) {
									t.Errorf("%s named runtime=%v want %s", state, namedErr, expected)
								}
							} else if namedErr != nil {
								t.Errorf("%s named runtime=%v", state, namedErr)
							}
							run := Run(idx, Options{SelectedClasses: []string{"ReadShapeTest"}, SelectedMethod: "sample", NoDiskCache: true, Parallelism: 1})
							if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
								t.Fatalf("%s project result: %+v", state, run.Summary())
							}
							got := run.Suites[0].Cases[0]
							if reject {
								if run.Summary().CompileErrors != 0 || run.Summary().Failed != 1 || got.Problem == nil || !strings.Contains(got.Problem.Message, `unknown field "`+field+`" on Account`) {
									t.Errorf("%s project shape: summary=%+v case=%+v", state, run.Summary(), got)
								}
							} else if !run.Summary().OK() || run.Summary().Passed != 1 {
								t.Errorf("%s project shape: %+v", state, run.Summary())
							}
						}
					})
				}
			}
		}
	}
}

// Constructor twins were rejected natively at both API versions in
// person_account_without_feature/org.tsv (ACCOUNT_CTOR and ACCOUNT_CTOR_CLASS).
func TestPersonAccountConstructorDiagnosticShapes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, enabled := range []bool{false, true} {
			for _, field := range []string{"FirstName", "LastName"} {
				t.Run(fmt.Sprintf("%s/enabled=%t/%s", api, enabled, field), func(t *testing.T) {
					root := t.TempDir()
					writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
					feature := `{"features":[]}`
					if enabled {
						feature = `{"features":["PersonAccounts"]}`
					}
					writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), feature)
					writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/Account.object-meta.xml"), `<CustomObject><fields><fullName>Name</fullName><type>Text</type></fields></CustomObject>`)
					body := `Account a = new Account(` + field + `='A');`
					path := filepath.Join(root, "force-app/main/default/classes/ReadConstructorTest.cls")
					writeFile(t, path, `@IsTest private class ReadConstructorTest { @IsTest static void sample() { `+body+` } }`)
					writeFile(t, path+"-meta.xml", `<ApexClass><apiVersion>`+api+`</apiVersion></ApexClass>`)
					idx := loadTestIndex(t, root)
					for route, result := range map[string]sema.Result{"anonymous": sema.AnalyzeAnonymous(idx, body, api), "check": sema.Analyze(idx)} {
						if enabled {
							if result.HasErrors() {
								t.Errorf("%s enabled diagnostics=%v", route, result.Diagnostics)
							}
							continue
						}
						found := false
						for _, d := range result.Diagnostics {
							if d.Code == "GLADESEMA019" && strings.Contains(d.NativeMessage+" "+d.Message, "Field does not exist: "+field+" on Account") {
								found = true
							}
						}
						if !found {
							t.Errorf("%s expected GLADESEMA019 for %s; diagnostics=%v", route, field, result.Diagnostics)
						}
					}
					run := Run(idx, Options{SelectedClasses: []string{"ReadConstructorTest"}, SelectedMethod: "sample", NoDiskCache: true, Parallelism: 1})
					if enabled && !run.Summary().OK() {
						t.Errorf("enabled project run: %+v", run.Summary())
					}
					if !enabled && run.Summary().CompileErrors == 0 {
						t.Errorf("missing disabled compile rejection: %+v", run.Summary())
					}
				})
			}
		}
	}
}

// The host wrappers reach the same VM member read; native rows record the
// underlying Apex behavior, not Salesforce UI transport error text.
func TestPersonAccountReadHostRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, enabled := range []bool{false, true} {
			for _, field := range []string{"FirstName", "LastName", "Name"} {
				for _, route := range []string{"lwc", "vf"} {
					t.Run(fmt.Sprintf("%s/enabled=%t/%s/%s", api, enabled, field, route), func(t *testing.T) {
						root := t.TempDir()
						writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
						feature := `{"features":[]}`
						if enabled {
							feature = `{"features":["PersonAccounts"]}`
						}
						writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), feature)
						idx := loadTestIndex(t, root)
						org := orgFromIndex(idx)
						machine := vm.New(nil)
						machine.SetOrg(&org)
						body, err := vm.CompileAnonymousWithOptions(`Account a = new Account(); return a.`+field+`;`, vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						class := vm.Class{Name: "ReadHost", Methods: map[string]vm.Method{
							"lwc": {Name: "ReadHost.lwc", ClassName: "ReadHost", APIVersion: api, ReturnType: "String", IsStatic: true, Modifiers: []string{"public", "AuraEnabled"}, Program: body},
							"vf":  {Name: "ReadHost.vf", ClassName: "ReadHost", APIVersion: api, ReturnType: "String", Modifiers: []string{"public"}, Program: body},
						}}
						if err := machine.RegisterClass(class); err != nil {
							t.Fatal(err)
						}
						var out vm.UIInvocationResult
						if route == "lwc" {
							out, err = machine.InvokeLWCMethod("ReadHost", "lwc", nil)
						} else {
							out, err = machine.InvokeVisualforceAction("ReadHost", "vf", "/apex/ReadHost", nil)
						}
						if err != nil {
							t.Fatal(err)
						}
						if !enabled && field != "Name" {
							if out.Success || out.Error == nil || !strings.Contains(out.Error.Message, `unknown field "`+field+`" on Account`) {
								t.Fatalf("missing host failure: %+v", out)
							}
						} else if !out.Success {
							t.Fatalf("host failed: %+v", out.Error)
						}
					})
				}
			}
		}
	}
}
