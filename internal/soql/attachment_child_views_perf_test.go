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
