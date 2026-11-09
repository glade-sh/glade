package startupcache

import (
	"os"
	"path/filepath"
	"testing"
)

// A .forceignore decides which package source loads, so editing a governing
// file, even below the project root, must invalidate the manifest.
func TestForceIgnoreEditChangesManifest(t *testing.T) {
	root := t.TempDir()
	write := func(path, source string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"62.0"}`)
	write(filepath.Join(root, "force-app", "classes", "Marker.cls"), `public class Marker {}`)
	ignore := filepath.Join(root, "force-app", ".forceignore")
	write(ignore, "Other.cls\n")
	before, err := ValidateInputWithSourceDigests(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	write(ignore, "Other.cls\nAnother.cls\n")
	after, err := ValidateInputWithSourceDigests(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if before.Digest() == after.Digest() {
		t.Fatal("package .forceignore edit did not invalidate manifest")
	}
}
