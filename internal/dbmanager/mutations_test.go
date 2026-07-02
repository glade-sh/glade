package dbmanager_test

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/dbmanager"
	"github.com/glade-sh/glade/internal/storage"
)

func TestManagerCreateUpdateDeleteRecordUsesDMLEngine(t *testing.T) {
	org := testManagerOrg()
	manager := dbmanager.New(&org)

	created := manager.CreateRecord("Account", dbmanager.MutationPayload{Fields: map[string]dbmanager.FieldInput{
		"Name":     {State: "value", Value: "New Account"},
		"Industry": {State: "value", Value: "Technology"},
		"OwnerId":  {State: "value", ID: "005000000000001"},
	}})
	if !created.Success || created.ID == "" || !created.Created {
		t.Fatalf("created = %#v", created)
	}

	updated := manager.UpdateRecord("Account", created.ID, dbmanager.MutationPayload{Fields: map[string]dbmanager.FieldInput{
		"Name":        {State: "value", Value: "Updated Account"},
		"Description": {State: "null"},
	}})
	if !updated.Success {
		t.Fatalf("updated = %#v", updated)
	}
	if got := org.Objects["Account"].Records[storage.ID(created.ID)].Fields["Name"].String; got != "Updated Account" {
		t.Fatalf("updated Name = %q", got)
	}
	if !org.Objects["Account"].Records[storage.ID(created.ID)].ExplicitNulls["Description"] {
		t.Fatalf("Description explicit null not recorded")
	}

	deleted := manager.DeleteRecord("Account", created.ID)
	if !deleted.Success {
		t.Fatalf("deleted = %#v", deleted)
	}
	if !org.Objects["Account"].Records[storage.ID(created.ID)].System.IsDeleted {
		t.Fatalf("record not soft-deleted")
	}
}

func TestManagerCreateRecordReturnsFieldErrors(t *testing.T) {
	org := testManagerOrg()
	manager := dbmanager.New(&org)

	result := manager.CreateRecord("Account", dbmanager.MutationPayload{Fields: map[string]dbmanager.FieldInput{
		"Description": {State: "value", Value: "Missing name"},
	}})
	if result.Success || result.StatusCode == "" || !containsString(result.Fields, "Name") {
		t.Fatalf("result = %#v", result)
	}
}

func TestManagerCreateRecordBlocksSetupObjects(t *testing.T) {
	org := testManagerOrg()
	org.Objects["Profile"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Profile",
			Label:   "Profile",
			Fields: map[string]storage.Field{
				"Id":   {APIName: "Id", Type: storage.FieldID, Createable: storage.BoolFlag(false), Updateable: storage.BoolFlag(false)},
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	manager := dbmanager.New(&org)

	result := manager.CreateRecord("Profile", dbmanager.MutationPayload{Fields: map[string]dbmanager.FieldInput{
		"Name": {State: "value", Value: "Standard User"},
	}})

	if result.Success || result.StatusCode != "INVALID_OPERATION" || !strings.Contains(result.Message, "metadata-backed") {
		t.Fatalf("result = %#v", result)
	}
	if len(org.Objects["Profile"].Records) != 0 {
		t.Fatalf("records = %#v, want none", org.Objects["Profile"].Records)
	}
}

func TestManagerUndeleteRecordRestoresSoftDeletedRecord(t *testing.T) {
	org := testManagerOrg()
	manager := dbmanager.New(&org)

	restored := manager.UndeleteRecord("Account", "001000000000002")
	if !restored.Success {
		t.Fatalf("undelete = %#v", restored)
	}
	if org.Objects["Account"].Records["001000000000002"].System.IsDeleted {
		t.Fatalf("record still deleted")
	}
}
