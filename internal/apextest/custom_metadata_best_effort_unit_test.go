package apextest

import (
	"testing"

	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
)

func TestProjectMetadataBestEffortLoadKeepsParentBatch(t *testing.T) {
	for _, parent := range []string{"ShortParent", "FifteenCharsPar", "LongNamedParentRec"} {
		t.Run(parent, func(t *testing.T) {
			org := storage.NewOrgState()
			parentDefinition := storage.ObjectDefinition{APIName: "ParentRow__mdt", Fields: map[string]storage.Field{}}
			childDefinition := storage.ObjectDefinition{APIName: "ChildRow__mdt", Fields: map[string]storage.Field{
				"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"ParentRow__mdt"}, RelationshipName: "Parent__r"},
			}}
			storage.EnsureStandardObjectFields(&parentDefinition)
			storage.EnsureStandardObjectFields(&childDefinition)
			org.Objects[parentDefinition.APIName] = storage.ObjectState{Definition: parentDefinition}
			org.Objects[childDefinition.APIName] = storage.ObjectState{Definition: childDefinition}
			applyCustomMetadataRecordsBestEffort(&org, []schema.CustomMetadataRecord{
				{ObjectName: "ChildRow__mdt", DeveloperName: "Child", FullName: "ChildRow.Child", File: "child.xml", Values: []schema.CustomMetadataValue{{Field: "Parent__c", Value: parent}}},
				{ObjectName: "ParentRow__mdt", DeveloperName: parent, FullName: "ParentRow." + parent, File: "parent.xml"},
				{ObjectName: "UnknownRow__mdt", DeveloperName: "Stray", File: "stray.xml", Values: []schema.CustomMetadataValue{{Field: "Unknown__c", Value: "true"}}},
				{ObjectName: "UnknownRow__mdt", DeveloperName: "OtherStray", File: "other-stray.xml", Values: []schema.CustomMetadataValue{{Field: "OtherUnknown__c", Value: "true"}}},
			})
			parents, children := org.Objects["ParentRow__mdt"].Records, org.Objects["ChildRow__mdt"].Records
			if len(parents) != 1 || len(children) != 1 {
				t.Fatalf("parent/child counts: %d/%d", len(parents), len(children))
			}
			for _, child := range children {
				if _, present := parents[child.Fields["Parent__c"].ID]; !present {
					t.Fatalf("child parent reference %v does not resolve by developer name", child.Fields["Parent__c"])
				}
			}
		})
	}
}
