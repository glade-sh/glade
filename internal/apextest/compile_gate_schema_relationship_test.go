package apextest

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
)

// Matching native shapes: ../sema/testdata/schema_relationship_gate/org.tsv:2-26,
// 147-171 (API 62/67 rejection), with loaded and named twins in the same table.
// Owner/What/Who custom-field queries are natively rejected even with valid
// metadata; reference ownership is distinct from query validity.
func TestSchemaReferenceGateRelationshipQueryRoutes(t *testing.T) {
	for _, tc := range []struct {
		route, query, target string
		want                 bool
	}{
		{"parent_select", "SELECT Account.Destination__c FROM Contact", "Account", true},
		{"parent_multihop", "SELECT Account.Parent.Destination__c FROM Contact", "Account", true},
		{"parent_where", "SELECT Id FROM Contact WHERE Account.Destination__c != null", "Account", true},
		{"parent_order", "SELECT Id FROM Contact ORDER BY Account.Destination__c", "Account", true},
		{"parent_group_having", "SELECT COUNT(Id) FROM Contact GROUP BY Account.Destination__c HAVING Account.Destination__c != null", "Account", true},
		{"parent_alias", "SELECT c.Account.Destination__c FROM Contact c", "Account", true},
		{"parent_object_qualifier", "SELECT Contact.Account.Destination__c FROM Contact", "Account", true},
		{"custom_derived_relationship", "SELECT Client__r.Destination__c FROM Parcel__c", "Account", true},
		{"custom_target_not_spelled", "SELECT Parcel__r.Destination__c FROM Shipment__c", "Parcel__c", true},
		{"owner_user", "SELECT Owner.Destination__c FROM Case", "User", true},
		{"owner_group", "SELECT Owner.Name FROM Case", "Group", true},
		{"what_account", "SELECT What.Destination__c FROM Task", "Account", true},
		{"what_opportunity", "SELECT What.Name FROM Task", "Opportunity", true},
		{"who_contact", "SELECT Who.Destination__c FROM Task", "Contact", true},
		{"who_lead", "SELECT Who.Name FROM Task", "Lead", true},
		{"typeof_when", "SELECT TYPEOF What WHEN Account THEN Destination__c ELSE Name END FROM Task", "Account", true},
		{"typeof_parent_hop", "SELECT TYPEOF What WHEN Opportunity THEN Account.Destination__c END FROM Task", "Account", true},
		{"child_query", "SELECT Id, (SELECT Owner.Destination__c FROM Contacts) FROM Account", "User", true},
		{"semi_join", "SELECT Id FROM User WHERE Id IN (SELECT OwnerId FROM Contact WHERE Account.Destination__c != null)", "Account", true},
		{"sosl_returning", "FIND 'x' RETURNING Contact(Account.Destination__c)", "Account", true},
		{"same_spelled_leaf", "SELECT Destination__c FROM Contact", "Account", false},
		{"user_account_parent", "SELECT Account.Destination__c FROM User", "Account", true},
		{"string_literal", "SELECT Id FROM Contact WHERE LastName = 'Account.Destination__c'", "Account", false},
		{"bind_variable", "SELECT Id FROM Contact WHERE LastName = :Account", "Account", false},
		{"sosl_user_account_parent", "FIND 'x' RETURNING Contact(Id), User(Account.Destination__c)", "Account", true},
	} {
		t.Run(tc.route, func(t *testing.T) {
			source := `public class Probe { static void run(String Account) { Object items = [` + tc.query + `]; } }`
			index := typesys.Index{Objects: schemaRelationshipGateMetadata(tc.target).Objects}
			got := compileGateReferencesSchemaObject("Probe.cls", source, newCompileGateUnits(index), tc.target)
			if got != tc.want {
				t.Fatalf("relationship target %s referenced=%t want=%t: %s", tc.target, got, tc.want, tc.query)
			}
		})
	}
}

func schemaRelationshipGateMetadata(target string) schema.Schema {
	objects := []schema.Object{
		{Name: "Parcel__c", Fields: []schema.Field{{Name: "Client__c", Type: "Lookup", ReferenceTo: []string{"Account"}, RelationshipName: "Parcels"}}},
		{Name: "Shipment__c", Fields: []schema.Field{{Name: "Parcel__c", Type: "Lookup", ReferenceTo: []string{"Parcel__c"}, RelationshipName: "Shipments"}}},
	}
	bad := schema.Field{Name: "Destination__c", Type: "Lookup", ReferenceTo: []string{"UnavailableDestination"}}
	for i := range objects {
		if objects[i].Name == target {
			objects[i].Fields = append(objects[i].Fields, bad)
			return schema.Schema{Objects: objects}
		}
	}
	return schema.Schema{Objects: append(objects, schema.Object{Name: target, Fields: []schema.Field{bad}})}
}

func TestSchemaReferenceGateRelationshipRunnerProjectRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range []struct{ route, query, target string }{
			{"parent", "SELECT Account.Destination__c FROM Contact", "Account"},
			{"custom", "SELECT Client__r.Destination__c FROM Parcel__c", "Account"},
			{"owner", "SELECT Owner.Destination__c FROM Case", "User"},
			{"what", "SELECT What.Destination__c FROM Task", "Account"},
			{"who", "SELECT Who.Destination__c FROM Task", "Contact"},
		} {
			t.Run(api+"/"+tc.route, func(t *testing.T) {
				root := t.TempDir()
				var paths []string
				for name, source := range map[string]string{
					"RelationshipUse.cls":  `public class RelationshipUse { public static Boolean value() { return true; } public static void query() { Object items = [` + tc.query + `]; } }`,
					"RelationshipTest.cls": `@IsTest private class RelationshipTest { @IsTest static void observe() { System.assert(RelationshipUse.value()); } }`,
					"IndependentTest.cls":  `@IsTest private class IndependentTest { @IsTest static void observe() { System.assertEquals(7,7); } }`,
				} {
					path := filepath.Join(root, name)
					writeFile(t, path, source)
					paths = append(paths, path)
				}
				index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: paths}, schemaRelationshipGateMetadata(tc.target))
				for _, cached := range []bool{false, true} {
					run := RunCasesContext(context.Background(), index, Options{NoDiskCache: !cached}, nil)
					got := map[string]testreport.Status{}
					for _, suite := range run.Suites {
						for _, c := range suite.Cases {
							got[c.ClassName] = c.Status
						}
					}
					if len(got) != 2 || got["IndependentTest"] != testreport.StatusPass || got["RelationshipTest"] != testreport.StatusCompileError {
						t.Fatalf("cached=%t relationship gate statuses=%v", cached, got)
					}
				}
			})
		}
	}
}
