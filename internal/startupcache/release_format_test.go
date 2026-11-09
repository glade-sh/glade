package startupcache

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReleaseFormatRejectsV034Snapshot(t *testing.T) {
	root := t.TempDir()
	entry := currentStartupCacheTestEntry(t, root)
	if err := Write(&entry, SubdirTest); err != nil {
		t.Fatal(err)
	}
	if got, err := Read(root, SubdirTest); err != nil || got == nil {
		t.Fatalf("current cache: got %v, error %v", got, err)
	}
	path := filepath.Join(root, SubdirTest, stateHeaderFile)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	header := mustUnmarshalTestCacheHeader(t, data)
	header.FormatVersion = 3 // v0.3.4 format, with all other identity fields current.
	if err := os.WriteFile(path, mustMarshalTestCacheHeader(t, header), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := Read(root, SubdirTest); err != nil || got != nil {
		t.Fatalf("v0.3.4 cache: got %v, error %v; want cache miss", got, err)
	}
}
