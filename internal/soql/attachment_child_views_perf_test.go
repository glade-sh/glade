package soql

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestEmptyAttachmentChildViewsAllocationBudget(t *testing.T) {
	org, query := attachmentChildViewWorkload(t, 100, false)
	cache := NewExecutionCache()
	allocs := testing.AllocsPerRun(5, func() {
		if _, err := ExecuteWithCache(org, query, cache); err != nil {
			t.Fatal(err)
		}
	})
	const maxAllocs = 5000
	if allocs > maxAllocs {
		t.Fatalf("empty attachment child views allocated %.0f times per query, budget %d", allocs, maxAllocs)
	}
}

// A filter takes the ordinary relationship route even when the backing Note
// has the selected Id. This case also runs against origin/main.
func TestFilteredAttachmentChildViewsUseStoredRows(t *testing.T) {
	const parentID = "a01000000000001"
	const noteID = "002000000000001"
	org := attachmentChildViewFixture(t,
		storage.FixtureObject{Name: "ViewParent__c", Records: []storage.FixtureRecord{{ID: parentID}}},
		storage.FixtureObject{Name: "Note", Records: []storage.FixtureRecord{
			{ID: noteID, Fields: map[string]storage.Value{"ParentId": storage.IDValue(parentID)}},
		}},
	)
	const query = "SELECT Id, (SELECT Id FROM CombinedAttachments WHERE Id = '" + noteID + "'), (SELECT Id FROM NotesAndAttachments WHERE Id = '" + noteID + "') FROM ViewParent__c"
	result, err := ParseAndExecute(org, query)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Records) != 1 || result.Records[0].ID != parentID {
		t.Fatalf("parent rows = %v, want %s", result.Records, parentID)
	}
	for _, relationship := range []string{"CombinedAttachments", "NotesAndAttachments"} {
		if rows, ok := result.Records[0].Children[relationship]; !ok || rows == nil || len(rows) != 0 {
			t.Errorf("%s = %#v, want an empty child list", relationship, rows)
		}
	}
}

func TestAttachmentChildViewsReadCurrentRowsWithWarmSchema(t *testing.T) {
	org, query := attachmentChildViewWorkload(t, 1, false)
	cache := NewExecutionCache()
	parentID := storage.ID("a01000000000001")
	noteID := storage.ID("002000000000001")
	importedID := storage.ID("00P000000000001")
	check := func(want storage.ID, context ExecutionContext) {
		t.Helper()
		result, err := ExecuteWithCacheAndContext(org, query, cache, context)
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Records) != 1 {
			t.Fatalf("parent rows = %d, want 1", len(result.Records))
		}
		const relationship = "CombinedAttachments"
		if want == "" {
			if rows := result.Records[0].Children[relationship]; len(rows) != 0 {
				t.Fatalf("%s = %+v, want an empty child list", relationship, rows)
			}
			return
		}
		assertAttachmentChildView(t, result.Records[0], relationship, "CombinedAttachment", want)
	}
	put := func(name string, id storage.ID) {
		object := org.Objects[name]
		object.PutRecord(id, storage.Record{ID: id, Object: name, Fields: map[string]storage.Value{"ParentId": storage.IDValue(parentID)}})
		org.Objects[name] = object
	}
	check("", ExecutionContext{})
	put("Note", noteID)
	check(noteID, ExecutionContext{})
	check("", ExecutionContext{ChildRecordVisible: func(name string, _ storage.Record) bool {
		return name != "CombinedAttachment"
	}})
	put("CombinedAttachment", importedID)
	check(importedID, ExecutionContext{})
}

func TestAttachmentChildViewPlanRechecksLoadedRelationshipsAfterLazyInstall(t *testing.T) {
	org := attachmentChildViewFixture(t, storage.FixtureObject{Name: "ViewParent__c"})
	org.EnableStandardSpreadSource()
	parent := org.Objects["ViewParent__c"].Definition
	cache := NewExecutionCache()
	const relationship = "CombinedAttachments"
	const view = "CombinedAttachment"
	if got := planAttachmentView(org, parent, relationship, view, newChildRelationshipQueryCache(cache)); got.loaded {
		t.Fatal("expected the platform fallback before the loaded edge")
	}

	// Simulate a loaded schema edge becoming visible between validation and
	// projection while the execution cache still holds the earlier result.
	org.Objects["ViewChild__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName: "ViewChild__c",
		Relations: []storage.Relationship{
			{Field: "Parent__c", ParentObjects: []string{parent.APIName}, ChildRelationship: relationship},
		},
	}}
	if got := planAttachmentView(org, parent, relationship, view, newChildRelationshipQueryCache(cache)); !got.loaded {
		t.Fatal("expected to use the newly visible loaded edge")
	}
}

func TestAttachmentChildViewsFindCandidatesWithAndWithoutStorageIndex(t *testing.T) {
	const emptyParent = "a01000000000001"
	const noteParent = "a01000000000002"
	const noteID = "002000000000001"
	org := attachmentChildViewFixture(t,
		storage.FixtureObject{Name: "ViewParent__c", Records: []storage.FixtureRecord{{ID: emptyParent}, {ID: noteParent}}},
		storage.FixtureObject{Name: "Note", Records: []storage.FixtureRecord{
			{ID: noteID, Fields: map[string]storage.Value{"ParentId": storage.IDValue(noteParent + "AAA")}},
		}},
	)
	check := func(t *testing.T) {
		result := queryAttachmentChildViews(t, org, ExecutionContext{})
		if len(result.Records) != 2 {
			t.Fatalf("parent rows = %d, want 2", len(result.Records))
		}
		for _, parent := range result.Records {
			if parent.ID == noteParent {
				assertAttachmentChildView(t, parent, "CombinedAttachments", "CombinedAttachment", noteID)
				assertAttachmentChildView(t, parent, "NotesAndAttachments", "NoteAndAttachment", noteID)
			} else if len(parent.Children["CombinedAttachments"]) != 0 || len(parent.Children["NotesAndAttachments"]) != 0 {
				t.Errorf("unexpected attachment views for %s", parent.ID)
			}
		}
	}
	t.Run("parent index", check)
	note := org.Objects["Note"]
	note.Definition.Indexes = []storage.IndexDefinition{{Name: "Note.Parent", Object: "Note", Fields: []string{"ParentId"}}}
	org.Objects["Note"] = note
	storage.RebuildObjectIndexes(&org, "Note")
	t.Run("storage index", check)
}

func BenchmarkAttachmentChildViewWorkloads(b *testing.B) {
	for _, workload := range []struct {
		name        string
		parentCount int
		populated   bool
	}{
		{"Empty1000", 1000, false},
		{"Sparse100", 100, true},
	} {
		b.Run(workload.name, func(b *testing.B) {
			org, query := attachmentChildViewWorkload(b, workload.parentCount, workload.populated)
			cache := NewExecutionCache()
			result, err := ExecuteWithCache(org, query, cache)
			if err != nil {
				b.Fatal(err)
			}
			if len(result.Records) != workload.parentCount {
				b.Fatalf("parent rows = %d, want %d", len(result.Records), workload.parentCount)
			}
			for _, child := range query.ChildQueries {
				rows := 0
				for _, parent := range result.Records {
					children, ok := parent.Children[child.Relationship]
					if !ok || children == nil || len(children) > 1 {
						b.Fatalf("unexpected %s rows for %s: %#v", child.Relationship, parent.ID, children)
					}
					rows += len(children)
				}
				want := 0
				if workload.populated {
					want = workload.parentCount / 10
				}
				if rows != want {
					b.Fatalf("%s rows = %d, want %d", child.Relationship, rows, want)
				}
			}
			b.ReportAllocs()
			for b.Loop() {
				if _, err := ExecuteWithCache(org, query, cache); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func attachmentChildViewWorkload(t testing.TB, parentCount int, populated bool) (storage.OrgState, Query) {
	t.Helper()
	parents := make([]storage.FixtureRecord, parentCount)
	for i := range parents {
		parents[i].ID = storage.ID(fmt.Sprintf("a01%012d", i+1))
	}
	objects := []storage.FixtureObject{{Name: "ViewParent__c", Records: parents}}
	if populated {
		var notes, combined, notesAndAttachments []storage.FixtureRecord
		for i := 0; i < parentCount; i += 10 {
			fields := map[string]storage.Value{"ParentId": storage.IDValue(parents[i].ID)}
			notes = append(notes, storage.FixtureRecord{ID: storage.ID(fmt.Sprintf("002%012d", i+1)), Fields: fields})
			combined = append(combined, storage.FixtureRecord{ID: storage.ID(fmt.Sprintf("00P%012d", i+1)), Fields: fields})
			notesAndAttachments = append(notesAndAttachments, storage.FixtureRecord{ID: storage.ID(fmt.Sprintf("00P%012d", i+2)), Fields: fields})
		}
		objects = append(objects,
			storage.FixtureObject{Name: "Note", Records: notes},
			storage.FixtureObject{Name: "CombinedAttachment", Records: combined},
			storage.FixtureObject{Name: "NoteAndAttachment", Records: notesAndAttachments},
		)
	}
	org := attachmentChildViewFixture(t, objects...)
	storage.EnsureStandardObjectSpread(&org)
	query, err := Parse("SELECT Id, (SELECT Id FROM CombinedAttachments), (SELECT Id FROM NotesAndAttachments) FROM ViewParent__c")
	if err != nil {
		t.Fatal(err)
	}
	return org, query
}
