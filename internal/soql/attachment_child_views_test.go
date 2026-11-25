package soql

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestAttachmentChildViewsPreserveImportedRows(t *testing.T) {
	const importedParent = "a0100000000000A"
	const derivedParent = "a0100000000000a"
	const importedCombined = "00P000000000001"
	const importedNoteAndAttachment = "00P000000000002"
	const note = "002000000000001"
	org := attachmentChildViewFixture(t,
		storage.FixtureObject{Name: "ViewParent__c", Records: []storage.FixtureRecord{
			{ID: importedParent}, {ID: derivedParent},
		}},
		storage.FixtureObject{Name: "CombinedAttachment", Records: []storage.FixtureRecord{
			{ID: importedCombined, Fields: map[string]storage.Value{"ParentId": storage.IDValue(importedParent)}},
		}},
		storage.FixtureObject{Name: "NoteAndAttachment", Records: []storage.FixtureRecord{
			{ID: importedNoteAndAttachment, Fields: map[string]storage.Value{"ParentId": storage.IDValue(importedParent)}},
		}},
		storage.FixtureObject{Name: "Note", Records: []storage.FixtureRecord{
			{ID: note, Fields: map[string]storage.Value{"ParentId": storage.IDValue(derivedParent)}},
		}},
	)
	result := queryAttachmentChildViews(t, org, ExecutionContext{})
	if len(result.Records) != 2 {
		t.Fatalf("parent rows = %d, want 2", len(result.Records))
	}
	for _, parent := range result.Records {
		wantCombined, wantNoteAndAttachment := storage.ID(note), storage.ID(note)
		if parent.ID == importedParent {
			wantCombined, wantNoteAndAttachment = importedCombined, importedNoteAndAttachment
		}
		assertAttachmentChildView(t, parent, "CombinedAttachments", "CombinedAttachment", wantCombined)
		assertAttachmentChildView(t, parent, "NotesAndAttachments", "NoteAndAttachment", wantNoteAndAttachment)
	}
}

func TestAttachmentChildViewsCheckReturnedTypeVisibility(t *testing.T) {
	const parentID = "a01000000000001"
	const noteID = "002000000000001"
	org := attachmentChildViewFixture(t,
		storage.FixtureObject{Name: "ViewParent__c", Records: []storage.FixtureRecord{{ID: parentID}}},
		storage.FixtureObject{Name: "Note", Records: []storage.FixtureRecord{
			{ID: noteID, Fields: map[string]storage.Value{"ParentId": storage.IDValue(parentID)}},
		}},
	)
	for _, tc := range []struct {
		view, relationship, otherView, otherRelationship string
	}{
		{"CombinedAttachment", "CombinedAttachments", "NoteAndAttachment", "NotesAndAttachments"},
		{"NoteAndAttachment", "NotesAndAttachments", "CombinedAttachment", "CombinedAttachments"},
	} {
		t.Run(tc.view, func(t *testing.T) {
			context := ExecutionContext{ChildRecordVisible: func(objectName string, row storage.Record) bool {
				if objectName == tc.view || objectName == tc.otherView {
					if row.Object != objectName || row.ID != noteID || row.Fields["ParentId"].ID != parentID {
						t.Errorf("visibility callback received wrong %s row: %+v", objectName, row)
					}
				}
				return objectName != tc.view
			}}
			result := queryAttachmentChildViews(t, org, context)
			if len(result.Records) != 1 {
				t.Fatalf("parent rows = %d, want 1", len(result.Records))
			}
			parent := result.Records[0]
			if rows := parent.Children[tc.relationship]; len(rows) != 0 {
				t.Errorf("%s returned vetoed rows: %+v", tc.relationship, rows)
			}
			assertAttachmentChildView(t, parent, tc.otherRelationship, tc.otherView, noteID)
		})
	}
}

func BenchmarkAttachmentChildViews(b *testing.B) {
	const parentCount = 100
	parents := make([]storage.FixtureRecord, parentCount)
	for i := range parents {
		parents[i].ID = storage.ID(fmt.Sprintf("a01%012d", i+1))
	}
	org := attachmentChildViewFixture(b, storage.FixtureObject{Name: "ViewParent__c", Records: parents})
	storage.EnsureStandardObjectSpread(&org)
	query, err := Parse("SELECT Id, (SELECT Id FROM CombinedAttachments), (SELECT Id FROM NotesAndAttachments) FROM ViewParent__c")
	if err != nil {
		b.Fatal(err)
	}
	cache := NewExecutionCache()
	result, err := ExecuteWithCache(org, query, cache)
	if err != nil {
		b.Fatal(err)
	}
	if len(result.Records) != parentCount {
		b.Fatalf("parent rows = %d, want %d", len(result.Records), parentCount)
	}
	for _, parent := range result.Records {
		for _, child := range query.ChildQueries {
			if rows, ok := parent.Children[child.Relationship]; !ok || len(rows) != 0 {
				b.Fatalf("parent %s %s = %+v, want an empty child list", parent.ID, child.Relationship, rows)
			}
		}
	}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := ExecuteWithCache(org, query, cache); err != nil {
			b.Fatal(err)
		}
	}
}

func attachmentChildViewFixture(t testing.TB, objects ...storage.FixtureObject) storage.OrgState {
	t.Helper()
	org := storage.NewOrgState()
	fixture := storage.NewFixture()
	fixture.Org.APIVersion = "67.0"
	fixture.Objects = objects
	if err := storage.ApplyFixture(&org, fixture); err != nil {
		t.Fatal(err)
	}
	return org
}

func queryAttachmentChildViews(t *testing.T, org storage.OrgState, context ExecutionContext) Result {
	t.Helper()
	query, err := Parse("SELECT Id, (SELECT Id FROM CombinedAttachments), (SELECT Id FROM NotesAndAttachments) FROM ViewParent__c")
	if err != nil {
		t.Fatal(err)
	}
	result, err := ExecuteWithCacheAndContext(org, query, NewExecutionCache(), context)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func assertAttachmentChildView(t *testing.T, parent storage.Record, relationship, view string, id storage.ID) {
	t.Helper()
	rows := parent.Children[relationship]
	if len(rows) != 1 || rows[0].ID != id || rows[0].Object != view || rows[0].Fields["Id"].ID != id {
		t.Errorf("parent %s %s = %+v, want %s %s", parent.ID, relationship, rows, view, id)
		return
	}
	if _, projected := rows[0].Fields["ParentId"]; projected {
		t.Errorf("parent %s %s projected unrequested ParentId", parent.ID, relationship)
	}
}
