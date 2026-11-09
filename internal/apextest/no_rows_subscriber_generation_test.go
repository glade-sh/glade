package apextest

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestSubscriberMetadataSnapshotRemovalFailsClosed(t *testing.T) {
	const fixture = "testdata/conformance/no_rows_subscriber/project"
	root := t.TempDir()
	if err := filepath.WalkDir(fixture, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		name, err := filepath.Rel(fixture, path)
		if err != nil {
			return err
		}
		writeFile(t, filepath.Join(root, name), string(body))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	index := loadTestIndex(t, root)
	meta := filepath.Join(root, "force-app/main/default/triggers/SyntheticEventTrigger.trigger-meta.xml")
	if err := os.Remove(meta); err != nil {
		t.Fatal(err)
	}
	if _, err := orgFromIndexWithError(index); err == nil {
		t.Fatal("removed captured trigger metadata silently changed subscriber generation")
	}
}
