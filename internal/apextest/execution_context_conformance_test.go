package apextest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// Owned observations and fixtures run without Salesforce or external tooling.
func TestExecutionContextOrgConformance(t *testing.T) {
	type familyCase struct {
		ID, Code, Expected       string
		Compile, RawNull         bool
		RemainingReason          string
		AnonymousRemainingReason string
	}
	var data struct {
		APIVersions            []string `json:"apiVersions"`
		AssertedCases          int
		AssertedAnonymousCases int
		MetadataFiles          map[string]string
		Declarations           map[string]string
		NamedObservations      map[string]map[string]string
		Cases                  []familyCase
		Controls               []familyCase
	}
	raw, err := os.ReadFile("testdata/conformance/execution_context.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Cases) != 267 || len(data.Controls) != 2 || strings.Join(data.APIVersions, ",") != "62.0,67.0" {
		t.Fatalf("incomplete oracle: %d rows, APIs %v", len(data.Cases), data.APIVersions)
	}
	rows := make([]familyCase, 0, len(data.Cases))
	anonymousCases := 0
	seen := map[string]bool{}
	for _, tc := range append(append([]familyCase{}, data.Cases...), data.Controls...) {
		if tc.ID == "" || seen[tc.ID] {
			t.Fatalf("missing or repeated row ID: %q", tc.ID)
		}
		seen[tc.ID] = true
		if tc.RemainingReason != "" {
			t.Logf("remaining %s: %s", tc.ID, tc.RemainingReason)
			continue
		}
		if tc.AnonymousRemainingReason == "" {
			anonymousCases++
		} else {
			for _, api := range data.APIVersions {
				if _, ok := data.NamedObservations[api][tc.ID]; !ok {
					t.Fatalf("missing native named observation for %s at API %s", tc.ID, api)
				}
			}
		}
		rows = append(rows, tc)
	}
	if len(rows) != data.AssertedCases || len(rows) < 200 {
		t.Fatalf("asserted rows: %d, export says %d", len(rows), data.AssertedCases)
	}
	if anonymousCases != data.AssertedAnonymousCases {
		t.Fatalf("asserted anonymous rows: %d, export says %d", anonymousCases, data.AssertedAnonymousCases)
	}
	body := func(tc familyCase, expected string) string {
		code := tc.Code
		if !tc.Compile {
			code = "Object r; " + code
		}
		if strings.HasPrefix(expected, "COMPILE_ERROR") {
			return code
		}
		prefix := "String expectedText=" + conformanceApexString(expected) + "; String observedText; Boolean observedNull=false;\n"
		observation := " observedText=String.valueOf(r); observedNull=(r==null);"
		comparison := "System.assert(expectedText.equals(observedText), '" + tc.ID + " expected <'+expectedText+'> actual <'+observedText+'>');"
		if tc.RawNull {
			observation = " observedNull=(r==null); observedText=observedNull?'null':String.valueOf(r);"
			comparison += "System.assert(observedNull, '" + tc.ID + " expected raw null actual <'+observedText+'>');"
		}
		return prefix + "try {" + code + observation + "} catch(Exception e) {observedText='EXC|'+e.getTypeName()+'|'+e.getMessage();}\n" + comparison
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "execution context", api)
			if len(data.NamedObservations[api]) != 7 {
				t.Fatalf("missing native named-route controls at API %s", api)
			}
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			for name, source := range data.MetadataFiles {
				path := filepath.Join(root, filepath.FromSlash(name))
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				writeFile(t, path, source)
			}
			classDir := filepath.Join(root, "force-app", "main", "default", "classes")
			if err := os.MkdirAll(classDir, 0o755); err != nil {
				t.Fatal(err)
			}
			names := make([]string, 0, len(data.Declarations))
			for name := range data.Declarations {
				names = append(names, name)
			}
			sort.Strings(names)
			paths := make([]string, 0, len(names))
			for _, name := range names {
				path := filepath.Join(classDir, name+".cls")
				writeFile(t, path, data.Declarations[name])
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
				paths = append(paths, path)
			}
			buildIndex := func(extra string) typesys.Index {
				p, err := project.Load(root)
				if err != nil {
					t.Fatal(err)
				}
				p.ApexFiles = append([]string{}, paths...)
				if extra != "" {
					p.ApexFiles = append(p.ApexFiles, extra)
				}
				return typesys.Build(p, gladeschema.Schema{})
			}
			index := buildIndex("")
			if index.HasErrors() {
				t.Fatalf("helper parser: %v", index.Diagnostics)
			}
			if a := sema.Analyze(index); a.HasErrors() {
				t.Fatalf("helper semantics: %v", a.Diagnostics)
			}
			org := orgFromIndex(index)
			users := org.Objects["User"]
			user := users.Records["005000000000001"]
			user.Fields["LocaleSidKey"] = storage.StringValue("en_US")
			user.Fields["LanguageLocaleKey"] = storage.StringValue("en_US")
			user.Fields["TimeZoneSidKey"] = storage.StringValue("America/Los_Angeles")
			user.Fields["UserRoleId"] = storage.NullValue()
			users.Records[user.ID] = user
			org.Objects["User"] = users
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})

			var namedSource strings.Builder
			namedSource.WriteString(`@IsTest private class ExecutionContextProbe {
 @TestSetup static void configuredUser(){
  User u=[SELECT Id FROM User WHERE Id=:UserInfo.getUserId()];
  u.LocaleSidKey='en_US'; u.LanguageLocaleKey='en_US';
  u.TimeZoneSidKey='America/Los_Angeles'; u.UserRoleId=null; update u;
 }
`)
			accepted := 0
			for _, tc := range rows {
				if strings.HasPrefix(tc.Expected, "COMPILE_ERROR") {
					continue
				}
				expected := tc.Expected
				if observed, ok := data.NamedObservations[api][tc.ID]; ok {
					expected = observed
				}
				accepted++
				fmt.Fprintf(&namedSource, "@IsTest static void observed%s(){\n%s\n}\n", tc.ID, body(tc, expected))
			}
			namedSource.WriteString("}\n")
			namedPath := filepath.Join(classDir, "ExecutionContextProbe.cls")
			writeFile(t, namedPath, namedSource.String())
			writeFile(t, namedPath+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			namedIndex := buildIndex(namedPath)
			if namedIndex.HasErrors() {
				t.Fatalf("named parser: %v", namedIndex.Diagnostics)
			}
			if a := sema.Analyze(namedIndex); a.HasErrors() {
				t.Fatalf("named semantics: %v", a.Diagnostics)
			}
			run := Run(namedIndex, Options{NoDiskCache: true, Parallelism: 1})
			namedResults := map[string]testreport.Case{}
			for _, suite := range run.Suites {
				for _, result := range suite.Cases {
					id := strings.TrimPrefix(result.MethodName, "observed")
					if result.ClassName != "ExecutionContextProbe" || namedResults[id].MethodName != "" {
						t.Fatalf("unexpected/repeated named result: %#v", result)
					}
					namedResults[id] = result
				}
			}
			if run.Summary().Total != accepted || len(namedResults) != accepted {
				t.Fatalf("named results: %#v, unique %d expected %d: %s", run.Summary(), len(namedResults), accepted, firstRunProblem(run))
			}
			for _, tc := range rows {
				t.Run(tc.ID, func(t *testing.T) {
					source := body(tc, tc.Expected)
					rejected := strings.HasPrefix(tc.Expected, "COMPILE_ERROR")
					kind := "exact"
					if rejected {
						kind = "category"
					}
					if tc.AnonymousRemainingReason != "" {
						t.Logf("remaining %s/anonymous: %s", tc.ID, tc.AnonymousRemainingReason)
					} else {
						t.Run("anonymous", func(t *testing.T) {
							counts.TrackKind(t, "anonymous", kind, 1)
							analysis := sema.AnalyzeAnonymous(index, source, api)
							program, compileErr := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
							if rejected {
								if compileErr == nil && !analysis.HasErrors() {
									t.Fatal("org rejects compilation; anonymous path accepted")
								}
								return
							}
							if analysis.HasErrors() {
								t.Fatalf("anonymous semantics: %v", analysis.Diagnostics)
							}
							if compileErr != nil {
								t.Fatal(compileErr)
							}
							if _, err := runner.execute(program); err != nil {
								t.Fatal(err)
							}
						})
					}
					t.Run("isTest", func(t *testing.T) {
						counts.TrackKind(t, "@IsTest", kind, 1)
						if !rejected {
							result := namedResults[tc.ID]
							if result.Status != testreport.StatusPass {
								t.Fatalf("named row: status %s problem %v", result.Status, result.Problem)
							}
							return
						}
						path := filepath.Join(classDir, "ExecutionContextRejected.cls")
						writeFile(t, path, "@IsTest private class ExecutionContextRejected { @IsTest static void observed(){"+source+"} }")
						writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
						rejectedIndex := buildIndex(path)
						if !rejectedIndex.HasErrors() && !sema.Analyze(rejectedIndex).HasErrors() {
							r := Run(rejectedIndex, Options{NoDiskCache: true, Parallelism: 1})
							if r.Summary().CompileErrors == 0 {
								t.Fatalf("org rejects compilation; named path accepted: %#v %s", r.Summary(), firstRunProblem(r))
							}
						}
					})
				})
			}
		})
	}
}
