package sobject

import (
	"github.com/glade-sh/glade/internal/schema"
	"testing"
)

func TestRelationshipOrderDescribeStorageRoundTrip(t *testing.T) {
	order := 0
	input := schema.Schema{Objects: []schema.Object{{Name: "OwnedDetail__c", SharingModel: "ControlledByParent", Fields: []schema.Field{{Name: "Parent__c", Type: "MasterDetail", ReferenceTo: []string{"Account"}, RelationshipName: "Parent__r", RelationshipOrder: &order}}}}}
	registry := BuildDescribeRegistry(input)
	described, ok := registry.Objects["OwnedDetail__c"]
	if !ok {
		t.Fatal("detail missing")
	}
	field, ok := described.Fields["Parent__c"]
	if !ok || field.RelationshipOrder == nil || *field.RelationshipOrder != 0 {
		t.Fatal("describe lost explicit order zero")
	}
	definition := ToObjectDefinition(described)
	restored := FromObjectDefinition(definition)
	if restored.Fields["Parent__c"].RelationshipOrder == nil || *restored.Fields["Parent__c"].RelationshipOrder != 0 {
		t.Fatal("storage round trip lost relationship order")
	}
	order = 1
	if *restored.Fields["Parent__c"].RelationshipOrder != 0 {
		t.Fatal("order metadata aliases source pointer")
	}
	if _, exists := restored.Fields["OwnerId"]; exists {
		t.Fatal("master detail owner field restored")
	}
}
