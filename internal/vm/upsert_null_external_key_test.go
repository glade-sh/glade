package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Candidate implementation guard. Existing-Id selector precedence still needs
// native capture; this is not an exported Salesforce conformance expectation.
func TestExecUpsertExplicitNullExternalKeyWithID(t *testing.T) {
	program, err := CompileAnonymous(`
TriadParcel__c row = new TriadParcel__c(Name='parcel', ParcelKey__c='key');
insert row;
row.ParcelKey__c = null;
Database.UpsertResult[] results = Database.upsert(new List<TriadParcel__c>{row}, TriadParcel__c.ParcelKey__c, false);
System.assertEquals(false, results[0].isSuccess());
System.assertEquals(StatusCode.MISSING_ARGUMENT, results[0].getErrors()[0].getStatusCode());
System.assertEquals('key', [SELECT ParcelKey__c FROM TriadParcel__c WHERE Id=:row.Id].ParcelKey__c);
TriadParcel__c sparse = new TriadParcel__c(Id=row.Id, Name='updated');
System.assertEquals(true, Database.upsert(sparse, TriadParcel__c.ParcelKey__c, false).isSuccess());
TriadParcel__c matched = new TriadParcel__c(Name='matched', ParcelKey__c='key');
System.assertEquals(false, Database.upsert(matched, TriadParcel__c.ParcelKey__c, false).isCreated());
System.assertEquals(row.Id, matched.Id);
`)
	if err != nil {
		t.Fatal(err)
	}
	org := testDataOrg()
	org.Objects["TriadParcel__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "TriadParcel__c", KeyPrefix: "a01", Fields: map[string]storage.Field{
			"Name":         {APIName: "Name", Type: storage.FieldString},
			"ParcelKey__c": {APIName: "ParcelKey__c", Type: storage.FieldString, ExternalID: true},
		}},
		Records: map[storage.ID]storage.Record{},
	}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
