package dml

import (
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestPermissionSetGroupInsertCreatesAssociatedPermissionSet(t *testing.T) {
	org := permissionGroupProjectionOrg()
	engine := NewEngine(&org)
	result := engine.Insert([]storage.Record{permissionGroupProjectionInput()})
	if len(result) != 1 || !result[0].Success {
		t.Fatalf("group insert=%#v", result)
	}
	linked := permissionGroupProjectionChildren(org, result[0].ID)
	if len(linked) != 1 {
		t.Fatalf("associated permission sets=%d, want 1", len(linked))
	}
	expected := map[string]storage.Value{
		"Name": storage.StringValue("OwnedGroup"), "Label": storage.StringValue("Distinct group label"),
		"Type": storage.StringValue("Group"), "PermissionSetGroupId": storage.IDValue(result[0].ID),
		"IsCustom": storage.BooleanValue(true), "IsOwnedByProfile": storage.BooleanValue(false),
	}
	for name, want := range expected {
		if got := linked[0].Fields[name]; !reflect.DeepEqual(got, want) {
			t.Fatalf("associated %s=%#v, want %#v", name, got, want)
		}
	}
	for _, name := range []string{"NamespacePrefix", "LicenseId"} {
		if value, present := linked[0].GetField(name); present && value.Kind != storage.ValueNull {
			t.Fatalf("associated %s=%#v, want null", name, value)
		}
	}
}

func TestPermissionSetGroupInsertPreservesStandaloneRecord(t *testing.T) {
	org := permissionGroupProjectionOrg()
	engine := NewEngine(&org)
	result := engine.Insert([]storage.Record{{Object: "PermissionSet", Fields: map[string]storage.Value{"Name": storage.StringValue("OwnedStandalone"), "Label": storage.StringValue("Standalone label")}}})
	if len(result) != 1 || !result[0].Success {
		t.Fatalf("standalone insert=%#v", result)
	}
	original := org.Objects["PermissionSet"].Records[result[0].ID].Clone()
	groups := engine.Insert([]storage.Record{permissionGroupProjectionInput()})
	if len(groups) != 1 || !groups[0].Success {
		t.Fatalf("group insert=%#v", groups)
	}
	if got := org.Objects["PermissionSet"].Records[result[0].ID]; !reflect.DeepEqual(got, original) {
		t.Fatalf("standalone permission set changed: before=%#v after=%#v", original, got)
	}
}

func TestPermissionSetGroupAssociatedInsertFailureRollsBackParent(t *testing.T) {
	for _, journal := range []bool{false, true} {
		t.Run(map[bool]string{false: "snapshot", true: "journal"}[journal], func(t *testing.T) {
			org := permissionGroupProjectionOrg()
			engine := NewEngine(&org)
			engine.IDs.Prefixes["PermissionSet"] = "invalid"
			if journal {
				engine.IsolationJournal = storage.NewIsolationJournal(&org)
			}
			result := engine.Insert([]storage.Record{permissionGroupProjectionInput()})
			if len(result) != 1 || result[0].Success {
				t.Fatalf("expected dependent insert failure, got %#v", result)
			}
			if len(org.Objects["PermissionSetGroup"].Records) != 0 || len(org.Objects["PermissionSet"].Records) != 0 {
				t.Fatal("dependent insert failure retained parent or child")
			}
		})
	}
}

func TestPermissionSetGroupProjectionParticipatesInIsolationRollback(t *testing.T) {
	org := permissionGroupProjectionOrg()
	engine := NewEngine(&org)
	journal := storage.NewIsolationJournal(&org)
	engine.IsolationJournal = journal
	mark := journal.Mark()
	result := engine.Insert([]storage.Record{permissionGroupProjectionInput()})
	if len(result) != 1 || !result[0].Success {
		t.Fatalf("group insert=%#v", result)
	}
	if len(permissionGroupProjectionChildren(org, result[0].ID)) != 1 {
		t.Fatal("missing associated permission set before rollback")
	}
	if err := journal.Rollback(mark); err != nil {
		t.Fatal(err)
	}
	if len(org.Objects["PermissionSetGroup"].Records) != 0 || len(org.Objects["PermissionSet"].Records) != 0 {
		t.Fatal("isolation rollback retained parent or child")
	}
}

func permissionGroupProjectionOrg() storage.OrgState {
	org := storage.NewOrgState()
	for _, name := range []string{"PermissionSetGroup", "PermissionSet"} {
		storage.EnsureStandardObject(&org, name)
	}
	return org
}
func permissionGroupProjectionInput() storage.Record {
	return storage.Record{Object: "PermissionSetGroup", Fields: map[string]storage.Value{"DeveloperName": storage.StringValue("OwnedGroup"), "MasterLabel": storage.StringValue("Distinct group label")}}
}
func permissionGroupProjectionChildren(org storage.OrgState, id storage.ID) []storage.Record {
	var children []storage.Record
	for _, record := range org.Objects["PermissionSet"].Records {
		value, _ := record.GetField("PermissionSetGroupId")
		if storage.IDsEqual(idFromStorageValue(value), id) {
			children = append(children, record)
		}
	}
	return children
}

// This is a preservation control for an uncaptured setup context.
func TestPermissionSetGroupProjectionPreservesNamespaceBoundary(t *testing.T) {
	org := permissionGroupProjectionOrg()
	org.Namespace = "owned"
	engine := NewEngine(&org)
	result := engine.Insert([]storage.Record{permissionGroupProjectionInput()})
	if len(result) != 1 || !result[0].Success {
		t.Fatalf("group insert=%#v", result)
	}
	if len(permissionGroupProjectionChildren(org, result[0].ID)) != 0 {
		t.Fatal("changed uncaptured namespace setup projection")
	}
}
