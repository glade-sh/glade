package dml

import (
	"errors"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestWithTransactionRewindsRecordCreations(t *testing.T) {
	org := testOrg()
	engine := NewEngine(&org)
	kept := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Kept")}}})[0]
	if !kept.Success {
		t.Fatal(kept.Error)
	}
	var rolledBackID storage.ID
	rejected := errors.New("synthetic rollback")
	err := engine.WithTransaction(func(tx *Engine) error {
		inserted := tx.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Temporary")}}})[0]
		if !inserted.Success {
			t.Fatal(inserted.Error)
		}
		rolledBackID = inserted.ID
		return rejected
	})
	if !errors.Is(err, rejected) {
		t.Fatalf("transaction error = %v", err)
	}
	if len(org.Objects["Account"].Records) != 1 || !engine.RecordCreations.Contains(kept.ID) {
		t.Fatal("rollback lost the earlier insert")
	}
	if engine.RecordCreations.Contains(rolledBackID) {
		t.Error("rollback retained phantom insertion provenance")
	}
	if engine.RecordCreations.Mark() != 1 {
		t.Error("rollback retained the failed insertion log entry")
	}
	reused := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Replacement")}}})[0]
	if !reused.Success || reused.ID != rolledBackID || !engine.RecordCreations.Contains(reused.ID) {
		t.Fatalf("reused insertion = %#v; rolled-back ID = %s", reused, rolledBackID)
	}
}

func TestPlatformInsertRecordsCreationProvenance(t *testing.T) {
	org := testOrg()
	engine := NewEngine(&org)
	id, err := engine.insertPlatformRecord(storage.Record{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Sprout")}})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := org.Objects["Account"].Records[id]; !ok {
		t.Fatal("platform insert did not materialize the row")
	}
	if !engine.RecordCreations.Contains(id) {
		t.Fatal("platform insert omitted creation provenance")
	}
}

func TestGeneratedUserRowsRecordCreationProvenance(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	storage.EnsureStandardObject(&org, "User")
	engine := NewEngine(&org)
	inserted := engine.Insert([]storage.Record{{Object: "User", Fields: map[string]storage.Value{
		"Username": storage.StringValue("sprout@example.invalid"), "LastName": storage.StringValue("Sprout"),
		"Alias": storage.StringValue("sprout"), "Email": storage.StringValue("sprout@example.invalid"),
		"EmailEncodingKey": storage.StringValue("UTF-8"), "LanguageLocaleKey": storage.StringValue("en_US"),
		"LocaleSidKey": storage.StringValue("en_US"), "TimeZoneSidKey": storage.StringValue("America/Los_Angeles"),
		"ProfileId": storage.IDValue(storage.DefaultProfileID), "IsActive": storage.BooleanValue(true),
	}}})[0]
	if !inserted.Success {
		t.Fatal(inserted.Error)
	}
	for _, relation := range []struct{ object, field string }{{"UserLogin", "UserId"}, {"PermissionSetAssignment", "AssigneeId"}} {
		t.Run(relation.object, func(t *testing.T) {
			found := false
			for id, record := range org.Objects[relation.object].Records {
				if idFromStorageValue(record.Fields[relation.field]) != inserted.ID {
					continue
				}
				found = true
				if !engine.RecordCreations.Contains(id) {
					t.Error("generated row omitted creation provenance")
				}
			}
			if !found {
				t.Fatal("existing generated-row hook did not materialize a row")
			}
		})
	}
}

func TestRecordCreationsRollbackPreservesEarlierInserts(t *testing.T) {
	var records RecordCreations
	first := storage.ID("001000000000001")
	second := storage.ID("001000000000002")
	records.Record(first)
	mark := records.Mark()
	records.Record(second)
	records.Forget(second)
	records.Record(second)
	if !records.Contains(first) || !records.Contains(second) {
		t.Fatal("insert tracking missing before rollback")
	}
	records.Rollback(mark)
	if !records.Contains(first) || records.Contains(second) {
		t.Fatal("rollback did not retain exactly the earlier insert")
	}
	records.Record(second)
	if !records.Contains(second) {
		t.Fatal("a later insert was not tracked after rollback")
	}
}

func TestRecordCreationsRecognizesIDSpellings(t *testing.T) {
	var records RecordCreations
	records.Record("001000000000001AAA")
	if !records.Contains("001000000000001") {
		t.Fatal("15-character spelling did not resolve inserted ID")
	}
	mark := records.Mark()
	records.Record("001000000000001")
	if records.Mark() != mark {
		t.Fatal("equivalent spelling introduced a second insertion")
	}
}

func TestFlowFailedInsertRewindsRecordCreations(t *testing.T) {
	org := testOrg()
	object := org.Objects["Account"]
	object.Definition.FlowRules = []storage.FlowRule{{Name: "RejectInsert", Active: true, TriggerType: "RecordAfterSave", Steps: []storage.FlowStep{{Kind: "customError", CustomError: storage.FlowCustomError{Name: "Reject", Messages: []storage.FlowCustomErrorMessage{{Message: "Synthetic rejection"}}}}}}}
	org.Objects["Account"] = object
	engine := NewEngine(&org)
	result := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Sprout")}}})
	if result[0].Success {
		t.Fatal("flow-rejected insert succeeded")
	}
	if engine.RecordCreations.Mark() != 0 {
		t.Fatal("failed row retained insertion provenance")
	}
	if len(org.Objects["Account"].Records) != 0 {
		t.Fatal("failed row remained in storage")
	}
}
