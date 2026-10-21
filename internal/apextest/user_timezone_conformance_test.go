package apextest

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

type userTimezoneCase struct {
	ID, Group, Code, CodeSHA256 string
	ExpectedByRouteAPI          map[string]map[string]string
}

// The 68 synthetic shapes have native answers at all three APIs on both
// routes. The fixture strips only the terminal P|row| transport envelope;
// payload whitespace, DML field lists and route-specific rejections survive.
// The captured display name is daylight time, as is the VM's fixed May clock.
// Row 068 rejects UTC on insert: it supplies no UTC restoration evidence.
func TestUserTimezoneNativeConformance(t *testing.T) {
	var data struct {
		APIVersions       []string `json:"apiVersions"`
		ExpectationStatus string
		Observations      int
		Cases             []userTimezoneCase
	}
	raw, err := os.ReadFile("testdata/conformance/user_timezone.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" || data.ExpectationStatus != "native captured" || data.Observations != 408 || len(data.Cases) != 68 {
		t.Fatalf("invalid native time-zone matrix: APIs=%v status=%q observations=%d cases=%d", data.APIVersions, data.ExpectationStatus, data.Observations, len(data.Cases))
	}
	for i, row := range data.Cases {
		if row.ID != fmt.Sprintf("C162TZ_%03d", i+1) || row.Group == "" || row.Code == "" || fmt.Sprintf("%x", sha256.Sum256([]byte(row.Code))) != row.CodeSHA256 || len(row.ExpectedByRouteAPI) != 2 {
			t.Fatalf("invalid source-bound time-zone row: %s", row.ID)
		}
		for _, route := range []string{"anonymous", "istest"} {
			values := row.ExpectedByRouteAPI[route]
			if len(values) != 3 {
				t.Fatalf("%s/%s missing API answers", row.ID, route)
			}
			for _, api := range data.APIVersions {
				if values[api] == "" || values[api] == "?" || values[api] != values["62.0"] {
					t.Fatalf("%s/%s/%s missing or changed native answer", row.ID, route, api)
				}
			}
		}
	}
	body := func(row userTimezoneCase, expected string) string {
		return "String r; try {\n" + row.Code +
			"\n} catch(DmlException e) { List<Object> failures=new List<Object>(); for(Integer i=0;i<e.getNumDml();i++){ failures.add(new List<Object>{String.valueOf(e.getDmlType(i)),e.getDmlFieldNames(i)}); } r=JSON.serialize(new List<Object>{e.getTypeName(),failures});" +
			" } catch(Exception e) { r=JSON.serialize(new List<Object>{e.getTypeName(),e.getMessage()}); }\n" +
			"String expectedText=" + conformanceApexString(expected) + "; System.assert(expectedText.equals(r)," + conformanceApexString(row.ID) + "+' expected <'+expectedText+'> actual <'+r+'>');"
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "UserTimezone", api)
			root := t.TempDir()
			project := fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api)
			writeFile(t, filepath.Join(root, "sfdx-project.json"), project)
			const className = "UserTimezoneOracleTest"
			var source strings.Builder
			source.WriteString("@IsTest private class " + className + " {\n")
			for _, row := range data.Cases {
				if strings.HasPrefix(row.ExpectedByRouteAPI["istest"][api], "COMPILE_ERROR|") {
					continue
				}
				fmt.Fprintf(&source, "@IsTest static void %s() {\n%s\n}\n", row.ID, body(row, row.ExpectedByRouteAPI["istest"][api]))
			}
			source.WriteString("}\n")
			path := filepath.Join(root, "force-app/main/default/classes", className+".cls")
			writeFile(t, path, source.String())
			writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			index := loadTestIndex(t, root)
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			if analysis := sema.Analyze(index); analysis.HasErrors() {
				t.Fatalf("named semantics: %v", analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			cases := Discover(index, Options{SelectedClasses: []string{className}})
			if len(cases) != 67 {
				t.Fatalf("named discovery: %d, want 67", len(cases))
			}
			methods, methodErrors := compileTestMethods(cases)
			programs, programErrors := compileTestInvokePrograms(cases)
			byID := make(map[string]TestCase)
			for _, tc := range cases {
				key := testCaseKey(tc)
				if methodErrors[key] != nil || programErrors[key] != nil {
					t.Fatalf("%s compile: %v %v", tc.MethodName, methodErrors[key], programErrors[key])
				}
				byID[tc.MethodName] = tc
			}
			for _, row := range data.Cases {
				t.Run(row.ID, func(t *testing.T) {
					for _, route := range []string{"anonymous", "istest"} {
						countRoute := route
						if route == "istest" {
							countRoute = "@IsTest"
						}
						expected := row.ExpectedByRouteAPI[route][api]
						if strings.HasPrefix(expected, "COMPILE_ERROR|") {
							counts.RunKind(t, countRoute, row.ID, route, "category", func(t *testing.T) {
								code := "String r;" + row.Code
								if route == "anonymous" {
									if analysis := sema.AnalyzeAnonymous(index, code, api); !analysis.HasErrors() {
										t.Fatal("native rejected TimeZone.getOffset(String), local semantics accepted it")
									}
									return
								}
								invalidRoot := t.TempDir()
								writeFile(t, filepath.Join(invalidRoot, "sfdx-project.json"), project)
								invalidPath := filepath.Join(invalidRoot, "force-app/main/default/classes/RejectedTimezoneTest.cls")
								writeFile(t, invalidPath, "@IsTest private class RejectedTimezoneTest { @IsTest static void rejected() {"+code+"} }")
								writeFile(t, invalidPath+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
								invalid := loadTestIndex(t, invalidRoot)
								if !invalid.HasErrors() && !sema.Analyze(invalid).HasErrors() {
									t.Fatal("native rejected named TimeZone.getOffset(String), local semantics accepted it")
								}
							})
							continue
						}
						counts.Run(t, countRoute, row.ID, route, func(t *testing.T) {
							if route == "anonymous" {
								code := body(row, expected)
								if analysis := sema.AnalyzeAnonymous(index, code, api); analysis.HasErrors() {
									t.Fatalf("anonymous semantics: %v", analysis.Diagnostics)
								}
								program, err := vm.CompileAnonymousWithOptions(code, vm.CompileOptions{APIVersion: api})
								if err != nil {
									t.Fatal(err)
								}
								if _, err := runner.execute(program); err != nil {
									t.Fatal(err)
								}
								return
							}
							method := byID[row.ID]
							key := testCaseKey(method)
							machine := runner.newMachine()
							if err := machine.RegisterMethod(methods[key]); err != nil {
								t.Fatal(err)
							}
							machine.EnableTestContext()
							machine.SetTestSeeAllData(method.SeeAllData)
							if _, err := machine.ExecuteInClass(programs[key], className); err != nil {
								t.Fatal(err)
							}
						})
					}
				})
			}
		})
	}
}
