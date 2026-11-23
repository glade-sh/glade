package vm_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

// capturedRequestIndex copies a captured fixture project into a temp root and
// indexes it for the requested API version.
func capturedRequestIndex(t *testing.T, fixture, api string) typesys.Index {
	t.Helper()
	root := t.TempDir()
	source := filepath.Join(root, "sources")
	if err := os.MkdirAll(source, 0755); err != nil {
		t.Fatal(err)
	}
	paths, err := filepath.Glob(filepath.Join(fixture, "force-app", "main", "default", "classes", "*.cls"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("missing sources: %v", err)
	}
	for _, path := range paths {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(source, filepath.Base(path)), b, 0644); err != nil {
			t.Fatal(err)
		}
	}
	metadataRoot := filepath.Join(fixture, "force-app", "main", "default")
	err = filepath.Walk(metadataRoot, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			if path == filepath.Join(metadataRoot, "classes") {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(metadataRoot, path)
		if err != nil {
			return err
		}
		target := filepath.Join(root, "metadata", rel)
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, body, 0644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return executionCaptureIndex(t, root, api)
}
