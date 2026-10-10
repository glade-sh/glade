package sobject

import (
	"testing"

	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
)

func TestMasterDetailReparentabilityControlsDescribeAndStorageWritability(t *testing.T) {
	registry := BuildDescribeRegistry(schema.Schema{Objects: []schema.Object{{
		Name: "OwnedDetail__c",
		Fields: []schema.Field{
			{Name: "Enabled__c", Type: "MasterDetail", ReferenceTo: []string{"Account"}, ReparentableMasterDetail: true},
			{Name: "Disabled__c", Type: "MasterDetail", ReferenceTo: []string{"Account"}},
		},
	}}})

	describe, err := registry.Describe("OwnedDetail__c")
	if err != nil {
		t.Fatal(err)
	}
	definition := ToObjectDefinition(describe)
	for field, want := range map[string]bool{"Enabled__c": true, "Disabled__c": false} {
		if got := storage.FieldFlagValue(describe.Fields[field].Updateable, false); got != want {
			t.Errorf("describe %s updateable = %v, want %v", field, got, want)
		}
		if got := storage.FieldFlagValue(definition.Fields[field].Updateable, false); got != want {
			t.Errorf("definition %s updateable = %v, want %v", field, got, want)
		}
	}
}
