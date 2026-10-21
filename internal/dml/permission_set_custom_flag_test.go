package dml

import (
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestPermissionSetInsertMarksAuthoredRecordCustom(t *testing.T) {
	org := permissionGroupProjectionOrg()
	engine := NewEngine(&org)
	results := engine.Insert([]storage.Record{{Object: "PermissionSet", Fields: map[string]storage.Value{"Name": storage.StringValue("OwnedStandalone"), "Label": storage.StringValue("Distinct standalone label")}}})
	if len(results) != 1 || !results[0].Success {
		t.Fatalf("permission set insert=%#v", results)
	}
	row := org.Objects["PermissionSet"].Records[results[0].ID]
	if got := row.Fields["IsCustom"]; got.Kind != storage.ValueBoolean || !got.Boolean {
		t.Fatalf("new permission set IsCustom=%#v, want true", got)
	}
}

func TestPermissionSetInsertPreservesSeededProfileFlags(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	before := map[storage.ID]storage.Record{}
	for id, row := range org.Objects["PermissionSet"].Records {
		before[id] = row.Clone()
	}
	if len(before) == 0 {
		t.Fatal("missing seeded control records")
	}
	engine := NewEngine(&org)
	results := engine.Insert([]storage.Record{{Object: "PermissionSet", Fields: map[string]storage.Value{"Name": storage.StringValue("OwnedStandalone"), "Label": storage.StringValue("Distinct standalone label")}}})
	if len(results) != 1 || !results[0].Success {
		t.Fatalf("permission set insert=%#v", results)
	}
	for id, want := range before {
		if got := org.Objects["PermissionSet"].Records[id]; !reflect.DeepEqual(got, want) {
			t.Fatalf("seeded permission set %s changed", id)
		}
	}
}
