package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// The explicit metadata flag and existing record type remain authoritative.
func TestCase349PersistedFamilyAndExistingType38a(t *testing.T) {
	for _, family := range []bool{false, true} {
		for _, label := range []string{"Synthetic", "Individual_Business"} {
			t.Run(label+map[bool]string{false: "/business", true: "/person"}[family], func(t *testing.T) {
				org := testOrg()
				storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
				id := storage.ID("012000000000101")
				other := storage.ID("012000000000102")
				account := org.Objects["Account"]
				account.Definition.RecordTypes = []storage.RecordTypeInfo{{ID: id, DeveloperName: label, Active: true, Available: true, PersonAccount: family}, {ID: other, DeveloperName: "Other", Active: true, Available: true, PersonAccount: true}}
				org.Objects["Account"] = account
				org.Objects["RecordType"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "RecordType", KeyPrefix: "012", Fields: map[string]storage.Field{}}, Records: map[storage.ID]storage.Record{id: {ID: id, Object: "RecordType", Fields: map[string]storage.Value{"SobjectType": storage.StringValue("Account"), "DeveloperName": storage.StringValue(label), "IsPersonType": storage.BooleanValue(family)}}}}
				storage.EnsureDeterministicPlatformData(&org)
				def := org.Objects["Account"].Definition
				record := storage.Record{Object: "Account", ID: "001000000000101", Fields: map[string]storage.Value{"RecordTypeId": storage.IDValue(id), "LastName": storage.StringValue("Synthetic")}}
				if got := AccountHasExplicitBusinessRecordType("Account", def, record); got == family {
					t.Errorf("business=%v, persisted person=%v", got, family)
				}
				object := org.Objects["Account"]
				object.Records[record.ID] = record.Clone()
				org.Objects["Account"] = object
				engine := NewEngine(&org)
				engine.applyDefaultRecordTypeID("Account", def, &record)
				if got := idFromStorageValue(record.Fields["RecordTypeId"]); !storage.IDsEqual(got, id) {
					t.Errorf("existing explicit record type changed to %s", got)
				}
			})
		}
	}
}

func TestCase349ExistingExplicitRecordTypeNeverReplaced38a(t *testing.T) {
	for _, signal := range []string{"LastName", "FirstName"} {
		t.Run(signal, func(t *testing.T) {
			org := testOrg()
			id := storage.ID("012000000000101")
			other := storage.ID("012000000000102")
			def := storage.ObjectDefinition{APIName: "Account", RecordTypes: []storage.RecordTypeInfo{{ID: id, DeveloperName: "Business", Active: true, Available: true}, {ID: other, DeveloperName: "Person", Active: true, Available: true, PersonAccount: true}}}
			record := storage.Record{Object: "Account", ID: "001000000000101", Fields: map[string]storage.Value{"RecordTypeId": storage.IDValue(id), signal: storage.StringValue("Synthetic")}}
			obj := org.Objects["Account"]
			obj.Records[record.ID] = record.Clone()
			org.Objects["Account"] = obj
			engine := NewEngine(&org)
			engine.applyDefaultRecordTypeID("Account", def, &record)
			if got := idFromStorageValue(record.Fields["RecordTypeId"]); !storage.IDsEqual(got, id) {
				t.Errorf("explicit existing record type replaced: %s", got)
			}
		})
	}
}

// Existing-record updates and upserts must retain the persisted explicit type.
func TestCase349ExistingRecordTypeConnectedDML38a(t *testing.T) {
	for _, family := range []bool{false, true} {
		for _, op := range []string{"update", "upsert"} {
			t.Run(op+map[bool]string{false: "/business", true: "/person"}[family], func(t *testing.T) {
				org := testOrg()
				storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
				storage.EnsureStandardObject(&org, "Contact")
				id := storage.ID("012000000000111")
				other := storage.ID("012000000000112")
				account := org.Objects["Account"]
				account.Definition.RecordTypes = []storage.RecordTypeInfo{{ID: id, DeveloperName: "Existing", Active: true, Available: true, PersonAccount: family}, {ID: other, DeveloperName: "Other", Active: true, Available: true, PersonAccount: !family}}
				account.Records["001000000000111"] = storage.Record{Object: "Account", ID: "001000000000111", Fields: map[string]storage.Value{"Name": storage.StringValue("Synthetic"), "LastName": storage.StringValue("Synthetic"), "IsPersonAccount": storage.BooleanValue(family), "RecordTypeId": storage.IDValue(id)}}
				org.Objects["Account"] = account
				org.Objects["RecordType"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "RecordType", KeyPrefix: "012", Fields: map[string]storage.Field{}}, Records: map[storage.ID]storage.Record{id: {ID: id, Object: "RecordType", Fields: map[string]storage.Value{"SobjectType": storage.StringValue("Account"), "DeveloperName": storage.StringValue("Existing"), "IsPersonType": storage.BooleanValue(family)}}}}
				storage.EnsureDeterministicPlatformData(&org)
				engine := NewEngine(&org)
				input := storage.Record{Object: "Account", ID: "001000000000111", Fields: map[string]storage.Value{"RecordTypeId": storage.IDValue(id), "Description": storage.StringValue("Changed")}}
				var result []Result
				if op == "update" {
					result = engine.Update([]storage.Record{input})
				} else {
					result = engine.Upsert([]storage.Record{input})
				}
				if len(result) != 1 || !result[0].Success {
					t.Fatalf("%s: %#v", op, result)
				}
				got := idFromStorageValue(org.Objects["Account"].Records[input.ID].Fields["RecordTypeId"])
				if !storage.IDsEqual(got, id) {
					t.Errorf("type changed to %s", got)
				}
			})
		}
	}
}
