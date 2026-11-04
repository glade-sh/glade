package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// Queried Name rejects on person updates: case349_component_query/org.tsv:2-3,6-7,11-12,15-16.
func TestCase349QueriedNameWithChangedComponents(t *testing.T) {
	for _, field := range []string{"FirstName", "LastName"} {
		t.Run(field, func(t *testing.T) {
			org := testOrg()
			storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
			storage.EnsureStandardObject(&org, "Contact")
			engine := NewEngine(&org)
			inserted := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"LastName": storage.StringValue("Synthetic")}}})
			if len(inserted) != 1 || !inserted[0].Success {
				t.Fatal(inserted)
			}
			candidate := storage.Record{Object: "Account", ID: inserted[0].ID, Fields: map[string]storage.Value{"Name": storage.StringValue("Synthetic"), field: storage.StringValue("Changed")}}
			candidate.MarkDMLNameUnassigned()
			result := engine.Update([]storage.Record{candidate})
			if len(result) != 1 || result[0].Success || result[0].StatusCode != "INVALID_FIELD_FOR_INSERT_UPDATE" {
				t.Fatalf("queried Name result: %#v", result)
			}
		})
	}
}
