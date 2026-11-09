package soql

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// The native api68_boundary/org.tsv observes rejection at 62 and 67 and
// acceptance at 68. Query cache entries must not cross those source versions.
func TestAPI68RelationshipExecutionCacheVersionBoundary(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	query, err := Parse("SELECT Id,(SELECT Id FROM ContactPointOttSvcs LIMIT 1) FROM Account LIMIT 1")
	if err != nil {
		t.Fatal(err)
	}
	cache := NewExecutionCache()
	for _, api := range []string{"67.0", "68.0", "62.0", "68.0"} {
		_, err := ExecuteWithCacheAndContext(org, query, cache, ExecutionContext{APIVersion: api})
		if got, want := err == nil, api == "68.0"; got != want {
			t.Errorf("API %s success = %t, want %t: %v", api, got, want, err)
		}
	}
}

func TestAPI68LoadedRelationshipExecutionCacheVersionBoundary(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	org.Objects["ContactPointOttSvc"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "ContactPointOttSvc", Fields: map[string]storage.Field{"Id": {APIName: "Id", Type: storage.FieldID}}, Relations: []storage.Relationship{{Field: "ParentId", ParentObjects: []string{"Account", "Individual"}, ChildRelationship: "ContactPointOttSvcs", CascadeDelete: true}}}}
	query, err := Parse("SELECT Id,(SELECT Id FROM ContactPointOttSvcs LIMIT 1) FROM Account LIMIT 1")
	if err != nil {
		t.Fatal(err)
	}
	cache := NewExecutionCache()
	for _, api := range []string{"67.0", "68.0", "62.0", "68.0"} {
		_, err := ExecuteWithCacheAndContext(org, query, cache, ExecutionContext{APIVersion: api})
		if got, want := err == nil, api == "68.0"; got != want {
			t.Errorf("API %s loaded success = %t, want %t: %v", api, got, want, err)
		}
	}
}

func TestAPI68RelationshipProjectionStaysChildScoped(t *testing.T) {
	org := storage.NewOrgState()
	org.APIVersion = "68.0"
	if _, _, ok := orgWithLazyStandardObject(org, "ContactPointOttSvc"); ok {
		t.Fatal("child-only projection exposed through generic object hydration")
	}
}
