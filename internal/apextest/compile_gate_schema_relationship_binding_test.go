package apextest

import (
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
	"testing"
)

func TestSchemaReferenceGateRelationshipBindingRoutes(t *testing.T) {
	for _, tc := range []struct {
		route, query, target, namespace string
		want                            bool
	}{
		{"from_relationship_alias", "SELECT a.Destination__c FROM Contact c, c.Account a", "Account", "", true},
		{"custom_owner", "SELECT Owner.Destination__c FROM Parcel__c", "User", "", true},
		{"custom_created_by", "SELECT CreatedBy.Destination__c FROM Parcel__c", "User", "", true},
		{"custom_modified_by", "SELECT LastModifiedBy.Destination__c FROM Parcel__c", "User", "", true},
		{"namespace_local_hop", "SELECT Client__r.Destination__c FROM Parcel__c", "Account", "pkg", true},
		{"namespace_qualified_hop", "SELECT pkg__Client__r.Destination__c FROM pkg__Parcel__c", "Account", "pkg", true},
		{"namespace_root_qualifier", "SELECT Parcel__c.Client__r.Destination__c FROM Parcel__c", "Account", "pkg", true},
		{"namespace_other_hop", "SELECT other__Client__r.Destination__c FROM Parcel__c", "Account", "pkg", false},
		{"child_query_alias", "SELECT Id, (SELECT c.Owner.Destination__c FROM Contacts c) FROM Account", "User", "", true},
		{"typeof_unselected_target", "SELECT TYPEOF Who WHEN Contact THEN Name END FROM Task", "Lead", "", false},
	} {
		t.Run(tc.route, func(t *testing.T) {
			t.Parallel()
			metadata := schemaRelationshipGateMetadata(tc.target)
			if tc.namespace != "" {
				metadata.Objects[0].Name = "pkg__Parcel__c"
				metadata.Objects[0].Fields[0].Name = "pkg__Client__c"
			}
			index := typesys.Index{Objects: metadata.Objects, Project: typesys.ProjectInfo{Namespace: tc.namespace}}
			source := `trigger RelationshipTrigger on Shipment__c (before insert) { Object items = [` + tc.query + `]; }`
			if got := compileGateReferencesSchemaObject("RelationshipTrigger.trigger", source, newCompileGateUnits(index), tc.target); got != tc.want {
				t.Fatalf("target %s referenced=%t want=%t: %s", tc.target, got, tc.want, tc.query)
			}
		})
	}
}

func TestSchemaReferenceGateRelationshipMasterDetailOwnerControl(t *testing.T) {
	index := typesys.Index{Objects: []schema.Object{{Name: "Detail__c", SharingModel: "ControlledByParent", Fields: []schema.Field{{Name: "Parent__c", Type: "MasterDetail", ReferenceTo: []string{"Account"}}}}}}
	source := `public class Probe { Object records = [SELECT Owner.Name FROM Detail__c]; }`
	if compileGateReferencesSchemaObject("Probe.cls", source, newCompileGateUnits(index), "User") {
		t.Fatal("master-detail child has no Owner relationship")
	}
}

func TestSchemaReferenceGateRelationshipFeatureRoutes(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		index := typesys.Index{}
		if enabled {
			index.OrgShapeFeatures = []string{"PersonAccounts"}
		}
		source := `public class Probe { Object records = [SELECT PersonContact.Destination__c FROM Account]; }`
		if got := compileGateReferencesSchemaObject("Probe.cls", source, newCompileGateUnits(index), "Contact"); got != enabled {
			t.Errorf("PersonAccounts=%t target referenced=%t", enabled, got)
		}
	}
}
