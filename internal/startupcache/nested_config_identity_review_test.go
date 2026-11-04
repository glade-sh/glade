package startupcache

import (
	"os"
	"path/filepath"
	"testing"
)

// Config identity is a local cache contract.
func TestNestedProjectConfigContentChangesManifestReview(t *testing.T) {
	root := t.TempDir()
	write := func(path, source string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(source), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"packs","default":true}]}`)
	child := filepath.Join(root, "packs", "child")
	config := filepath.Join(child, "sfdx-project.json")
	write(config, `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"","sourceApiVersion":"62.0"}`)
	write(filepath.Join(child, "force-app", "main", "classes", "Marker.cls"), `public class Marker {}`)
	before, err := ValidateInputWithSourceDigests(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	write(config, `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"","sourceApiVersion":"67.0"}`)
	after, err := ValidateInputWithSourceDigests(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if before.Digest() == after.Digest() {
		t.Fatal("nested project config edit did not invalidate manifest")
	}
}
