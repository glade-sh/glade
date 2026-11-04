package startupcache

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSelectedPackageAncestorConfigManifestReview(t *testing.T) {
	root := t.TempDir()
	write := func(path, data string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(root, "sfdx-project.json"), `{"namespace":"Root","packageDirectories":[{"path":"sub/force-app","default":true}]}`)
	config := filepath.Join(root, "sub", "sfdx-project.json")
	write(config, `{"namespace":"First","packageDirectories":[{"path":"force-app","default":true}]}`)
	write(filepath.Join(root, "sub", "force-app", "classes", "Example.cls"), `public class Example {}`)
	before, err := ValidateInputWithSourceDigests(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	write(config, `{"namespace":"","packageDirectories":[{"path":"force-app","default":true}]}`)
	after, err := ValidateInputWithSourceDigests(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if before.Digest() == after.Digest() {
		t.Fatal("ancestor namespace configuration was omitted from the cache identity")
	}
}
