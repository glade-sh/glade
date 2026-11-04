package apextest

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// The native matrix distinguishes seed events from candidate events. These
// assertions cover the latter, including rejections before and after triggers.
func TestCase349BusinessFollowupRoutes(t *testing.T) {
	f, err := os.Open("../../testdata/conformance/case349_followup/org.tsv")
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
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			var source strings.Builder
			source.WriteString("Id businessType = Account.SObjectType.getDescribe().getRecordTypeInfosByDeveloperName().get('Business_Default').getRecordTypeId();\n")
			count := 0
			ordered := append([][]string(nil), rows[1:]...)
			sort.SliceStable(ordered, func(i, j int) bool {
				priority := func(row []string) bool {
					return row[1] == "PersonHomePhone" && row[2] == "NONEMPTY" && row[3] == "insert"
				}
				return priority(ordered[i]) && !priority(ordered[j])
			})
			for _, row := range ordered {
				if row[5] != api || row[6] != "named-IsTest-project" {
					continue
				}
				var obs struct {
					Result struct {
						Success bool `json:"success"`
						Errors  struct {
							Items []struct {
								Status string `json:"statusCode"`
							} `json:"items"`
						} `json:"errors"`
					} `json:"result"`
					BeforeEvents []struct {
						Phase string `json:"phase"`
						New   struct {
							IsPersonAccount *bool `json:"IsPersonAccount"`
						} `json:"new"`
					} `json:"beforeEvents"`
				}
				if err := json.Unmarshal([]byte(row[7]), &obs); err != nil {
					t.Fatal(err)
				}
				events := 0
				for _, event := range obs.BeforeEvents {
					if event.Phase == "candidate" {
						events++
						if event.New.IsPersonAccount == nil || *event.New.IsPersonAccount {
							t.Fatalf("unexpected candidate state %s", row[0])
						}
					}
				}
				n := count
				count++
				if row[3] == "insert" {
					fmt.Fprintf(&source, "Account a%d = new Account(Name='Business', RecordTypeId=businessType);\n", n)
				} else {
					fmt.Fprintf(&source, "Account s%d = new Account(Name='Business', RecordTypeId=businessType); insert s%d; Account a%d = new Account(Id=s%d.Id);\n", n, n, n, n)
				}
				source.WriteString("Case349Events.values.clear();\n")
				if row[2] != "OMITTED" {
					values := map[string]string{"NULL": "null", "SPACES": "'   '", "TAB_SPACES": "' \\t '", "FALSE_TEXT": "'false'", "NONEMPTY": "'2125550100'"}
					value, ok := values[row[2]]
					if !ok {
						t.Fatal(row[2])
					}
					fmt.Fprintf(&source, "a%d.put('%s', %s);\n", n, row[1], value)
				}
				kind := "Database.SaveResult"
				if row[3] == "upsert" {
					kind = "Database.UpsertResult"
				}
				fmt.Fprintf(&source, "%s r%d=Database.%s(a%d, false); System.assertEquals(%t,r%d.isSuccess(),'%s success');\n", kind, n, row[3], n, obs.Result.Success, n, row[0])
				if !obs.Result.Success {
					fmt.Fprintf(&source, "System.assertEquals('%s',String.valueOf(r%d.getErrors()[0].getStatusCode()),'%s status');\n", obs.Result.Errors.Items[0].Status, n, row[0])
				}
				fmt.Fprintf(&source, "System.assertEquals(%d,Case349Events.values.size(),'%s candidate events');\n", events, row[0])
				if events != 0 {
					fmt.Fprintf(&source, "System.assertEquals(false,Case349Events.values[0],'%s person state');\n", row[0])
				}
			}
			if count != 46 {
				t.Fatalf("native rows: %d", count)
			}
			root := t.TempDir()
			base := filepath.Join(root, "force-app/main/default")
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), `{"edition":"Developer","features":["PersonAccounts"]}`)
			writeFile(t, filepath.Join(base, "objects/Account/recordTypes/Business_Default.recordType-meta.xml"), readFixtureFile(t, "../../testdata/conformance/person_account_followups/force-app/main/default/objects/Account/recordTypes/Business_Default.recordType-meta.xml"))
			classMeta := fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api)
			writeFile(t, filepath.Join(base, "classes/Case349Events.cls"), `public class Case349Events { public static List<Boolean> values = new List<Boolean>(); }`)
			writeFile(t, filepath.Join(base, "classes/Case349Events.cls-meta.xml"), classMeta)
			writeFile(t, filepath.Join(base, "classes/Case349FollowupTest.cls"), "@IsTest private class Case349FollowupTest { @IsTest static void exercise() { "+source.String()+" } }")
			writeFile(t, filepath.Join(base, "classes/Case349FollowupTest.cls-meta.xml"), classMeta)
			writeFile(t, filepath.Join(base, "triggers/Case349Observer.trigger"), `trigger Case349Observer on Account (before insert, before update) { for (Account a : Trigger.new) Case349Events.values.add(a.IsPersonAccount); }`)
			writeFile(t, filepath.Join(base, "triggers/Case349Observer.trigger-meta.xml"), fmt.Sprintf(`<ApexTrigger xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexTrigger>`, api))
			index := loadTestIndex(t, root)
			if len(index.Triggers) != 1 {
				t.Fatalf("observer trigger not indexed: %d", len(index.Triggers))
			}
			t.Run("anonymous", func(t *testing.T) {
				org := orgFromIndex(index)
				org.APIVersion = api
				machine := vm.New(nil)
				machine.SetOrg(&org)
				if err := RegisterProjectRuntime(machine, index); err != nil {
					t.Fatal(err)
				}
				program, err := vm.CompileAnonymousWithOptions(source.String(), vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err = machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
			for _, route := range []string{"project", "warm"} {
				t.Run(route, func(t *testing.T) {
					result := Run(index, Options{SelectedClasses: []string{"Case349FollowupTest"}, Parallelism: 1, ParallelMethods: true})
					if summary := result.Summary(); summary.Total != 1 || summary.Passed != 1 {
						t.Fatalf("%s: %+v %s", route, summary, strings.TrimSpace(firstRunProblem(result)))
					}
				})
			}
		})
	}
}
