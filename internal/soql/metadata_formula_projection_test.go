package soql

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestExecuteFieldDefinitionCalculatedProjection(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["CatalogEntry__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName: "CatalogEntry__c",
		Fields: map[string]storage.Field{
			"Computed__c": {APIName: "Computed__c", Type: storage.FieldCalculated, Formula: "1 + 2"},
			"Stored__c":   {APIName: "Stored__c", Type: storage.FieldString},
		},
	}}
	// Tooling API FieldDefinition.IsCalculated is a Boolean identifying a
	// calculated field (Tooling API Developer Guide, FieldDefinition).
	for _, tc := range []struct {
		name string
		want bool
	}{
		{"Computed__c", true},
		{"Stored__c", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ParseAndExecute(org, "SELECT QualifiedApiName, IsCalculated FROM FieldDefinition WHERE EntityDefinitionId = 'CatalogEntry__c' AND QualifiedApiName = '"+tc.name+"'")
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Records) != 1 {
				t.Fatalf("field rows = %d, want 1", len(result.Records))
			}
			got := result.Records[0].Fields["IsCalculated"]
			if got.Kind != storage.ValueBoolean || got.Boolean != tc.want {
				t.Fatalf("IsCalculated = %#v, want Boolean %v", got, tc.want)
			}
		})
	}
	for _, tc := range []struct {
		name, filter, wantField string
	}{
		{"formula", "IsCalculated = TRUE", "Computed__c"},
		{"stored", "IsCalculated = FALSE", "Stored__c"},
		{"empty", "IsCalculated = TRUE AND QualifiedApiName = 'Stored__c'", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ParseAndExecute(org, "SELECT QualifiedApiName, (SELECT QualifiedApiName FROM Fields WHERE "+tc.filter+") FROM EntityDefinition WHERE QualifiedApiName = 'CatalogEntry__c'")
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Records) != 1 {
				t.Fatalf("parent rows = %d, want 1", len(result.Records))
			}
			children, queried := result.Records[0].Children["Fields"]
			if !queried {
				t.Fatal("queried child relationship missing")
			}
			if tc.wantField == "" {
				if len(children) != 0 {
					t.Fatalf("empty filter children = %#v", children)
				}
				return
			}
			if len(children) != 1 || children[0].Fields["QualifiedApiName"].String != tc.wantField {
				t.Fatalf("children = %#v, want %s", children, tc.wantField)
			}
		})
	}
}
