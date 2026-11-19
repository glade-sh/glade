package soql

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// syncVirtualEntitySharingModels must copy on write. A runtime clone shares
// its EntityDefinition rows with the template org, so the projected
// InternalSharingModel row must land in the clone's own record map while
// the template's row and definition stay untouched.
func TestEntitySharingSyncOnCloneLeavesTemplateUnchanged(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	account := org.Objects["Account"]
	account.Definition.APIName = "Account"
	account.Definition.SharingModel = "ReadWrite"
	org.Objects["Account"] = account
	storage.EnsureStandardObject(&org, "EntityDefinition")
	entities := org.Objects["EntityDefinition"]
	entities.SetRecords(map[storage.ID]storage.Record{
		storage.ID("Account"): {ID: storage.ID("Account"), Object: "EntityDefinition", Fields: map[string]storage.Value{
			"QualifiedApiName": storage.StringValue("Account"),
		}},
	})
	org.Objects["EntityDefinition"] = entities

	clone := storage.NewRuntimeTemplate(org).CloneSnapshotOrg()
	if !clone.Objects["EntityDefinition"].RecordsShared {
		t.Fatal("EntityDefinition rows are not shared with the clone")
	}
	syncVirtualEntitySharingModels(&clone)

	if got := clone.Objects["EntityDefinition"].Records[storage.ID("Account")].Fields["InternalSharingModel"].String; got != "ReadWrite" {
		t.Fatalf("clone InternalSharingModel = %q, want ReadWrite", got)
	}
	template := org.Objects["EntityDefinition"].Records[storage.ID("Account")]
	if _, leaked := template.Fields["InternalSharingModel"]; leaked {
		t.Fatalf("projected sharing model leaked into the shared template row: %+v", template.Fields["InternalSharingModel"])
	}
	if _, leaked := org.Objects["EntityDefinition"].Definition.Metadata[virtualEntitySharingKeyPrefix+"Account"]; leaked {
		t.Fatal("generated sharing marker leaked into the template definition")
	}
}
