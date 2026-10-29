package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestStandardConstructorVirtualFieldsNativeProjectRoutes(t *testing.T) {
	raw, err := os.ReadFile("testdata/conformance/standard_constructor_virtual_fields/observations.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		API, Case, Route, Source string
		ClassName                string `json:"class_name"`
		SourceSHA                string `json:"source_sha256"`
		Compiled, Success        bool
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 24 {
		t.Fatalf("native route matrix=%d, want 24", len(rows))
	}
	for _, row := range rows {
		t.Run(row.API+"/"+row.Route+"/"+row.Case, func(t *testing.T) {
			digest := sha256.Sum256([]byte(row.Source))
			if hex.EncodeToString(digest[:]) != row.SourceSHA {
				t.Fatal("native source changed")
			}
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+row.API+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			if row.Route != "anonymous" {
				p := filepath.Join(root, "force-app/main/default/classes", row.ClassName+".cls")
				writeFile(t, p, row.Source)
				writeFile(t, p+"-meta.xml", "<ApexClass><apiVersion>"+row.API+"</apiVersion></ApexClass>")
			}
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if row.Route == "anonymous" {
				analysis = sema.AnalyzeAnonymous(index, row.Source, row.API)
			}
			if got := !index.HasErrors() && !analysis.HasErrors(); got != row.Compiled {
				t.Fatalf("native compiled=%t; Glade=%t: %v %v", row.Compiled, got, index.Diagnostics, analysis.Diagnostics)
			}
			if !row.Compiled {
				return
			}
			if row.Route == "test-runner" {
				run := Run(index, Options{SelectedClasses: []string{row.ClassName}, SelectedMethod: "sample", NoDiskCache: true, Parallelism: 1})
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem != nil {
					t.Fatalf("native pass; Glade %+v", run.Summary())
				}
				return
			}
			source := row.Source
			if row.Route == "named" {
				source = row.ClassName + ".sample();"
			}
			program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: row.API})
			if err != nil {
				t.Fatal(err)
			}
			org := orgFromIndex(index)
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			if _, err = runner.execute(program); err != nil {
				t.Fatalf("native success; Glade %s", strings.TrimSpace(err.Error()))
			}
		})
	}
}

func TestStandardCommerceCapturedSettingProjectRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "named", "test-runner"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				root := t.TempDir()
				raw, err := os.ReadFile("testdata/conformance/standard_describe_profiles/profiles/P10.json")
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), string(raw))
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
				body := "WebStore row=new WebStore(Name='Owned store'); System.assertEquals('Owned store',row.Name);"
				source := "public class CommerceSettingRoute { public static void sample(){" + body + "} }"
				if route == "test-runner" {
					source = "@IsTest private class CommerceSettingRoute { @IsTest static void sample(){" + body + "} }"
				}
				if route != "anonymous" {
					p := filepath.Join(root, "force-app/main/default/classes/CommerceSettingRoute.cls")
					writeFile(t, p, source)
					writeFile(t, p+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
				}
				index := loadTestIndex(t, root)
				analysis := sema.Analyze(index)
				if route == "anonymous" {
					analysis = sema.AnalyzeAnonymous(index, body, api)
				}
				if index.HasErrors() || analysis.HasErrors() {
					t.Fatalf("native Commerce setting enables WebStore: %v %v", index.Diagnostics, analysis.Diagnostics)
				}
				if route == "test-runner" {
					run := Run(index, Options{SelectedClasses: []string{"CommerceSettingRoute"}, SelectedMethod: "sample", NoDiskCache: true, Parallelism: 1})
					if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem != nil {
						t.Fatalf("Commerce runner: %+v", run.Summary())
					}
					return
				}
				if route == "named" {
					body = "CommerceSettingRoute.sample();"
				}
				program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				if _, err := runner.execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestStandardConfiguredRecordTypeFieldProjectRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "named", "test-runner"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
				writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/fields/RecordTypeId.field-meta.xml"), `<CustomField><fullName>RecordTypeId</fullName><type>Lookup</type><referenceTo>RecordType</referenceTo><relationshipName>RecordType</relationshipName></CustomField>`)
				body := "Account row=new Account(); row.put('RecordTypeId',null); System.assertEquals(null,row.get('RecordTypeId'));"
				source := "public class ConfiguredRecordTypeRoute { public static void sample(){" + body + "} }"
				if route == "test-runner" {
					source = "@IsTest private class ConfiguredRecordTypeRoute { @IsTest static void sample(){" + body + "} }"
				}
				if route != "anonymous" {
					p := filepath.Join(root, "force-app/main/default/classes/ConfiguredRecordTypeRoute.cls")
					writeFile(t, p, source)
					writeFile(t, p+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
				}
				index := loadTestIndex(t, root)
				analysis := sema.Analyze(index)
				if route == "anonymous" {
					analysis = sema.AnalyzeAnonymous(index, body, api)
				}
				if index.HasErrors() || analysis.HasErrors() {
					t.Fatalf("configured field: %v %v", index.Diagnostics, analysis.Diagnostics)
				}
				if route == "test-runner" {
					run := Run(index, Options{SelectedClasses: []string{"ConfiguredRecordTypeRoute"}, SelectedMethod: "sample", NoDiskCache: true, Parallelism: 1})
					if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem != nil {
						t.Fatalf("configured runner: %+v", run.Summary())
					}
					return
				}
				if route == "named" {
					body = "ConfiguredRecordTypeRoute.sample();"
				}
				program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				if _, err := runner.execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
