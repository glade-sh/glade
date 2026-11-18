package apextest

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestSOQLNonCountLiteralNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/soql_count_order_followup"
	b, err := os.ReadFile(filepath.Join(fixture, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases map[string]string
	if err = json.Unmarshal(b, &cases); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(filepath.Join(fixture, "org.tsv"))
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
	for _, row := range rows[1:] {
		row := row
		t.Run(row[0]+"/"+row[2], func(t *testing.T) {
			root := t.TempDir()
			q := strings.ReplaceAll(cases[row[2]], "'", `\'`)
			source := "Object result; try { result = Database.query('" + q + "'); } catch (Exception e) { result = new Map<String,String>{'type'=>e.getTypeName(),'message'=>e.getMessage()}; } System.assert(false,'OBS|'+JSON.serialize(result));"
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+row[0]+`"}`)
			path := filepath.Join(root, "force-app/main/default/classes/LiteralProbeTest.cls")
			writeFile(t, path, "@IsTest public class LiteralProbeTest { @IsTest static void check() { "+source+" } }")
			writeFile(t, path+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+row[0]+`</apiVersion><status>Active</status></ApexClass>`)
			index := loadTestIndex(t, root)
			t.Setenv("GLADE_HOME", t.TempDir())
			for _, phase := range []string{"cold", "warm"} {
				t.Run("project/"+phase, func(t *testing.T) {
					run := Run(index, Options{SelectedClasses: []string{"LiteralProbeTest"}, Parallelism: 1})
					var message string
					for _, suite := range run.Suites {
						for _, c := range suite.Cases {
							if c.Problem != nil {
								message = c.Problem.Message
							}
						}
					}
					_, payload, ok := strings.Cut(message, "OBS|")
					if !ok {
						t.Fatalf("missing observation: %s", message)
					}
					var got, want any
					if json.Unmarshal([]byte(payload), &got) != nil || json.Unmarshal([]byte(row[3]), &want) != nil || !reflect.DeepEqual(got, want) {
						t.Fatalf("native %s; Glade %s", row[3], payload)
					}
				})
			}
			program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: row[0]})
			if err != nil {
				t.Fatal(err)
			}
			org := orgFromIndex(index)
			org.APIVersion = row[0]
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			for _, phase := range []string{"cold", "warm"} {
				t.Run(phase, func(t *testing.T) {
					_, err := runner.execute(program)
					var re *vm.RuntimeError
					if !errors.As(err, &re) {
						t.Fatalf("observation: %v", err)
					}
					_, payload, ok := strings.Cut(re.ExceptionMessage(), "OBS|")
					if !ok {
						t.Fatal(re.ExceptionMessage())
					}
					var got, want any
					if json.Unmarshal([]byte(payload), &got) != nil || json.Unmarshal([]byte(row[3]), &want) != nil {
						t.Fatalf("invalid JSON: %s / %s", payload, row[3])
					}
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("native %s; Glade %s", row[3], payload)
					}
				})
			}
		})
	}
}

// Native field-access observations: soql_count_order/org.tsv, compile rows.
func TestClosedSchemaProjectOverlayDirectories(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"base","default":true},{"path":"overlay"}],"sourceApiVersion":"`+api+`"}`)
			writeFile(t, filepath.Join(root, "base/main/default/objects/ShapeRecord__c/ShapeRecord__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Shape</label><pluralLabel>Shapes</pluralLabel><nameField><label>Shape Name</label><type>Text</type></nameField><deploymentStatus>Deployed</deploymentStatus><sharingModel>ReadWrite</sharingModel></CustomObject>`)
			writeFile(t, filepath.Join(root, "overlay/main/default/objects/ShapeRecord__c/fields/Known__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Known__c</fullName><label>Known</label><type>Text</type><length>80</length></CustomField>`)
			for _, tc := range []struct{ name, body, missing string }{
				{"known", "Object value = new ShapeRecord__c(Known__c='ok').Known__c;", ""},
				{"new_missing", "Object value = new ShapeRecord__c().Bogus__c;", "Bogus__c"},
				{"local_missing", "ShapeRecord__c row = new ShapeRecord__c(); Object value = row.Other__c;", "Other__c"},
			} {
				t.Run(tc.name, func(t *testing.T) {
					file := filepath.Join(root, "overlay/main/default/classes/OverlayProbe.cls")
					writeFile(t, file, "public class OverlayProbe { public static void run() { "+tc.body+" } }")
					writeFile(t, file+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
					index := loadTestIndex(t, root)
					for _, route := range []string{"project", "anonymous"} {
						t.Run(route, func(t *testing.T) {
							result := sema.Analyze(index)
							if route == "anonymous" {
								result = sema.AnalyzeAnonymous(index, tc.body, api)
							}
							if tc.missing == "" {
								if result.HasErrors() {
									t.Fatalf("known field rejected: %v", result.Diagnostics)
								}
								return
							}
							for _, d := range result.Diagnostics {
								if d.NativeMessage == "Variable does not exist: "+tc.missing || d.Code == "GLADESEMA021" && strings.Contains(d.Message, tc.missing) {
									return
								}
							}
							t.Fatalf("missing field %s: expected field rejection diagnostic; got %v", tc.missing, result.Diagnostics)
						})
					}
				})
			}
		})
	}
}
