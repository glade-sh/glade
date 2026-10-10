package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestEmptySOQLResultIsNonNullList(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Widget__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Widget__c",
			Fields: map[string]storage.Field{
				"Id": {APIName: "Id", Type: storage.FieldID},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	machine := New(nil)
	machine.SetOrg(&org)

	got, err := machine.executeSOQL("SELECT Id FROM Widget__c", &Result{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != ValueList || got.List == nil || len(got.List) != 0 {
		t.Fatalf("empty SOQL result = %#v, want non-nil empty List", got)
	}
}

func TestSOQLIDSetBindFiltersNonMatchingRecord(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Widget__mdt"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Widget__mdt",
			Fields: map[string]storage.Field{
				"Id": {APIName: "Id", Type: storage.FieldID},
			},
		},
		Records: map[storage.ID]storage.Record{
			"a03000000000001AAA": {ID: "a03000000000001AAA", Object: "Widget__mdt"},
		},
	}
	machine := New(nil)
	machine.SetOrg(&org)
	machine.Globals["ids"] = Set(platformScalar("Id", "a03000000000002AAA"))

	got, err := machine.executeSOQL("SELECT Id FROM Widget__mdt WHERE Id IN :ids", &Result{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != ValueList || len(got.List) != 0 {
		t.Fatalf("non-matching bind result = %#v, want empty List", got)
	}
}

func TestSOQLTypedIDBindKeepsCaseSensitivePrefix(t *testing.T) {
	const objectName = "MembershipType__c"
	const recordID = "a1e000000000001"
	org := storage.NewOrgState()
	org.Objects[objectName] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: objectName, KeyPrefix: "a1e"},
		Records: map[storage.ID]storage.Record{
			recordID: {ID: recordID, Object: objectName},
		},
	}
	machine := New(nil)
	machine.SetOrg(&org)
	id := String("a1E000000000001")
	id.Type = "Id"
	machine.Globals["requestedId"] = id

	got, err := machine.executeSOQL("SELECT Id FROM MembershipType__c WHERE Id = :requestedId", &Result{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != ValueList || len(got.List) != 0 {
		t.Fatalf("case-distinct typed Id bind result = %#v, want empty list", got)
	}

	id.Text = recordID
	machine.Globals["requestedId"] = id
	got, err = machine.executeSOQL("SELECT Id FROM MembershipType__c WHERE Id = :requestedId", &Result{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != ValueList || len(got.List) != 1 {
		t.Fatalf("exact typed Id bind result = %#v, want one row", got)
	}
}
