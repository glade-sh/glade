package soql

import (
	"fmt"
	"slices"
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

func TestAttachmentChildViewsReadCurrentRelationshipsWithWarmCache(t *testing.T) {
	const parentID = "a01000000000001"
	const childID = "a02000000000001"
	const noteID = "002000000000001"
	views := []struct{ relationship, object string }{
		{"CombinedAttachments", "CombinedAttachment"},
		{"NotesAndAttachments", "NoteAndAttachment"},
	}
	for _, invalidation := range []struct {
		name  string
		apply func(*storage.OrgState)
	}{
		{"schema edit", (*storage.OrgState).ClearRuntimeSchemaStamp},
		{"external org update", (*storage.OrgState).InvalidateRuntimeGeneration},
		{"direct declaration edit", func(*storage.OrgState) {}},
	} {
		t.Run(invalidation.name, func(t *testing.T) {
			org := attachmentChildViewFixture(t,
				storage.FixtureObject{Name: "ViewParent__c", Records: []storage.FixtureRecord{{ID: parentID}}},
				storage.FixtureObject{Name: "ViewChild__c", Records: []storage.FixtureRecord{
					{ID: childID, Fields: map[string]storage.Value{"Parent__c": storage.IDValue(parentID)}},
				}},
				storage.FixtureObject{Name: "Note", Records: []storage.FixtureRecord{
					{ID: noteID, Fields: map[string]storage.Value{"ParentId": storage.IDValue(parentID)}},
				}},
			)
			query, err := Parse("SELECT Id, (SELECT Id FROM CombinedAttachments), (SELECT Id FROM NotesAndAttachments) FROM ViewParent__c")
			if err != nil {
				t.Fatal(err)
			}
			cache := NewExecutionCache()
			child := org.Objects["ViewChild__c"]
			declared := child
			for _, view := range views {
				declared.Definition.Relations = append(declared.Definition.Relations, storage.Relationship{
					Field: "Parent__c", ParentObjects: []string{"ViewParent__c"}, ChildRelationship: view.relationship,
				})
			}
			org.Objects["ViewChild__c"] = declared
			check := func(wantDeclared bool) {
				t.Helper()
				result, err := ExecuteWithCache(org, query, cache)
				if err != nil {
					t.Fatal(err)
				}
				if len(result.Records) != 1 || result.Records[0].ID != parentID {
					t.Fatalf("parent rows = %+v, want %s", result.Records, parentID)
				}
				for _, view := range views {
					object, id := view.object, storage.ID(noteID)
					if wantDeclared {
						object, id = "ViewChild__c", childID
					}
					assertAttachmentChildView(t, result.Records[0], view.relationship, object, id)
				}
			}
			check(true)
			org.Objects["ViewChild__c"] = child
			invalidation.apply(&org)
			check(false)
			org.Objects["ViewChild__c"] = declared
			invalidation.apply(&org)
			check(true)

			retargeted := declared
			retargeted.Definition.Relations = slices.Clone(declared.Definition.Relations)
			for i := range retargeted.Definition.Relations {
				retargeted.Definition.Relations[i].ParentObjects = []string{"OtherParent__c"}
			}
			org.Objects["ViewChild__c"] = retargeted
			invalidation.apply(&org)
			check(false)
		})
	}
}

func TestAttachmentChildViewsReadCurrentPluralLabelsWithWarmCache(t *testing.T) {
	const parentID = "a01000000000001"
	const childID = "a02000000000001"
	const noteID = "002000000000001"
	for _, view := range []struct{ relationship, object string }{
		{"CombinedAttachments", "CombinedAttachment"},
		{"NotesAndAttachments", "NoteAndAttachment"},
	} {
		t.Run(view.relationship, func(t *testing.T) {
			org := attachmentChildViewFixture(t,
				storage.FixtureObject{Name: "ViewParent__c", Records: []storage.FixtureRecord{{ID: parentID}}},
				storage.FixtureObject{Name: "ViewChild__c", Records: []storage.FixtureRecord{
					{ID: childID, Fields: map[string]storage.Value{"Parent__c": storage.IDValue(parentID)}},
				}},
				storage.FixtureObject{Name: "Note", Records: []storage.FixtureRecord{
					{ID: noteID, Fields: map[string]storage.Value{"ParentId": storage.IDValue(parentID)}},
				}},
			)
			child := org.Objects["ViewChild__c"]
			child.Definition.Relations = []storage.Relationship{
				{Field: "Parent__c", ParentObjects: []string{"ViewParent__c"}},
			}
			child.Definition.PluralLabel = view.relationship
			org.Objects["ViewChild__c"] = child
			query, err := Parse("SELECT Id, (SELECT Id FROM " + view.relationship + ") FROM ViewParent__c")
			if err != nil {
				t.Fatal(err)
			}
			check := func(cache *ExecutionCache, object string, id storage.ID) {
				t.Helper()
				result, err := ExecuteWithCache(org, query, cache)
				if err != nil {
					t.Fatal(err)
				}
				if len(result.Records) != 1 || result.Records[0].ID != parentID {
					t.Fatalf("parent rows = %+v, want %s", result.Records, parentID)
				}
				assertAttachmentChildView(t, result.Records[0], view.relationship, object, id)
			}
			cache := NewExecutionCache()
			check(cache, "ViewChild__c", childID)
			child.Definition.PluralLabel = "Unrelated Children"
			org.Objects["ViewChild__c"] = child
			check(NewExecutionCache(), view.object, noteID)
			check(cache, view.object, noteID)
		})
	}
}

func TestAttachmentChildRelationshipCacheTracksResolverInputs(t *testing.T) {
	const parentName = "ViewParent__c"
	const childName = "ViewChild__c"
	const view = "CombinedAttachments"
	for _, test := range []struct {
		name        string
		prepare     func(*storage.OrgState, *storage.ObjectDefinition)
		change      func(*storage.OrgState, *storage.ObjectDefinition)
		queryParent string
		queryEdge   string
		wantBefore  string
		wantAfter   string
	}{
		{
			name: "loaded inventory at the same count", wantBefore: childName, wantAfter: "OtherChild__c",
			change: func(org *storage.OrgState, _ *storage.ObjectDefinition) {
				org.Objects["OtherChild__c"] = org.Objects[childName]
				delete(org.Objects, childName)
			},
		},
		{
			name: "definition API name", wantBefore: childName, wantAfter: "CombinedAttachment",
			prepare: func(_ *storage.OrgState, def *storage.ObjectDefinition) {
				def.APIName, def.Relations[0].ChildRelationship = view, ""
			},
			change: func(_ *storage.OrgState, def *storage.ObjectDefinition) { def.APIName = childName },
		},
		{
			name: "label", wantBefore: childName, wantAfter: "CombinedAttachment",
			prepare: func(_ *storage.OrgState, def *storage.ObjectDefinition) {
				def.Label, def.Relations[0].ChildRelationship = view, ""
			},
			change: func(_ *storage.OrgState, def *storage.ObjectDefinition) { def.Label = "Other Children" },
		},
		{
			name: "plural label", wantBefore: childName, wantAfter: "CombinedAttachment",
			prepare: func(_ *storage.OrgState, def *storage.ObjectDefinition) {
				def.PluralLabel, def.Relations[0].ChildRelationship = view, ""
			},
			change: func(_ *storage.OrgState, def *storage.ObjectDefinition) { def.PluralLabel = "Other Children" },
		},
		{
			name: "declared relationship target", wantBefore: childName, wantAfter: "CombinedAttachment",
			change: func(_ *storage.OrgState, def *storage.ObjectDefinition) {
				def.Relations[0].ParentObjects[0] = "OtherParent__c"
			},
		},
		{
			name: "reference field target at the same count", wantBefore: childName, wantAfter: "CombinedAttachment",
			prepare: func(_ *storage.OrgState, def *storage.ObjectDefinition) {
				def.PluralLabel, def.Relations = view, nil
				def.Fields["OwnerId"] = storage.Field{APIName: "OwnerId", Type: storage.FieldReference, ReferenceTo: []string{parentName}}
			},
			change: func(_ *storage.OrgState, def *storage.ObjectDefinition) {
				def.Fields["OwnerId"].ReferenceTo[0] = "OtherParent__c"
			},
		},
		{
			name: "namespace priority", wantBefore: "alpha__ViewChild__c", wantAfter: "beta__ViewChild__c",
			prepare: func(org *storage.OrgState, def *storage.ObjectDefinition) {
				org.Namespace = "alpha"
				org.Objects["alpha__ViewChild__c"] = storage.ObjectState{Definition: *def}
				org.Objects["beta__ViewChild__c"] = storage.ObjectState{Definition: *def}
			},
			change: func(org *storage.OrgState, _ *storage.ObjectDefinition) { org.Namespace = "beta" },
		},
		{
			name: "API version", queryParent: "Account", queryEdge: "ContactPointOttSvcs",
			wantBefore: "ContactPointOttSvc", wantAfter: "",
			prepare: func(org *storage.OrgState, _ *storage.ObjectDefinition) { org.APIVersion = "68.0" },
			change:  func(org *storage.OrgState, _ *storage.ObjectDefinition) { org.APIVersion = "67.0" },
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			org := storage.NewOrgState()
			definition := storage.ObjectDefinition{
				APIName: childName, Fields: map[string]storage.Field{},
				Relations: []storage.Relationship{{Field: "Parent__c", ChildRelationship: view, ParentObjects: []string{parentName}}},
			}
			if test.prepare != nil {
				test.prepare(&org, &definition)
			}
			org.Objects[childName] = storage.ObjectState{Definition: definition}
			parent, edge := storage.ObjectDefinition{APIName: parentName}, view
			if test.queryParent != "" {
				parent.APIName, edge = test.queryParent, test.queryEdge
			}
			check := func(cache *ExecutionCache, want string) {
				t.Helper()
				got := childRelationshipResolutionCached(org, parent, edge, newChildRelationshipQueryCache(cache), true)
				if got.childObjectName != want || got.ok != (want != "") {
					t.Errorf("resolved (%q, %t), want (%q, %t)", got.childObjectName, got.ok, want, want != "")
				}
			}
			cache := NewExecutionCache()
			check(cache, test.wantBefore)
			test.change(&org, &definition)
			if current, exists := org.Objects[childName]; exists {
				current.Definition = definition
				org.Objects[childName] = current
			}
			check(NewExecutionCache(), test.wantAfter)
			check(cache, test.wantAfter)
		})
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

func TestCombinedAttachmentsUsesPlatformFileLinks(t *testing.T) {
	const parentID = "a01000000000001"
	const customChildID = "a02000000000001"
	org := attachmentChildViewFixture(t,
		storage.FixtureObject{Name: "ViewParent__c", Records: []storage.FixtureRecord{{ID: parentID}}},
		storage.FixtureObject{Name: "CustomLink__c", Records: []storage.FixtureRecord{
			{ID: customChildID, Fields: map[string]storage.Value{"Parent__c": storage.IDValue(parentID)}},
		}},
	)
	customLink := org.Objects["CustomLink__c"]
	customLink.Definition.Fields["Parent__c"] = storage.Field{
		APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"ViewParent__c"},
	}
	customLink.Definition.Relations = []storage.Relationship{
		{Field: "Parent__c", ParentObjects: []string{"ViewParent__c"}, ChildRelationship: "ContentDocumentLinks__r"},
	}
	org.Objects["CustomLink__c"] = customLink

	result, err := ParseAndExecute(org, "SELECT Id, (SELECT Id FROM CombinedAttachments), (SELECT Id FROM ContentDocumentLinks__r) FROM ViewParent__c")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Records) != 1 {
		t.Fatalf("parent rows = %d, want 1", len(result.Records))
	}
	parent := result.Records[0]
	if rows := parent.Children["CombinedAttachments"]; len(rows) != 0 {
		t.Errorf("CombinedAttachments = %+v, want an empty child list", rows)
	}
	assertAttachmentChildView(t, parent, "ContentDocumentLinks__r", "CustomLink__c", customChildID)
}

func TestCombinedAttachmentsUsesImportedDocumentIDs(t *testing.T) {
	const parentID = "a01000000000001"
	const documentID = "069000000000001"
	for _, tc := range []struct {
		name string
		id   storage.Value
	}{
		{"typed", storage.IDValue(documentID)},
		{"string", storage.StringValue(documentID)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := attachmentChildViewFixture(t,
				storage.FixtureObject{Name: "ViewParent__c", Records: []storage.FixtureRecord{{ID: parentID}}},
				storage.FixtureObject{Name: "ContentDocumentLink", Records: []storage.FixtureRecord{
					{ID: "06A000000000001", Fields: map[string]storage.Value{
						"LinkedEntityId": storage.IDValue(parentID), "ContentDocumentId": tc.id,
					}},
				}},
			)
			result := queryAttachmentChildViews(t, org, ExecutionContext{})
			if len(result.Records) != 1 {
				t.Fatalf("parent rows = %d, want 1", len(result.Records))
			}
			assertAttachmentChildView(t, result.Records[0], "CombinedAttachments", "CombinedAttachment", documentID)
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
