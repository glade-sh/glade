package vm

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// testdata/conformance/case349_review/org.tsv:5-8: explicit business stays
// business in before triggers even when a person-only contact will be rejected.
func TestCase349ReviewBusinessTriggerWithPersonSignal(t *testing.T) {
	for _, signal := range []string{"FirstName", "LastName", "PersonTitle"} {
		t.Run(signal, func(t *testing.T) {
			org := testDataOrg()
			storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
			account := org.Objects["Account"]
			account.Definition.RecordTypes = []storage.RecordTypeInfo{{ID: "012000000000001", DeveloperName: "Business", Active: true, Available: true}}
			org.Objects["Account"] = account
			machine := New(nil)
			machine.SetOrg(&org)
			records := []storage.Record{{Object: "Account", Fields: map[string]storage.Value{"RecordTypeId": storage.IDValue("012000000000001"), signal: storage.StringValue("Synthetic"), "PersonHomePhone": storage.StringValue("4155550123")}}}
			machine.applyBeforeDMLDerivedFields(records)
			person, ok := records[0].GetField("IsPersonAccount")
			if ok && (person.Kind != storage.ValueBoolean || person.Boolean) {
				t.Fatalf("business trigger person=%#v", person)
			}
		})
	}
}

// Native row testdata/conformance/case349_mixed/org.tsv:6,13 rejects a person
// Name update before the before trigger; row 7,14 permits person contact update.
func TestCase349PersonNamePreflightAndStoredFamily(t *testing.T) {
	org := testDataOrg()
	storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
	account := org.Objects["Account"]
	account.Definition.RecordTypes = []storage.RecordTypeInfo{{ID: "012000000000001", DeveloperName: "SyntheticPerson", Active: true, Available: true, PersonAccount: true}}
	id := storage.ID("001000000000101")
	account.Records[id] = storage.Record{Object: "Account", ID: id, Fields: map[string]storage.Value{"RecordTypeId": storage.IDValue("012000000000001"), "IsPersonAccount": storage.BooleanValue(true), "LastName": storage.StringValue("Synthetic")}}
	org.Objects["Account"] = account
	machine := New(nil)
	machine.SetOrg(&org)
	for _, op := range []struct{ op, key string }{{"update", ""}, {"upsert", ""}, {"upsert", "Id"}} {
		personName := []storage.Record{{Object: "Account", ID: id, Fields: map[string]storage.Value{"Name": storage.StringValue("Synthetic"), "PersonHomePhone": storage.StringValue("4155550123")}}}
		got := machine.preflightBusinessAccountNameFailures(personName, op.op, op.key)
		if len(got) != 1 || got[0].Success || got[0].StatusCode != "INVALID_FIELD_FOR_INSERT_UPDATE" {
			t.Errorf("%s/%s person Name: %#v", op.op, op.key, got)
		}
		personFirst := []storage.Record{{Object: "Account", ID: id, Fields: map[string]storage.Value{"FirstName": storage.StringValue("Synthetic")}}}
		personFirst[0].Fields["RecordTypeId"] = storage.IDValue("012000000000001")
		if got := machine.preflightBusinessAccountNameFailures(personFirst, op.op, op.key); len(got) != 0 && !got[0].Success {
			t.Errorf("%s/%s stored person first name: %#v", op.op, op.key, got)
		}
	}
}
