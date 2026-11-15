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

func TestSourceSpreadSemiJoinPreservesNeighborDiagnostic(t *testing.T) {
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
			query, err := Parse("SELECT Id FROM Account WHERE Id IN (SELECT Id FROM CareProgram)")
			if err != nil {
				t.Fatal(err)
			}
			_, err = Execute(org, query)
			if err == nil || err.Error() != "soql: unknown object CareProgram" {
				t.Fatalf("origin/main diagnostic = %v", err)
			}
		})
	}
}

func TestSourceSpreadChildRelationshipPreservesNeighbor(t *testing.T) {
	org := storage.NewOrgState()
	org.EnableStandardSpreadSource()
	storage.ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, project.PlainDeveloperDescribeShape})
	if _, ok := storage.EnsureLazyStandardObject(&org, "CareProgram"); !ok {
		t.Fatal("inventory object was not materialized")
	}
	if _, ok := storage.EnsureLazyStandardObject(&org, "User"); !ok {
		if _, exists := org.Objects["User"]; !exists {
			t.Fatal("User was not materialized")
		}
	}
	checked := false
	for _, relation := range org.Objects["CareProgram"].Definition.Relations {
		for _, parent := range relation.ParentObjects {
			if parent != "User" {
				continue
			}
			checked = true
			name, _, ok := loadedChildRelationship(org, org.Objects["User"].Definition, derivedChildRelationshipName(org.Objects["CareProgram"].Definition))
			if ok && name == "CareProgram" {
				t.Fatal("origin/main child relationship omitted the neighbor")
			}
		}
	}
	if !checked {
		t.Fatal("no User child relationship was available to test")
	}
}

func TestSourceSpreadChildRelationshipCacheTracksVisibility(t *testing.T) {
	org := storage.NewOrgState()
	org.EnableStandardSpreadSource()
	storage.ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, project.PlainDeveloperDescribeShape})
	if _, ok := storage.EnsureLazyStandardObject(&org, "CareProgram"); !ok {
		t.Fatal("inventory object was not materialized")
	}
	storage.EnsureLazyStandardObject(&org, "User")
	object := org.Objects["CareProgram"]
	parent := org.Objects["User"].Definition
	relationship := derivedChildRelationshipName(object.Definition)
	cache := &childRelationshipQueryCache{execution: NewExecutionCache()}
	check := func(want bool) {
		t.Helper()
		name, _, ok := childRelationshipCached(org, parent, relationship, cache)
		if got := ok && name == "CareProgram"; got != want {
			t.Fatalf("child visibility %t, want %t", got, want)
		}
	}
	check(false)
	original := object.Definition.Label
	object.Definition.Label = "edited"
	org.Objects["CareProgram"] = object
	check(true)
	object.Definition.Label = original
	org.Objects["CareProgram"] = object
	check(false)
	if object.Records == nil {
		object.SetRecords(make(map[storage.ID]storage.Record))
	}
	id := storage.ID("a00000000000001")
	object.PutRecord(id, storage.Record{ID: id, Object: "CareProgram"})
	org.Objects["CareProgram"] = object
	check(true)
	delete(object.Records, id)
	org.Objects["CareProgram"] = object
	check(false)
	org.StandardSpreadCapturedPlain = false
	check(true)
	org.StandardSpreadCapturedPlain = true
	check(false)
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

func TestSourceSpreadEditedNeighborRemainsVisibleAfterSerialization(t *testing.T) {
	org := storage.NewOrgState()
	org.EnableStandardSpreadSource()
	storage.ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, project.PlainDeveloperDescribeShape})
	if _, ok := storage.EnsureLazyStandardObject(&org, "CareProgram"); !ok {
		t.Fatal("inventory object was not materialized")
	}
	object := org.Objects["CareProgram"]
	object.Definition.Label = "edited"
	org.Objects["CareProgram"] = object
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
	if _, err := Execute(restored, query); err != nil {
		t.Fatalf("edited serialized definition was hidden: %v", err)
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

func TestSourceSpreadVirtualVisibilityTracksCapturedShape(t *testing.T) {
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
	org.StandardSpreadCapturedPlain = false
	check(1)
	org.StandardSpreadCapturedPlain = true
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
