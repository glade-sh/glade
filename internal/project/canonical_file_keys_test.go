package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCanonicalFileKeysMatchPerPathResolution(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "real")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(real, "A.cls")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, filepath.Join(root, "linkdir")); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	if err := os.Symlink(file, filepath.Join(root, "linkfile.cls")); err != nil {
		t.Fatal(err)
	}
	paths := []string{
		file,
		filepath.Join(root, "linkdir", "A.cls"),
		filepath.Join(root, "linkfile.cls"),
		filepath.Join(root, "missing.cls"),
		filepath.Join(root, "linkdir", "missing.cls"),
		filepath.Join(root, "nodir", "x.cls"),
	}
	keys := newCanonicalFileKeys()
	for _, path := range paths {
		if got, want := keys.key(path), canonicalFileKey(path); got != want {
			t.Fatalf("key(%q) = %q, want %q", path, got, want)
		}
	}
	if got := dedupeFilePaths(keys, paths[:3]); len(got) != 1 || got[0] != file {
		t.Fatalf("dedupe = %v, want only %q", got, file)
	}
}
