package soql

import (
	"encoding/json"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// The source-backed direct query stays at its origin/main result for an
// uncaptured neighbor, even if the physical inventory is made eager-equivalent.
func TestSourceSpreadDirectQueryPreservesNeighbor(t *testing.T) {
	for _, tc := range []struct {
		name     string
		features []string
	}{
		{"default", []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, project.PlainDeveloperDescribeShape}},
		{"optional", []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, "EnableSetPasswordInApi"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := storage.NewOrgState()
			org.EnableStandardSpreadSource()
			storage.ApplyProjectOrgShape(&org, tc.features)
			query, err := Parse("SELECT Id FROM CareProgram LIMIT 2")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Execute(org, query); err == nil {
				t.Fatal("origin/main rejected the uncaptured neighbor query")
			}
		})
	}
}

func TestSourceSpreadDirectQueryPreservesEditedNeighbor(t *testing.T) {
	org := storage.NewOrgState()
	org.EnableStandardSpreadSource()
	storage.ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, project.PlainDeveloperDescribeShape})
	if _, ok := storage.EnsureLazyStandardObject(&org, "CareProgram"); !ok {
		t.Fatal("inventory object was not materialized")
	}
	object := org.Objects["CareProgram"]
	original := object.Definition.Fields["Id"]
	defer func() { object.Definition.Fields["Id"] = original }()
	edited := original
	edited.Label = "edited"
	object.Definition.Fields["Id"] = edited
	query, err := Parse("SELECT Id FROM CareProgram LIMIT 2")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(org, query); err != nil {
		t.Fatalf("edited definition was hidden: %v", err)
	}
}

func TestSourceSpreadDirectQueryPreservesNeighborAfterSerialization(t *testing.T) {
	org := storage.NewOrgState()
	org.EnableStandardSpreadSource()
	storage.ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, project.PlainDeveloperDescribeShape})
	if _, ok := storage.EnsureLazyStandardObject(&org, "CareProgram"); !ok {
		t.Fatal("inventory object was not materialized")
	}
	data, err := json.Marshal(org)
	if err != nil {
		t.Fatal(err)
	}
	var restored storage.OrgState
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	query, err := Parse("SELECT Id FROM CareProgram LIMIT 2")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(restored, query); err == nil {
		t.Fatal("serialized origin/main neighbor query became visible")
	}
}

func TestSourceSpreadDirectQueryPreservesNeighborRecords(t *testing.T) {
	org := storage.NewOrgState()
	org.EnableStandardSpreadSource()
	storage.ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, project.PlainDeveloperDescribeShape})
	if _, ok := storage.EnsureLazyStandardObject(&org, "CareProgram"); !ok {
		t.Fatal("inventory object was not materialized")
	}
	object := org.Objects["CareProgram"]
	if object.Records == nil {
		object.SetRecords(make(map[storage.ID]storage.Record))
	}
	id := storage.ID("a00000000000001")
	object.PutRecord(id, storage.Record{ID: id, Object: "CareProgram"})
	org.Objects["CareProgram"] = object
	query, err := Parse("SELECT Id FROM CareProgram LIMIT 2")
	if err != nil {
		t.Fatal(err)
	}
	result, err := Execute(org, query)
	if err != nil {
		t.Fatalf("record-bearing definition was hidden: %v", err)
	}
	if result.Rows != 1 {
		t.Fatalf("rows %d, want 1", result.Rows)
	}
}

func TestSourceSpreadVirtualVisibilityTracksEdits(t *testing.T) {
	org := storage.NewOrgState()
	org.EnableStandardSpreadSource()
	storage.ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, project.PlainDeveloperDescribeShape})
	if _, ok := storage.EnsureLazyStandardObject(&org, "CareProgram"); !ok {
		t.Fatal("inventory object was not materialized")
	}
	org.RuntimeSchemaStamp = "fixed"
	query, err := Parse("SELECT QualifiedApiName FROM EntityDefinition WHERE QualifiedApiName = 'CareProgram'")
	if err != nil {
		t.Fatal(err)
	}
	cache := NewExecutionCache()
	check := func(want int) {
		t.Helper()
		result, err := ExecuteWithCache(org, query, cache)
		if err != nil {
			t.Fatal(err)
		}
		if result.Rows != want {
			t.Fatalf("virtual rows %d, want %d", result.Rows, want)
		}
	}
	check(0)
	object := org.Objects["CareProgram"]
	original := object.Definition.Label
	object.Definition.Label = "edited"
	org.Objects["CareProgram"] = object
	check(1)
	object.Definition.Label = original
	org.Objects["CareProgram"] = object
	check(0)
	if object.Records == nil {
		object.SetRecords(make(map[storage.ID]storage.Record))
	}
	id := storage.ID("a00000000000001")
	object.PutRecord(id, storage.Record{ID: id, Object: "CareProgram"})
	org.Objects["CareProgram"] = object
	check(1)
	delete(object.Records, id)
	org.Objects["CareProgram"] = object
	check(0)
}

func TestSourceSpreadByValueLookupPreservesNeighborVisibility(t *testing.T) {
	org := storage.NewOrgState()
	org.EnableStandardSpreadSource()
	storage.ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, project.PlainDeveloperDescribeShape})
	if _, ok := storage.ResolveObjectName(org, "CareProgram"); !ok {
		t.Fatal("inventory object was not resolved")
	}
	query, err := Parse("SELECT Id FROM CareProgram LIMIT 2")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(org, query); err == nil {
		t.Fatal("by-value inventory lookup changed origin/main query visibility")
	}
}

func TestSourceSpreadByValueLookupAfterOlderSerialization(t *testing.T) {
	org := storage.NewOrgState()
	org.EnableStandardSpreadSource()
	storage.ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, project.PlainDeveloperDescribeShape})
	data, err := json.Marshal(org)
	if err != nil {
		t.Fatal(err)
	}
	var saved map[string]json.RawMessage
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	delete(saved, "standardSpreadSuppressed")
	data, err = json.Marshal(saved)
	if err != nil {
		t.Fatal(err)
	}
	var restored storage.OrgState
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if _, ok := storage.ResolveObjectName(restored, "CareProgram"); !ok {
		t.Fatal("inventory object was not resolved")
	}
	query, err := Parse("SELECT Id FROM CareProgram LIMIT 2")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(restored, query); err == nil {
		t.Fatal("older serialized origin/main neighbor became visible")
	}
}
