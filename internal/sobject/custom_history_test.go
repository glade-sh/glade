package sobject

import (
	"testing"

	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
)

func TestBuildDescribeRegistryHistoryFacetsStayScoped(t *testing.T) {
	plain := schema.Object{Name: "PlainParcel__c", Fields: []schema.Field{{Name: "Value__c", Type: "Text", Createable: storage.BoolFlag(false), Updateable: storage.BoolFlag(false), Nillable: storage.BoolFlag(false), DefaultedOnCreate: storage.BoolFlag(true)}}}
	registry := BuildDescribeRegistry(schema.Schema{Objects: []schema.Object{plain}})
	field := registry.Objects[plain.Name].Fields["Value__c"]
	// Ordinary snapshots retain their existing facet import policy. History's
	// intrinsic read-only facets belong only to the derived platform table.
	if field.Createable != nil || field.Updateable != nil || field.Nillable != nil || field.DefaultedOnCreate != nil {
		t.Fatalf("ordinary snapshot facets changed: %#v", field)
	}
}

func TestBuildDescribeRegistryHistoryPreservesParentPrefix(t *testing.T) {
	objects := []schema.Object{{Name: "TrackedParcel__c", Label: "Audit parcel"}, {Name: "PlainParcel__c"}}
	before := BuildDescribeRegistry(schema.Schema{Objects: objects})
	objects[0].EnableHistory = true
	after := BuildDescribeRegistry(schema.Schema{Objects: objects})
	for _, name := range []string{"TrackedParcel__c", "PlainParcel__c"} {
		if before.Objects[name].KeyPrefix != after.Objects[name].KeyPrefix {
			t.Fatalf("history renumbered %s", name)
		}
	}
	history, ok := after.Objects["TrackedParcel__History"]
	if !ok || len(history.Fields) != 9 || history.Label != "History: Audit parcel" {
		t.Fatalf("history shape: %#v", history)
	}
	for _, field := range history.Fields {
		if storage.FieldFlagValue(field.Createable, true) || storage.FieldFlagValue(field.Updateable, true) {
			t.Fatalf("history field became writable: %#v", field)
		}
	}
	if _, ok := after.Objects["PlainParcel__History"]; ok {
		t.Fatal("disabled parent acquired history")
	}
}
