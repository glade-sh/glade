package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Triggers and helper methods share the transaction's query and row budgets:
// https://developer.salesforce.com/ja/wiki/apex_code_best_practices
// Owned governor rows R076-R080 and R103/R106 establish ordinary static and
// dynamic query accounting; they are not native trigger observations.
func TestExecTriggerSOQLSharesTransactionCounters(t *testing.T) {
	for _, timing := range []string{triggerTimingBefore, triggerTimingAfter} {
		for _, operation := range []string{"insert", "update"} {
			for _, dynamic := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/dynamic=%t", timing, operation, dynamic), func(t *testing.T) {
					selected := "[SELECT Id FROM Account WHERE Name = 'seed']"
					empty := "[SELECT Id FROM Account WHERE Id = null]"
					if dynamic {
						selected = "Database.query('SELECT Id FROM Account WHERE Name = \\'seed\\'')"
						empty = "Database.query('SELECT Id FROM Account WHERE Id = null')"
					}
					helper, err := CompileAnonymous(fmt.Sprintf(`
Integer queriesBefore = Limits.getQueries();
Integer rowsBefore = Limits.getQueryRows();
List<Account> selected = %s;
List<Account> empty = %s;
System.assertEquals(1, selected.size());
System.assertEquals(0, empty.size());
System.assertEquals(queriesBefore + 2, Limits.getQueries(), 'helper queries');
System.assertEquals(rowsBefore + 1, Limits.getQueryRows(), 'helper rows');
`, selected, empty))
					if err != nil {
						t.Fatal(err)
					}
					trigger, err := CompileAnonymous(`QueryLedger.read();`)
					if err != nil {
						t.Fatal(err)
					}
					setup := "Account row = new Account(Name = 'changed');"
					if operation == "update" {
						setup += "insert row; row.Name = 'updated';"
					}
					program, err := CompileAnonymous(setup + fmt.Sprintf(`
List<Account> empty = [SELECT Id FROM Account WHERE Id = null];
Integer queriesBefore = Limits.getQueries();
Integer rowsBefore = Limits.getQueryRows();
%s row;
System.assertEquals(queriesBefore + 2, Limits.getQueries(), 'caller queries');
System.assertEquals(rowsBefore + 1, Limits.getQueryRows(), 'caller rows');
empty = [SELECT Id FROM Account WHERE Id = null];
System.assertEquals(queriesBefore + 3, Limits.getQueries(), 'subsequent caller query');
System.assertEquals(rowsBefore + 1, Limits.getQueryRows(), 'empty caller query rows');
`, operation))
					if err != nil {
						t.Fatal(err)
					}
					machine := New(nil)
					org := testDataOrg()
					account := org.Objects["Account"]
					id := storage.ID("001000000000099")
					account.Records[id] = storage.Record{ID: id, Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("seed")}}
					org.Objects["Account"] = account
					machine.SetOrg(&org)
					if err := machine.RegisterClass(Class{Name: "QueryLedger", Methods: map[string]Method{
						"read": {Name: "QueryLedger.read", ClassName: "QueryLedger", IsStatic: true, ReturnType: "void", Program: helper},
					}}); err != nil {
						t.Fatal(err)
					}
					if err := machine.RegisterTrigger(Trigger{Name: "AccountQueryLedger", Object: "Account", Timing: timing, Operation: operation, Program: trigger}); err != nil {
						t.Fatal(err)
					}
					if _, err := machine.Execute(program); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}

// The query exemption depends on the metadata projection, not trigger depth.
// The API66 owned CMT query proof records both ordinary and long-text cases:
// TestCMTQueryLimitsAPI66Proof. Salesforce documents the long-text exception at
// https://help.salesforce.com/s/articleView?id=sf.custommetadatatypes_limits.htm&type=5
// The same allocation table specifies the records-returned transaction limit.
// These expectations compose that non-trigger proof with the documented shared
// transaction budget; no new native trigger capture is claimed.
func TestExecTriggerSOQLPreservesMetadataAccounting(t *testing.T) {
	for _, longText := range []bool{false, true} {
		for _, populated := range []bool{false, true} {
			t.Run(fmt.Sprintf("longText=%t/populated=%t", longText, populated), func(t *testing.T) {
				fields, name := "DeveloperName", "missing"
				queries, rows := 0, 0
				if longText {
					fields += ", Details__c"
					queries = 1
				}
				if populated {
					name = "seed"
					rows = 1
				}
				trigger, err := CompileAnonymous(fmt.Sprintf(`
Integer queriesBefore = Limits.getQueries();
Integer rowsBefore = Limits.getQueryRows();
List<CounterPolicy__mdt> selected = [SELECT %s FROM CounterPolicy__mdt WHERE DeveloperName = '%s'];
System.assertEquals(%d, selected.size());
System.assertEquals(queriesBefore + %d, Limits.getQueries(), 'metadata queries');
System.assertEquals(rowsBefore + %d, Limits.getQueryRows(), 'metadata rows');
`, fields, name, rows, queries, rows))
				if err != nil {
					t.Fatal(err)
				}
				program, err := CompileAnonymous(fmt.Sprintf(`
insert new Account(Name = 'changed');
System.assertEquals(%d, Limits.getQueries(), 'caller metadata queries');
System.assertEquals(%d, Limits.getQueryRows(), 'caller metadata rows');
`, queries, rows))
				if err != nil {
					t.Fatal(err)
				}
				org := testDataOrg()
				id := storage.ID("m01000000000001")
				org.Objects["CounterPolicy__mdt"] = storage.ObjectState{
					Definition: storage.ObjectDefinition{APIName: "CounterPolicy__mdt", Fields: map[string]storage.Field{
						"DeveloperName": {APIName: "DeveloperName", Type: storage.FieldString},
						"Details__c":    {APIName: "Details__c", Type: storage.FieldString, DisplayType: "TEXTAREA", Length: 1000},
					}},
					Records: map[storage.ID]storage.Record{id: {ID: id, Object: "CounterPolicy__mdt", Fields: map[string]storage.Value{
						"DeveloperName": storage.StringValue("seed"), "Details__c": storage.StringValue("details"),
					}}},
				}
				machine := New(nil)
				machine.SetOrg(&org)
				if err := machine.RegisterTrigger(Trigger{Name: "MetadataQueryLedger", Object: "Account", Timing: triggerTimingAfter, Operation: "insert", Program: trigger}); err != nil {
					t.Fatal(err)
				}
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
