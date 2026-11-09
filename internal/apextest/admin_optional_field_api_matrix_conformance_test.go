package apextest

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// adminOptionalFieldCarries pins Glade's current output for cells that differ
// from native (key: api/method/route). A carry fails when Glade changes.
var adminOptionalFieldCarries = map[string]string{}

// Native admin with and without an assigned field-level-security permission
// set; see testdata/conformance/admin_optional_field_api_matrix/ORG-RUN.txt.
func TestAdminOptionalFieldAPIMatrixNativeConformance(t *testing.T) {
	const family = "testdata/conformance/admin_optional_field_api_matrix"
	raw, err := os.Open(filepath.Join(family, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	reader := csv.NewReader(raw)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	apis := []string{"62.0", "66.0", "67.0", "68.0"}
	if len(rows) != 1+len(apis)*9 || strings.Join(rows[0], ",") != "source_api,method,observation" {
		t.Fatal("incomplete native matrix")
	}
	read := func(name string) string {
		b, err := os.ReadFile(filepath.Join(family, "anon", name))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	probe, staticSoql, staticInsert, seClass := read("probe.apex"), read("static_soql.apex"), read("static_insert.apex"), read("static_se_class.cls")
	for _, api := range apis {
		api := api
		for _, phase := range []string{"NoPermSet", "PermSet"} {
			phase := phase
			runConformanceSubtestParallel(t, api+"/"+phase, func(t *testing.T) {
				root := t.TempDir()
				project := filepath.Join(family, "project")
				err := filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
					if err != nil || entry.IsDir() {
						return err
					}
					body, err := os.ReadFile(path)
					if err != nil {
						return err
					}
					rel, err := filepath.Rel(project, path)
					if err != nil {
						return err
					}
					writeFile(t, filepath.Join(root, rel), strings.ReplaceAll(string(body), "62.0", api))
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
				if phase == "PermSet" {
					writeFile(t, filepath.Join(root, "glade.yml"), "test:\n  permissionSets: [CapFls]\n")
				}
				index := loadTestIndex(t, root)
				if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
					t.Fatalf("native compiled; Glade diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
				}
				// Every route replays the capture's recorded assignment
				// (glade.yml test.permissionSets in the PermSet phase).
				org := capturedFixtureOrg(t, index, api)
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				cases := Discover(index, Options{SelectedClasses: []string{"AdminProbe"}})
				methods, methodErrors := compileTestMethods(cases)
				programs, programErrors := compileTestInvokePrograms(cases)
				if len(cases) != 1 {
					t.Fatalf("discovered %d cases", len(cases))
				}
				c := cases[0]
				key := testCaseKey(c)
				for _, row := range rows[1:] {
					if row[0] != api || row[1] == "classStaticSecurityEnforced" || strings.HasSuffix(row[1], "NoPermSet") != (phase == "NoPermSet") {
						continue
					}
					name := row[1]
					var want map[string]any
					if json.Unmarshal([]byte(row[2]), &want) != nil {
						t.Fatalf("bad row %s", name)
					}
					compare := func(t *testing.T, route string, got map[string]any) {
						t.Helper()
						gotJSON, _ := json.Marshal(got)
						carryKey := api + "/" + name + "/" + route
						if reflect.DeepEqual(got, want) {
							if _, ok := adminOptionalFieldCarries[carryKey]; ok {
								t.Fatal("now matches native; remove its carry")
							}
							return
						}
						if pinned, ok := adminOptionalFieldCarries[carryKey]; ok {
							var local map[string]any
							if json.Unmarshal([]byte(pinned), &local) != nil || !reflect.DeepEqual(got, local) {
								t.Fatalf("carried cell changed: native=%s; Glade=%s", row[2], gotJSON)
							}
							t.Logf("CARRY native=%s; Glade=%s", row[2], gotJSON)
							return
						}
						t.Fatalf("CELL %s native=%s; Glade=%s", carryKey, row[2], gotJSON)
					}
					fromMessage := func(message string) map[string]any {
						_, payload, ok := strings.Cut(message, "OBSERVED|adminDirect|")
						var got map[string]any
						if !ok || json.Unmarshal([]byte(payload), &got) != nil {
							t.Fatalf("invalid transport: %s", message)
						}
						return got
					}
					t.Run(name, func(t *testing.T) {
						switch {
						case name == "adminNoPermSet" || name == "adminPermSet":
							t.Run("isTest", func(t *testing.T) {
								if methodErrors[key] != nil || programErrors[key] != nil {
									t.Fatalf("compile %v %v", methodErrors[key], programErrors[key])
								}
								machine := runner.newMachine()
								if err := machine.RegisterMethod(methods[key]); err != nil {
									t.Fatal(err)
								}
								machine.EnableTestContext()
								_, err := machine.ExecuteInClass(programs[key], c.ClassName)
								var runtimeErr *vm.RuntimeError
								if !errors.As(err, &runtimeErr) || runtimeErr.Type != "System.AssertException" {
									t.Fatalf("missing assertion transport: %v", err)
								}
								compare(t, "isTest", fromMessage(runtimeErr.ExceptionMessage()))
							})
							t.Run("project", func(t *testing.T) {
								run := Run(index, Options{SelectedClasses: []string{"AdminProbe"}, SelectedMethod: "adminDirect", NoDiskCache: true, Parallelism: 1})
								if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
									t.Fatalf("missing runner observation %+v", run.Summary())
								}
								compare(t, "project", fromMessage(run.Suites[0].Cases[0].Problem.Message))
							})
						case name == "anonNoPermSet" || name == "anonPermSet":
							program, err := vm.CompileAnonymousWithOptions(probe, vm.CompileOptions{APIVersion: api})
							if err != nil {
								t.Fatalf("anonymous compile: %v", err)
							}
							result, err := runner.newMachine().Execute(program)
							if err != nil {
								t.Fatalf("anonymous run: %v", err)
							}
							for _, line := range result.Debug {
								if _, payload, ok := strings.Cut(line, "OBSERVED|anonymous|"); ok {
									var got map[string]any
									if json.Unmarshal([]byte(payload), &got) != nil {
										t.Fatalf("bad payload %s", payload)
									}
									compare(t, "anonymous", got)
									return
								}
							}
							t.Fatal("missing anonymous observation")
						case strings.HasPrefix(name, "anonStatic"):
							src := staticSoql
							if strings.HasPrefix(name, "anonStaticInsert") {
								src = staticInsert
							}
							// The glade exec compile route: project semantics.
							got := map[string]any{"result": "compiled"}
							machine := runner.newMachine()
							visible := sema.WithoutAnonymousHiddenFields(index, machine.AnonymousPolicyHiddenFields(api, src))
							if result := sema.AnalyzeAnonymous(visible, src, api); result.HasErrors() {
								got["result"] = "compileError"
								t.Logf("compile: %s | native: %s", result.Diagnostics[0].Message, result.Diagnostics[0].NativeMessage)
							}
							compare(t, "anonymousCompile", got)
						}
					})
				}
				if phase == "NoPermSet" {
					t.Run("classStaticSecurityEnforced", func(t *testing.T) {
						dir := t.TempDir()
						writeFile(t, filepath.Join(dir, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
						for rel, body := range map[string]string{
							"force-app/main/default/classes/SeProbe.cls":                                 seClass,
							"force-app/main/default/classes/SeProbe.cls-meta.xml":                        `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>` + api + `</apiVersion><status>Active</status></ApexClass>`,
							"force-app/main/default/objects/CapObj__c/CapObj__c.object-meta.xml":         readFamily(t, project, "force-app/main/default/objects/CapObj__c/CapObj__c.object-meta.xml"),
							"force-app/main/default/objects/CapObj__c/fields/Optional__c.field-meta.xml": readFamily(t, project, "force-app/main/default/objects/CapObj__c/fields/Optional__c.field-meta.xml"),
						} {
							writeFile(t, filepath.Join(dir, rel), body)
						}
						idx := loadTestIndex(t, dir)
						got := map[string]any{"result": "compiled"}
						if idx.HasErrors() || sema.Analyze(idx).HasErrors() {
							got["result"] = "compileError"
						}
						for _, row := range rows[1:] {
							if row[0] == api && row[1] == "classStaticSecurityEnforced" {
								var want map[string]any
								_ = json.Unmarshal([]byte(row[2]), &want)
								carryKey := api + "/classStaticSecurityEnforced/sema"
								if reflect.DeepEqual(got, want) {
									if _, ok := adminOptionalFieldCarries[carryKey]; ok {
										t.Fatal("now matches native; remove its carry")
									}
									return
								}
								gotJSON, _ := json.Marshal(got)
								if pinned, ok := adminOptionalFieldCarries[carryKey]; ok && pinned == string(gotJSON) {
									t.Logf("CARRY native=%s; Glade=%s", row[2], gotJSON)
									return
								}
								t.Fatalf("CELL %s native=%s; Glade=%s", carryKey, row[2], gotJSON)
							}
						}
					})
				}
			})
		}
	}
}

func readFamily(t *testing.T, project, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(project, rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
