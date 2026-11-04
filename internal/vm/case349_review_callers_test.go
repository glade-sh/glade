package vm

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestCase349DecimalScaleParentMatching(t *testing.T) {
	org := testDataOrg()
	definition := storage.ObjectDefinition{APIName: "ReviewNumber__c", Fields: map[string]storage.Field{"ReviewKey__c": {APIName: "ReviewKey__c", Type: storage.FieldDecimal, ExternalID: true}}}
	org.Objects["ReviewNumber__c"] = storage.ObjectState{Definition: definition, Records: map[storage.ID]storage.Record{}}
	machine := New(nil)
	machine.SetOrg(&org)
	parent := Object("ReviewNumber__c")
	parent.Fields["ReviewKey__c"] = Decimal(2.5)
	stored := storage.Record{Object: "ReviewNumber__c", ID: "a01000000000002AAA", Fields: map[string]storage.Value{"ReviewKey__c": storage.DecimalValue("2.500")}}
	id, err := machine.resolveParentRelationshipReferenceID([]storage.Record{stored}, parent)
	if err != nil || id != stored.ID {
		t.Fatalf("scaled pending key: id=%q err=%v", id, err)
	}
	object := org.Objects["ReviewNumber__c"]
	object.Records[stored.ID] = stored
	org.Objects["ReviewNumber__c"] = object
	id, err = machine.resolveParentRelationshipReferenceID(nil, parent)
	if err != nil || id != stored.ID {
		t.Fatalf("scaled stored key: id=%q err=%v", id, err)
	}
}

func TestCase349NamedBusinessBeforeTrigger(t *testing.T) {
	for _, name := range []string{"Person_Commerce", "Individual_Business"} {
		org := testDataOrg()
		storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
		account := org.Objects["Account"]
		account.Definition.RecordTypes = []storage.RecordTypeInfo{{ID: "012000000000001", DeveloperName: name, Name: name, Active: true, Available: true}}
		org.Objects["Account"] = account
		machine := New(nil)
		machine.SetOrg(&org)
		records := []storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Business"), "RecordTypeId": storage.IDValue("012000000000001"), "PersonHomePhone": storage.StringValue("4155550123")}}}
		machine.applyBeforeDMLDerivedFields(records)
		person, ok := records[0].GetField("IsPersonAccount")
		if ok && (person.Kind != storage.ValueBoolean || person.Boolean) {
			t.Errorf("%s before person=%#v", name, person)
		}
	}
}
