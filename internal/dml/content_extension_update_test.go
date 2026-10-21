package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestContentExtensionHydratedUpdates(t *testing.T) {
	for _, specimen := range []struct {
		name      string
		path      string
		extension string
	}{
		{name: "extensionless", path: "specimen"},
		{name: "extensionful", path: "specimen.txt", extension: "txt"},
	} {
		for _, objectName := range []string{"ContentVersion", "ContentDocument"} {
			t.Run(specimen.name+"/"+objectName, func(t *testing.T) {
				org := contentExtensionTestOrg()
				engine := NewEngine(&org)
				versionID := insertContentExtensionVersion(t, &engine, specimen.path, "")
				id := versionID
				if objectName == "ContentDocument" {
					id = org.Objects["ContentVersion"].Records[versionID].Fields["ContentDocumentId"].ID
				}
				// Before-update hydration carries the stored derived fields even
				// when the caller's update supplies only an editable field.
				input := org.Objects[objectName].Records[id].Clone()
				input.Fields["Title"] = storage.StringValue("Revised specimen")
				updated := engine.Update([]storage.Record{input})
				if len(updated) != 1 || !updated[0].Success {
					t.Fatalf("hydrated update = %#v", updated)
				}
				stored := org.Objects[objectName].Records[id]
				if stored.Fields["Title"].String != "Revised specimen" {
					t.Fatalf("stored title = %#v", stored.Fields["Title"])
				}
				assertContentExtension(t, stored, specimen.extension)
			})
		}
	}
}

// These engine-level controls preserve rejection of changed readonly input.
// They do not assert the separate typed-JSON assignment surface.
func TestContentExtensionExplicitChangedValueRejected(t *testing.T) {
	for _, objectName := range []string{"ContentVersion", "ContentDocument"} {
		t.Run(objectName, func(t *testing.T) {
			org := contentExtensionTestOrg()
			engine := NewEngine(&org)
			versionID := insertContentExtensionVersion(t, &engine, "specimen.txt", "")
			id := versionID
			if objectName == "ContentDocument" {
				id = org.Objects["ContentVersion"].Records[versionID].Fields["ContentDocumentId"].ID
			}
			updated := engine.Update([]storage.Record{{
				Object: objectName,
				ID:     id,
				Fields: map[string]storage.Value{"FileExtension": storage.StringValue("changed")},
			}})
			if len(updated) != 1 || updated[0].Success || updated[0].StatusCode != "INVALID_FIELD_FOR_INSERT_UPDATE" {
				t.Fatalf("explicit readonly update = %#v", updated)
			}
			assertContentExtension(t, org.Objects[objectName].Records[id], "txt")
		})
	}
}

func TestContentExtensionlessReplacementClearsDocumentExtension(t *testing.T) {
	org := contentExtensionTestOrg()
	engine := NewEngine(&org)
	firstID := insertContentExtensionVersion(t, &engine, "specimen.txt", "")
	documentID := org.Objects["ContentVersion"].Records[firstID].Fields["ContentDocumentId"].ID
	secondID := insertContentExtensionVersion(t, &engine, "replacement", documentID)
	assertContentExtension(t, org.Objects["ContentVersion"].Records[firstID], "txt")
	assertContentExtension(t, org.Objects["ContentVersion"].Records[secondID], "")
	assertContentExtension(t, org.Objects["ContentDocument"].Records[documentID], "")
}

func insertContentExtensionVersion(t *testing.T, engine *Engine, path string, documentID storage.ID) storage.ID {
	t.Helper()
	fields := map[string]storage.Value{
		"Title":        storage.StringValue("Specimen"),
		"PathOnClient": storage.StringValue(path),
		"VersionData":  storage.BlobValue("owned bytes"),
	}
	if documentID != "" {
		fields["ContentDocumentId"] = storage.IDValue(documentID)
	}
	inserted := engine.Insert([]storage.Record{{Object: "ContentVersion", Fields: fields}})
	if len(inserted) != 1 || !inserted[0].Success {
		t.Fatalf("content version insert = %#v", inserted)
	}
	return inserted[0].ID
}

func assertContentExtension(t *testing.T, record storage.Record, expected string) {
	t.Helper()
	value, present := record.GetField("FileExtension")
	if expected == "" {
		if present && value.Kind != storage.ValueNull {
			t.Fatalf("%s extension = %#v, want null", record.Object, value)
		}
		return
	}
	if !present || value.Kind != storage.ValueString || value.String != expected {
		t.Fatalf("%s extension = %#v, want %q", record.Object, value, expected)
	}
}

func contentExtensionTestOrg() storage.OrgState {
	org := storage.NewOrgState()
	for _, name := range []string{"ContentVersion", "ContentDocument", "ContentDocumentLink"} {
		storage.EnsureStandardObject(&org, name)
	}
	storage.EnsureDeterministicPlatformData(&org)
	return org
}
