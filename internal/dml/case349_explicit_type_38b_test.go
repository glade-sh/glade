package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// Explicit RecordTypeId is retained, including for new records and person signals.
func TestCase349ExplicitRecordTypeNeverReplaced38b(t *testing.T) {
	for _, signal := range []string{"FirstName", "LastName"} {
		for _, id := range []storage.ID{"", "001000000000123"} {
			t.Run(signal+"/"+string(id), func(t *testing.T) {
				org := testOrg()
				business := storage.ID("012000000000101")
				person := storage.ID("012000000000102")
				def := storage.ObjectDefinition{APIName: "Account", RecordTypes: []storage.RecordTypeInfo{{ID: business, DeveloperName: "Business", Active: true, Available: true}, {ID: person, DeveloperName: "Person", Active: true, Available: true, PersonAccount: true}}}
				rec := storage.Record{Object: "Account", ID: id, Fields: map[string]storage.Value{"RecordTypeId": storage.IDValue(business), signal: storage.StringValue("Synthetic")}}
				engine := NewEngine(&org)
				engine.applyDefaultRecordTypeID("Account", def, &rec)
				if got := idFromStorageValue(rec.Fields["RecordTypeId"]); !storage.IDsEqual(got, business) {
					t.Errorf("explicit type replaced: %s", got)
				}
			})
		}
	}
}

// A VM-provided default is not an explicit RecordTypeId assignment.
func TestCase349VMDefaultRecordTypeMayBeReplaced38b(t *testing.T) {
	for _, signal := range []string{"FirstName", "LastName"} {
		t.Run(signal, func(t *testing.T) {
			org := testOrg()
			business := storage.ID("012000000000101")
			person := storage.ID("012000000000102")
			def := storage.ObjectDefinition{APIName: "Account", RecordTypes: []storage.RecordTypeInfo{{ID: business, DeveloperName: "Business", Active: true, Available: true}, {ID: person, DeveloperName: "Person", Active: true, Available: true, PersonAccount: true}}}
			rec := storage.Record{Object: "Account", Fields: map[string]storage.Value{"RecordTypeId": storage.IDValue(business), signal: storage.StringValue("Synthetic")}}
			rec.MarkDMLRecordTypeDefaulted()
			engine := NewEngine(&org)
			engine.applyDefaultRecordTypeID("Account", def, &rec)
			if got := idFromStorageValue(rec.Fields["RecordTypeId"]); !storage.IDsEqual(got, person) {
				t.Errorf("VM default not replaced by person type: %s", got)
			}
		})
	}
}

// Explicit family classification is independent of name component combinations.
func TestCase349ExplicitBusinessFamilyWithNameComponents38b(t *testing.T) {
	id := storage.ID("012000000000101")
	def := storage.ObjectDefinition{APIName: "Account", RecordTypes: []storage.RecordTypeInfo{{ID: id, DeveloperName: "Synthetic", PersonAccount: false}}}
	for _, withName := range []bool{false, true} {
		t.Run(map[bool]string{false: "components", true: "components-and-name"}[withName], func(t *testing.T) {
			fields := map[string]storage.Value{"RecordTypeId": storage.IDValue(id), "FirstName": storage.StringValue("Synthetic"), "LastName": storage.StringValue("Person")}
			if withName {
				fields["Name"] = storage.StringValue("Synthetic Person")
			}
			if !AccountHasExplicitBusinessRecordType("Account", def, storage.Record{Object: "Account", Fields: fields}) {
				t.Error("explicit business type not classified as business")
			}
		})
	}
}
