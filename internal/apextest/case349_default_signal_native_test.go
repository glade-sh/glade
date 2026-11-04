package apextest

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// The reviewed matrix supplies a business default and an explicit person twin.
// It observes the before-trigger kind as well as each Database result.
func TestCase349DefaultPersonSignalNative(t *testing.T) {
	f, err := os.Open("../../testdata/conformance/case349_default_signal/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 649 {
		t.Fatalf("native matrix rows = %d", len(rows)-1)
	}
	paired := make(map[string]map[string]any)
	seen := make(map[string]bool)
	for _, row := range rows[1:] {
		if (row[7] != "CAPTURED" && row[7] != "TERMINAL_ROLLBACK_UNVERIFIABLE") || (row[3] != "anonymous" && row[3] != "named") {
			t.Fatalf("invalid native row %s", row[0])
		}
		var observation map[string]any
		if err := json.Unmarshal([]byte(row[8]), &observation); err != nil {
			t.Fatal(err)
		}
		projection := make(map[string]any)
		for _, field := range []string{"result_status", "errors", "candidate_events", "stored_values", "isCreated"} {
			projection[field] = observation[field]
		}
		key := strings.Join([]string{row[1], row[2], row[4], row[5], row[6]}, "/")
		if seen[key+"/"+row[3]] {
			t.Fatalf("duplicate native route for %s", key)
		}
		seen[key+"/"+row[3]] = true
		if first, ok := paired[key]; ok {
			if !reflect.DeepEqual(first, projection) {
				t.Fatalf("native route mismatch for %s", key)
			}
		} else {
			paired[key] = projection
		}
	}
	if len(paired) != 324 {
		t.Fatalf("native route pairs = %d", len(paired))
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			base := filepath.Join(root, "force-app/main/default")
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), `{"edition":"Developer","features":["PersonAccounts"]}`)
			for _, obj := range []string{"Account/recordTypes/Business_Default.recordType-meta.xml", "PersonAccount/recordTypes/Patient.recordType-meta.xml"} {
				writeFile(t, filepath.Join(base, "objects", obj), readFixtureFile(t, filepath.Join("../../testdata/conformance/person_account_followups/force-app/main/default/objects", obj)))
			}
			meta := fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api)
			writeFile(t, filepath.Join(base, "classes/DefaultSignalEvents.cls"), `public class DefaultSignalEvents { public static List<Boolean> values = new List<Boolean>(); }`)
			writeFile(t, filepath.Join(base, "classes/DefaultSignalEvents.cls-meta.xml"), meta)
			writeFile(t, filepath.Join(base, "triggers/DefaultSignalObserver.trigger"), `trigger DefaultSignalObserver on Account (before insert, before update) { for (Account a : Trigger.new) DefaultSignalEvents.values.add(a.IsPersonAccount); }`)
			writeFile(t, filepath.Join(base, "triggers/DefaultSignalObserver.trigger-meta.xml"), fmt.Sprintf(`<ApexTrigger xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexTrigger>`, api))
			sources := map[string]string{}
			for _, mode := range []string{"main", "explicitbusiness", "person"} {
				var source strings.Builder
				if mode == "person" {
					source.WriteString("Id personType=Account.SObjectType.getDescribe().getRecordTypeInfosByDeveloperName().get('Patient').getRecordTypeId();\n")
				} else if mode == "explicitbusiness" {
					source.WriteString("Id businessType=Account.SObjectType.getDescribe().getRecordTypeInfosByDeveloperName().get('Business_Default').getRecordTypeId();\n")
				}
				count := 0
				for _, row := range rows[1:] {
					if row[1] != mode || row[2] != api || row[3] != "named" {
						continue
					}
					var obs struct {
						Result string         `json:"result_status"`
						Stored map[string]any `json:"stored_values"`
						Errors []struct {
							Status string   `json:"statusCode"`
							Fields []string `json:"fields"`
						} `json:"errors"`
						CandidateEvents []struct {
							Person bool `json:"isPersonAccount"`
						} `json:"candidate_events"`
					}
					if err := json.Unmarshal([]byte(row[8]), &obs); err != nil {
						t.Fatal(err)
					}
					n := count
					count++
					makeAccount := "new Account(Name='Business')"
					if mode == "person" {
						makeAccount = "new Account(LastName='Person',RecordTypeId=personType)"
					} else if mode == "explicitbusiness" {
						makeAccount = "new Account(Name='Business',RecordTypeId=businessType)"
					}
					if row[6] == "insert" {
						fmt.Fprintf(&source, "Account a%d=%s;\n", n, makeAccount)
					} else {
						fmt.Fprintf(&source, "Account s%d=%s; insert s%d; Account a%d=new Account(Id=s%d.Id);\n", n, makeAccount, n, n, n)
					}
					source.WriteString("DefaultSignalEvents.values.clear();\n")
					if row[5] != "omitted" {
						values := map[string]string{"null": "null", "spaces": "'   '", "tabspaces": "'\\t  '", "textfalse": "'false'", "validnonempty": "'4155550123'"}
						value, ok := values[row[5]]
						if !ok {
							t.Fatal(row[5])
						}
						if row[4] == "PersonEmail" && row[5] == "validnonempty" {
							value = "'synthetic@example.invalid'"
						}
						fmt.Fprintf(&source, "a%d.put('%s',%s);\n", n, row[4], value)
					}
					operation := row[6]
					if operation == "sparseupdate" {
						operation = "update"
					}
					if operation == "idupsert" {
						operation = "upsert"
					}
					kind := "Database.SaveResult"
					if operation == "upsert" {
						kind = "Database.UpsertResult"
					}
					success := obs.Result == "SUCCESS"
					fmt.Fprintf(&source, "%s r%d=Database.%s(a%d,false); System.assertEquals(%t,r%d.isSuccess(),'%s success');\n", kind, n, operation, n, success, n, row[0])
					if !success {
						if len(obs.Errors) == 0 {
							t.Fatal(row[0], "missing native error")
						}
						fmt.Fprintf(&source, "System.assertEquals('%s',String.valueOf(r%d.getErrors()[0].getStatusCode()),'%s status');\n", obs.Errors[0].Status, n, row[0])
						fmt.Fprintf(&source, "System.assertEquals('%s',String.join(r%d.getErrors()[0].getFields(),','),'%s fields');\n", strings.Join(obs.Errors[0].Fields, ","), n, row[0])
					}
					fmt.Fprintf(&source, "System.assertEquals(%d,DefaultSignalEvents.values.size(),'%s events');\n", len(obs.CandidateEvents), row[0])
					if len(obs.CandidateEvents) > 0 {
						fmt.Fprintf(&source, "System.assertEquals(%t,DefaultSignalEvents.values[0],'%s before');\n", obs.CandidateEvents[0].Person, row[0])
					}
					if success {
						if _, ok := obs.Stored[row[4]]; !ok {
							t.Fatalf("missing native stored field for %s", row[0])
						}
						fmt.Fprintf(&source, "Account stored%d=[SELECT IsPersonAccount,PersonHomePhone,PersonMobilePhone,PersonEmail FROM Account WHERE Id=:a%d.Id]; System.assertEquals(%t,stored%d.IsPersonAccount,'%s stored kind');\n", n, n, mode == "person", n, row[0])
						want := "null"
						if value, ok := obs.Stored[row[4]].(string); ok {
							want = "'" + strings.ReplaceAll(value, "'", "\\'") + "'"
						}
						fmt.Fprintf(&source, "System.assertEquals(%s,stored%d.get('%s'),'%s stored field');\n", want, n, row[4], row[0])
						if operation == "upsert" {
							fmt.Fprintf(&source, "System.assertEquals(false,r%d.isCreated(),'%s created');\n", n, row[0])
						}
					}
				}
				if count != 54 {
					t.Fatalf("%s native rows = %d", mode, count)
				}
				if mode == "main" {
					// Additional native malformed-email rows establish the early
					// validation boundary beyond the original text-false input.
					for i, value := range []string{"x", "x@y"} {
						fmt.Fprintf(&source, "DefaultSignalEvents.values.clear(); Account invalid%d=new Account(Name='Business',PersonEmail='%s'); Database.SaveResult invalidResult%d=Database.insert(invalid%d,false); System.assertEquals(false,invalidResult%d.isSuccess()); System.assertEquals('INVALID_EMAIL_ADDRESS',String.valueOf(invalidResult%d.getErrors()[0].getStatusCode())); System.assertEquals(0,DefaultSignalEvents.values.size());\n", i, value, i, i, i, i)
					}
				}
				sources[mode] = source.String()
				name := "DefaultSignal" + strings.Title(mode) + "Test"
				writeFile(t, filepath.Join(base, "classes", name+".cls"), "@IsTest private class "+name+" { @IsTest static void run() { "+source.String()+" } }")
				writeFile(t, filepath.Join(base, "classes", name+".cls-meta.xml"), meta)
			}
			index := loadTestIndex(t, root)
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			if len(index.Triggers) != 1 {
				t.Fatalf("observer trigger count %d", len(index.Triggers))
			}
			for _, mode := range []string{"main", "explicitbusiness", "person"} {
				t.Run(mode, func(t *testing.T) {
					t.Run("anonymous", func(t *testing.T) {
						org := orgFromIndex(index)
						org.APIVersion = api
						machine := vm.New(nil)
						machine.SetOrg(&org)
						if err := RegisterProjectRuntime(machine, index); err != nil {
							t.Fatal(err)
						}
						program, err := vm.CompileAnonymousWithOptions(sources[mode], vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						if _, err = machine.Execute(program); err != nil {
							t.Fatal(err)
						}
					})
					for _, route := range []string{"project", "warm"} {
						t.Run(route, func(t *testing.T) {
							name := "DefaultSignal" + strings.Title(mode) + "Test"
							result := Run(index, Options{SelectedClasses: []string{name}, Parallelism: 1, ParallelMethods: true})
							if summary := result.Summary(); summary.Total != 1 || summary.Passed != 1 {
								t.Fatalf("%s: %+v %s", route, summary, firstRunProblem(result))
							}
						})
					}
				})
			}
		})
	}
}

// The malformed-email capture uses distinct invalid forms and a valid-address
// rejection twin, all with the default business type.
func TestCase349MalformedEmailBeforeTrigger(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			base := filepath.Join(root, "force-app/main/default")
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), `{"edition":"Developer","features":["PersonAccounts"]}`)
			writeFile(t, filepath.Join(base, "objects/Account/recordTypes/Business_Default.recordType-meta.xml"), readFixtureFile(t, "../../testdata/conformance/person_account_followups/force-app/main/default/objects/Account/recordTypes/Business_Default.recordType-meta.xml"))
			writeFile(t, filepath.Join(base, "classes/InvalidEmailEvents.cls"), `public class InvalidEmailEvents { public static Integer count=0; }`)
			writeFile(t, filepath.Join(base, "classes/InvalidEmailEvents.cls-meta.xml"), fmt.Sprintf(`<ApexClass><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api))
			writeFile(t, filepath.Join(base, "triggers/InvalidEmailObserver.trigger"), `trigger InvalidEmailObserver on Account (before insert) { InvalidEmailEvents.count++; }`)
			writeFile(t, filepath.Join(base, "triggers/InvalidEmailObserver.trigger-meta.xml"), fmt.Sprintf(`<ApexTrigger><apiVersion>%s</apiVersion><status>Active</status></ApexTrigger>`, api))
			index := loadTestIndex(t, root)
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			machine := vm.New(nil)
			machine.SetOrg(&org)
			if err := RegisterProjectRuntime(machine, index); err != nil {
				t.Fatal(err)
			}
			source := `for(String v:new List<String>{'x','x@y','false'}) { InvalidEmailEvents.count=0; Account a=new Account(Name='Business',PersonEmail=v); Database.SaveResult r=Database.insert(a,false); System.assertEquals(false,r.isSuccess()); System.assertEquals('INVALID_EMAIL_ADDRESS',String.valueOf(r.getErrors()[0].getStatusCode())); System.assertEquals(0,InvalidEmailEvents.count); }
InvalidEmailEvents.count=0; Account valid=new Account(Name='Business',PersonEmail='synthetic@example.invalid'); Database.SaveResult result=Database.insert(valid,false); System.assertEquals(false,result.isSuccess()); System.assertEquals('INVALID_FIELD_FOR_INSERT_UPDATE',String.valueOf(result.getErrors()[0].getStatusCode())); System.assertEquals(1,InvalidEmailEvents.count);`
			program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
