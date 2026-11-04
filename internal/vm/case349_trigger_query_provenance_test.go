package vm

import (
	"fmt"
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// Queried Name rejects before trigger: case349_query_trigger/org.tsv:2-6,45-49.
func TestCase349QueriedNameTriggerHydration(t *testing.T) {
	for _, op := range []string{"update", "upsert"} {
		for _, api := range []string{"62.0", "67.0"} {
			for _, tc := range []struct{ name, trigger string }{{"untouched", "Integer observed=1;"}, {"same-name", "Trigger.new[0].Name=Trigger.new[0].Name;"}, {"changed-name", "Trigger.new[0].Name='Changed Synthetic';"}} {
				t.Run(op+"/"+api+"/"+tc.name, func(t *testing.T) {
					org := testDataOrg()
					org.APIVersion = api
					storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
					storage.EnsureStandardObject(&org, "Contact")
					machine := New(nil)
					machine.SetOrg(&org)
					if err := machine.RegisterClass(Class{Name: "QueryTrace", StaticFields: map[string]Field{"hits": {Name: "hits", Type: "Integer", Static: true, Access: "public", Value: Int(0), InitialValue: Int(0)}}}); err != nil {
						t.Fatal(err)
					}
					trigger, err := CompileAnonymousWithOptions("QueryTrace.hits++; "+tc.trigger, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if err = machine.RegisterTrigger(Trigger{Name: "HydrationObserver", Object: "Account", Timing: "before", Operation: "update", APIVersion: api, Program: trigger}); err != nil {
						t.Fatal(err)
					}
					source := fmt.Sprintf("Account seed=new Account(LastName='Synthetic'); insert seed; Account a=[SELECT Id,Name FROM Account WHERE Id=:seed.Id]; a.PersonHomePhone='4155550123'; QueryTrace.hits=0; Database.%sResult r=Database.%s(a,false); System.assertEquals(false,r.isSuccess(),'queried Name rejects before trigger'); System.assertEquals('INVALID_FIELD_FOR_INSERT_UPDATE',String.valueOf(r.getErrors()[0].getStatusCode())); System.assertEquals(0,QueryTrace.hits);", map[string]string{"update": "Save", "upsert": "Upsert"}[op], op)
					program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if _, err = machine.Execute(program); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}
