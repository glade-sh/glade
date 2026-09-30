package dml_test

import (
	"encoding/json"
	"testing"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
)

func recycleMembershipFixture(t *testing.T) (storage.OrgState, storage.Record) {
	t.Helper()
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Account", KeyPrefix: "001", Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}}}, Records: map[storage.ID]storage.Record{}}
	engine := dml.NewEngine(&org)
	inserted := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("retained")}}})
	if len(inserted) != 1 || !inserted[0].Success {
		t.Fatalf("insert=%#v", inserted)
	}
	row := storage.Record{Object: "Account", ID: inserted[0].ID}
	if result := engine.Delete([]storage.Record{row}); len(result) != 1 || !result[0].Success {
		t.Fatalf("delete=%#v", result)
	}
	return org, row
}

func requireRecycleMembershipQueries(t *testing.T, org storage.OrgState, row storage.Record, active, all int) {
	t.Helper()
	for _, tc := range []struct {
		allRows bool
		count   int
	}{{false, active}, {true, all}} {
		result, err := soql.Execute(org, soql.Query{Object: row.Object, Fields: []string{"Id"}, AllRows: tc.allRows, Where: &soql.Condition{Field: "Id", Op: "=", Value: storage.IDValue(row.ID)}})
		if err != nil || len(result.Records) != tc.count {
			t.Fatalf("allRows=%v count=%d err=%v", tc.allRows, len(result.Records), err)
		}
		for _, record := range result.Records {
			if !storage.IDsEqual(record.ID, row.ID) {
				t.Fatalf("wrong query identity=%s", record.ID)
			}
		}
	}
}

// Continuation assertions match the fresh Wave68 positive successor.
// Snapshot/journal checks are implementation integrity controls.
func TestRetainedRecycleMembershipTransitions(t *testing.T) {
	org, row := recycleMembershipFixture(t)
	engine := dml.NewEngine(&org)
	engine.Options.RetainEmptiedRecycleBinRecords = true
	first := engine.EmptyRecycleBin([]storage.Record{row})
	if len(first) != 1 || !first[0].Success || first[0].ID != row.ID || first[0].Errors != nil {
		t.Fatalf("first=%#v", first)
	}
	stored, exists := org.Objects[row.Object].Records[row.ID]
	if !exists || !stored.System.IsDeleted || !stored.System.RecycleBinEmptied {
		t.Fatalf("no membership transition: %#v", stored)
	}
	storage.RebuildObjectIndexes(&org, row.Object)
	requireRecycleMembershipQueries(t, org, row, 0, 1)
	data, err := json.Marshal(org)
	if err != nil {
		t.Fatal(err)
	}
	var restored storage.OrgState
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	persisted := dml.NewEngine(&restored)
	second := persisted.EmptyRecycleBin([]storage.Record{row})
	if len(second) != 1 || second[0].Success || second[0].ID != row.ID || second[0].StatusCode != "INVALID_ID_FIELD" || second[0].Error != "invalid record id; no recycle bin entry found" || len(second[0].Errors) != 1 || len(second[0].Errors[0].Fields) != 0 {
		t.Fatalf("second=%#v", second)
	}
	undone := persisted.Undelete([]storage.Record{row})
	if len(undone) != 1 || undone[0].Success || undone[0].StatusCode != "UNDELETE_FAILED" || undone[0].Error != "Entity is not in the recycle bin" {
		t.Fatalf("undelete=%#v", undone)
	}
	requireRecycleMembershipQueries(t, restored, row, 0, 1)
}

func TestRetainedRecycleMembershipRollback(t *testing.T) {
	for _, mode := range []string{"snapshot", "journal"} {
		t.Run(mode, func(t *testing.T) {
			org, row := recycleMembershipFixture(t)
			engine := dml.NewEngine(&org)
			engine.Options.RetainEmptiedRecycleBinRecords = true
			snapshot := storage.SnapshotRuntimeOrg(&org)
			journal := storage.NewIsolationJournal(&org)
			mark := journal.Mark()
			if mode == "journal" {
				engine.IsolationJournal = journal
			}
			if result := engine.EmptyRecycleBin([]storage.Record{row}); len(result) != 1 || !result[0].Success {
				t.Fatalf("empty=%#v", result)
			}
			if snapshot.Objects[row.Object].Records[row.ID].System.RecycleBinEmptied {
				t.Fatal("COW mutation contaminated snapshot")
			}
			if mode == "snapshot" {
				org = snapshot
			} else if err := journal.Rollback(mark); err != nil {
				t.Fatal(err)
			}
			if org.Objects[row.Object].Records[row.ID].System.RecycleBinEmptied {
				t.Fatal("rollback failed to restore membership")
			}
			engine = dml.NewEngine(&org)
			if result := engine.Undelete([]storage.Record{row}); len(result) != 1 || !result[0].Success {
				t.Fatalf("restored membership undelete=%#v", result)
			}
			storage.RebuildObjectIndexes(&org, row.Object)
			requireRecycleMembershipQueries(t, org, row, 1, 1)
		})
	}
}
