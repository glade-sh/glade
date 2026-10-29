package apextest

import (
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

func TestPersonContactAliasNativeConformance(t *testing.T) {
	runSyntheticObservationNativeConformance(t, "person_contact_alias", "PersonAliasProbe", "observeAlias", "PERSON_ALIAS_RESULT", 10, true)
}

func runSyntheticObservationNativeConformance(t *testing.T, directory, probe, method, marker string, count int, personAccounts bool) {
	t.Helper()
	fixture := filepath.Join("testdata/conformance", directory)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			err := filepath.WalkDir(filepath.Join(fixture, "force-app"), func(path string, entry os.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				relative, err := filepath.Rel(fixture, path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, relative), strings.ReplaceAll(string(body), "<apiVersion>67.0</apiVersion>", "<apiVersion>"+api+"</apiVersion>"))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			if personAccounts {
				writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), `{"features":["PersonAccounts"]}`)
			}
			for _, name := range []string{probe, probe + "Test"} {
				writeFile(t, filepath.Join(root, "force-app/main/default/classes", name+".cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			}
			index := loadTestIndex(t, root)
			for _, route := range []string{"anonymous", "istest"} {
				t.Run(route, func(t *testing.T) {
					native := filepath.Join(fixture, "api"+api[:2], "native", route)
					run, err := os.ReadFile(filepath.Join(native, "RUN-FACTS.txt"))
					if err != nil {
						t.Fatal(err)
					}
					if !strings.Contains(string(run), "native_complete=true\n") || !strings.Contains(string(run), "api="+api+"\n") {
						t.Fatal("incomplete native capture")
					}
					table, err := os.ReadFile(filepath.Join(native, "org.tsv"))
					if err != nil {
						t.Fatal(err)
					}
					lines := strings.Split(strings.TrimSpace(string(table)), "\n")
					if len(lines) != count+1 || lines[0] != "id\tgroup\troute\tstatus\tvalue" {
						t.Fatal("incomplete native table")
					}
					want := map[string]any{}
					for _, line := range lines[1:] {
						fields := strings.SplitN(line, "\t", 5)
						if len(fields) != 5 || fields[2] != route {
							t.Fatal("invalid native row")
						}
						var value any
						if err := json.Unmarshal([]byte(fields[4]), &value); err != nil {
							t.Fatal(err)
						}
						want[fields[0]] = value
					}
					var message string
					if route == "istest" {
						result := Run(index, Options{SelectedClasses: []string{probe + "Test"}, SelectedMethod: method, NoDiskCache: true, Parallelism: 1})
						if result.Summary().Total != 1 || len(result.Suites) != 1 || len(result.Suites[0].Cases) != 1 {
							t.Fatalf("selection: %#v", result.Summary())
						}
						observed := result.Suites[0].Cases[0]
						if observed.Problem == nil {
							t.Fatal("missing observation")
						}
						message = observed.Problem.Message
					} else {
						body := `System.assert(false,'` + marker + `|'+JSON.serialize(` + probe + `.observe()));`
						if analysis := sema.AnalyzeAnonymous(index, body, api); analysis.HasErrors() {
							t.Fatalf("anonymous: %v", analysis.Diagnostics)
						}
						org := orgFromIndex(index)
						runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
						program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						_, err = runner.newMachine().Execute(program)
						var runtimeErr *vm.RuntimeError
						if !errors.As(err, &runtimeErr) {
							t.Fatalf("observation: %v", err)
						}
						message = runtimeErr.ExceptionMessage()
					}
					_, payload, found := strings.Cut(message, marker+"|")
					if !found {
						t.Fatalf("native observation missing; Glade: %s", message)
					}
					var got map[string]any
					if err := json.Unmarshal([]byte(payload), &got); err != nil {
						t.Fatal(err)
					}
					if len(got) != len(want) {
						t.Fatalf("rows=%d; native=%d", len(got), len(want))
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
		})
	}
}

// This preserves the existing local business-account path while alias support
// is bounded to the optional scalar metadata in the native fixture.
func TestPersonContactAliasesPreserveBusinessAccountsWithRequiredContactField(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), `{"features":["PersonAccounts"]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Contact/fields/Required_Tag__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Required_Tag__c</fullName><label>Required Tag</label><type>Text</type><length>24</length><required>true</required></CustomField>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/BusinessAccountControl.cls"), `@IsTest private class BusinessAccountControl { @IsTest static void insertBusinessAccount() { Account row=new Account(Name='River'); insert row; System.assertNotEquals(null,row.Id); } }`)
	index := loadTestIndex(t, root)
	result := Run(index, Options{SelectedClasses: []string{"BusinessAccountControl"}, SelectedMethod: "insertBusinessAccount", NoDiskCache: true, Parallelism: 1})
	if result.Summary().Passed != 1 || result.Summary().Total != 1 {
		t.Fatalf("business Account behavior changed: %+v; suites=%+v", result.Summary(), result.Suites)
	}
}
