package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Queried Name rejects even when unassigned: case349_query_trigger/org.tsv:12-15,55-58.
func TestCase349DirectPersonUnchangedNameUpdate(t *testing.T) {
	for _, field := range []string{"PersonHomePhone", "PersonMobilePhone"} {
		t.Run(field, func(t *testing.T) {
			org := testOrg()
			storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
			storage.EnsureStandardObject(&org, "Contact")
			engine := NewEngine(&org)
			inserted := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"LastName": storage.StringValue("Synthetic")}}})
			if len(inserted) != 1 || !inserted[0].Success {
				t.Fatal(inserted)
			}
			candidate := storage.Record{Object: "Account", ID: inserted[0].ID, Fields: map[string]storage.Value{"Name": storage.StringValue("Synthetic"), field: storage.StringValue("4155550123")}}
			marker, ok := any(&candidate).(interface{ MarkDMLNameUnassigned() })
			if !ok {
				t.Fatal("queried-name provenance is unavailable")
			}
			marker.MarkDMLNameUnassigned()
			result := engine.Update([]storage.Record{candidate})
			if len(result) != 1 || result[0].Success || result[0].StatusCode != "INVALID_FIELD_FOR_INSERT_UPDATE" || result[0].Error != "Unable to create/update fields: Name. Please check the security settings of this field and verify that it is read/write for your profile or permission set." {
				t.Fatalf("queried Name result: %#v", result)
			}
		})
	}
}
