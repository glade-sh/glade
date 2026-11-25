package soql

import (
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

func attachmentChildViewFixture(t *testing.T, objects ...storage.FixtureObject) storage.OrgState {
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
