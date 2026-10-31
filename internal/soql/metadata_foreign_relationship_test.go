package soql

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sobject"
	"github.com/glade-sh/glade/internal/storage"
)

// Foreign references preserve the existing unmodified relationship value, even
// when the project contains a same-named object or field. This is a regression
// guard for the local resolver contract, not a new Salesforce observation.
func TestMetadataForeignRelationshipPreservesReference(t *testing.T) {
	for _, tc := range []struct{ name, object, field, wantObject, wantField string }{
		{"foreign_object", "other__Entry__c", "other__Entry__c.other__State__c", "other__Entry__c", "other__State__c"},
		{"foreign_field", "Entry__c", "Entry__c.other__State__c", "home__Entry__c", "other__State__c"},
		{"second_foreign_object", "third__Entry__c", "third__Entry__c.home__State__c", "third__Entry__c", "home__State__c"},
		{"second_foreign_field", "home__Entry__c", "home__Entry__c.third__State__c", "home__Entry__c", "third__State__c"},
		{"own", "Entry__c", "Entry__c.State__c", "home__Entry__c", "home__State__c"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sch := schema.Schema{Objects: []schema.Object{
				{Name: "Entry__c", Fields: []schema.Field{{Name: "State__c", Type: "Text"}}},
				{Name: "Link__mdt", Fields: []schema.Field{{Name: "Entity__c", Type: "MetadataRelationship", ReferenceTo: []string{"EntityDefinition"}}, {Name: "Column__c", Type: "MetadataRelationship", ReferenceTo: []string{"FieldDefinition"}}}},
			}, CustomMetadataRecords: []schema.CustomMetadataRecord{{FullName: "Link.Seed", ObjectName: "Link__mdt", DeveloperName: "Seed", Values: []schema.CustomMetadataValue{{Field: "Entity__c", Value: tc.object}, {Field: "Column__c", Value: tc.field}}}}}
			org := storage.NewOrgState()
			org.Namespace = "home"
			for name, desc := range sobject.BuildDescribeRegistry(sch).Objects {
				org.Objects[name] = storage.ObjectState{Definition: sobject.ToObjectDefinition(desc), Records: map[storage.ID]storage.Record{}}
			}
			if err := storage.ApplyCustomMetadataRecords(&org, sch.CustomMetadataRecords); err != nil {
				t.Fatal(err)
			}
			for _, c := range []struct{ name, path, want string }{{"entity", "Entity__r.QualifiedApiName", tc.wantObject}, {"field", "Column__r.QualifiedApiName", tc.wantField}} {
				t.Run(c.name, func(t *testing.T) {
					query := fmt.Sprintf("SELECT %s FROM Link__mdt", c.path)
					result, err := ParseAndExecute(org, query)
					if err != nil || len(result.Records) != 1 {
						t.Fatalf("projection: rows=%d err=%v", len(result.Records), err)
					}
					got := result.Records[0].Fields[c.path]
					if got.Kind != storage.ValueString || got.String != c.want {
						t.Errorf("projection %s = %#v, want %q", c.path, got, c.want)
					}
					query = fmt.Sprintf("SELECT %s FROM Link__mdt WHERE %s = '%s'", c.path, c.path, c.want)
					result, err = ParseAndExecute(org, query)
					if err != nil || len(result.Records) != 1 {
						t.Errorf("predicate %s: rows=%d err=%v; want 1", query, len(result.Records), err)
					}
				})
			}
		})
	}
}
