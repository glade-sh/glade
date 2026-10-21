package resource

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestFolderMetadataKeepsFrozenDefinitionPrivate(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Folder"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Folder", Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}}}}
	clone := org.CloneRuntimeFrozenDefinition()
	folder := filepath.Join(t.TempDir(), "Owned-meta.xml")
	if err := os.WriteFile(folder, []byte(`<DocumentFolder><name>Owned</name></DocumentFolder>`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := ensureFolderObject(&clone, []string{folder}, ""); err != nil {
		t.Fatal(err)
	}
	if _, ok := org.Objects["Folder"].Definition.Fields["PublicFolderAccess"]; ok {
		t.Fatal("folder metadata mutated the frozen definition")
	}
	if _, ok := clone.Objects["Folder"].Definition.Fields["PublicFolderAccess"]; !ok {
		t.Fatal("folder metadata was not installed")
	}
}
