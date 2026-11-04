package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The native account field captures do not cover recursive trigger errors;
// retain the existing SELF_REFERENCE_FROM_TRIGGER priority for both fields.
func TestCase349AccountSelfReferencePrecedesFieldPreflight(t *testing.T) {
	for _, field := range []struct{ name, assignment string }{{"firstname", "a.FirstName='Synthetic';"}, {"email", "a.PersonEmail='invalid';"}} {
		t.Run(field.name, func(t *testing.T) {
			org := testDataOrg()
			storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
			id := storage.ID("001000000000101")
			account := org.Objects["Account"]
			account.Definition.RecordTypes = []storage.RecordTypeInfo{{ID: "012000000000001", Active: true, Available: true}}
			account.Records[id] = storage.Record{Object: "Account", ID: id, Fields: map[string]storage.Value{"RecordTypeId": storage.IDValue("012000000000001"), "Name": storage.StringValue("Business")}}
			org.Objects["Account"] = account
			machine := New(nil)
			machine.SetOrg(&org)
			machine.beforeUpdateFrames = []beforeUpdateTriggerFrame{{name: "Synthetic", object: "Account", ids: map[string]bool{canonicalIDMapKey(string(id)): true}}}
			source := fmt.Sprintf("Account a=new Account(Id='001000000000101'); %s Database.SaveResult r=Database.update(a,false); System.assertEquals(false,r.isSuccess()); System.assertEquals('SELF_REFERENCE_FROM_TRIGGER',String.valueOf(r.getErrors()[0].getStatusCode()));", field.assignment)
			program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: "62.0"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
