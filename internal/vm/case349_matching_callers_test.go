package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Salesforce documents that parent references use a matching external ID:
// https://developer.salesforce.com/docs/platform/api/guide/sforce-api-calls-upsert.html
func TestCase349DecimalParentRelationshipMatching(t *testing.T) {
	org := testDataOrg()
	definition := storage.ObjectDefinition{APIName: "EqualityProbe__c", Fields: map[string]storage.Field{"Key__c": {APIName: "Key__c", Type: storage.FieldDecimal, ExternalID: true}}}
	org.Objects["EqualityProbe__c"] = storage.ObjectState{Definition: definition, Records: map[storage.ID]storage.Record{}}
	machine := New(nil)
	machine.SetOrg(&org)
	parent := Object("EqualityProbe__c")
	parent.Fields["Key__c"] = Decimal(2.5)
	wrong := storage.Record{Object: "EqualityProbe__c", ID: "a01000000000001AAA", Fields: map[string]storage.Value{"Key__c": storage.DecimalValue("1.25")}}
	id, err := machine.resolveParentRelationshipReferenceID([]storage.Record{wrong}, parent)
	if err != nil || id != "" {
		t.Fatalf("distinct pending key: id=%q err=%v", id, err)
	}
	right := storage.Record{Object: "EqualityProbe__c", ID: "a01000000000002AAA", Fields: map[string]storage.Value{"Key__c": storage.DecimalValue("2.5")}}
	id, err = machine.resolveParentRelationshipReferenceID([]storage.Record{wrong, right}, parent)
	if err != nil || id != right.ID {
		t.Fatalf("matching pending key: id=%q err=%v", id, err)
	}
	object := org.Objects["EqualityProbe__c"]
	object.Records[wrong.ID] = wrong
	object.Records[right.ID] = right
	org.Objects["EqualityProbe__c"] = object
	id, err = machine.resolveParentRelationshipReferenceID(nil, parent)
	if err != nil || id != right.ID {
		t.Fatalf("matching stored key: id=%q err=%v", id, err)
	}
}

// Salesforce documents an exception for an unqueried sObject field:
// https://developer.salesforce.com/docs/platform/apex-dev/references/apex-dev/apex-exception-types-catching.html
func TestCase349DecimalStoredNonDefaultRemainsUnqueried(t *testing.T) {
	for _, number := range []string{"2.50", "3.75"} {
		t.Run(number, func(t *testing.T) {
			source := `Account a=new Account(Name='Stored',Balance__c=` + number + `); insert a;
Account q=[SELECT Id FROM Account WHERE Id=:a.Id]; Boolean rejected=false;
try { Decimal x=q.Balance__c; } catch(SObjectException e) { rejected=true; }
System.assertEquals(true,rejected);`
			program, err := CompileAnonymous(source)
			if err != nil {
				t.Fatal(err)
			}
			org := testDataOrg()
			account := org.Objects["Account"]
			account.Definition.Fields["Balance__c"] = storage.Field{APIName: "Balance__c", Type: storage.FieldDecimal, DefaultValue: "0"}
			org.Objects["Account"] = account
			machine := New(nil)
			machine.SetOrg(&org)
			if _, err = machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
