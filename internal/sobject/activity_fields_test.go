package sobject

import (
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
)

// A201/A202/A210/A211 and M004/A209/A218 establish these Activity contracts.
func TestBuildDescribeRegistryInheritsActivityFields(t *testing.T) {
	activityFields := []schema.Field{
		{
			Name: "Related__c", Type: "Lookup", Label: "Related record",
			ReferenceTo: []string{"ActivityParent__c"}, RelationshipName: "Related__r",
			ChildRelationshipName: "Activities__r", DeleteConstraint: "SetNull",
		},
		{Name: "Detail__c", Type: "Text", Length: 40, Label: "Activity detail"},
	}
	input := schema.Schema{Objects: []schema.Object{
		{Name: "Activity", Fields: activityFields},
		{Name: "Task"},
		{Name: "ActivityParent__c"},
	}}
	registry := BuildDescribeRegistry(input)
	for _, objectName := range []string{"Task", "Event"} {
		t.Run(objectName, func(t *testing.T) {
			describe, err := registry.Describe(objectName)
			if err != nil {
				t.Fatal(err)
			}
			definition := ToObjectDefinition(describe)
			field := definition.Fields["Related__c"]
			if field.Type != storage.FieldReference || field.Label != "Related record" ||
				!reflect.DeepEqual(field.ReferenceTo, []string{"ActivityParent__c"}) ||
				field.RelationshipName != "Related__r" {
				t.Fatalf("inherited reference metadata: %#v", field)
			}
			found := false
			for _, relationship := range definition.Relations {
				if relationship.Field != "Related__c" {
					continue
				}
				found = true
				if !relationship.SetNullOnDelete || relationship.ParentRelationship != "Related__r" {
					t.Fatalf("M004/A209/A218 SetNull metadata: %#v", relationship)
				}
			}
			if !found {
				t.Fatal("inherited lookup relationship missing")
			}
			if field := definition.Fields["Detail__c"]; field.Length != 40 || field.Label != "Activity detail" {
				t.Fatalf("A202/A211 inherited text metadata: %#v", field)
			}
		})
	}
	if len(input.Objects) != 3 || len(input.Objects[1].Fields) != 0 || input.Objects[0].Fields[1].Length != 40 {
		t.Fatal("runtime inheritance mutated the source schema")
	}
}
