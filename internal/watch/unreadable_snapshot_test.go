package watch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUnreadableSnapshotPreservesMetadataContract(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "staticresources", "asset.resource")
	writeWatchFile(t, path, "content")
	scope := Scope{Roots: []string{root}}
	before, err := CaptureScope(scope)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0600) })
	if _, err := os.ReadFile(path); err == nil {
		t.Skip("file permissions do not deny reads for this user")
	}
	for name, capture := range map[string]func() (Snapshot, error){
		"scope":    func() (Snapshot, error) { return CaptureScope(scope) },
		"paths":    func() (Snapshot, error) { return CapturePaths([]string{path}) },
		"snapshot": func() (Snapshot, error) { return CaptureSnapshot(root) },
		"poll":     func() (Snapshot, error) { return captureScopeWithPrevious(scope, before) },
	} {
		t.Run(name, func(t *testing.T) {
			got, err := capture()
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := got.Files[path]; !ok {
				t.Fatal("stat-able file omitted")
			}
			if changes := DiffSnapshots(before, got); len(changes) != 0 {
				t.Fatalf("permission-only changes: %#v", changes)
			}
		})
	}
	unreadable, err := captureScopeWithPrevious(scope, before)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0600); err != nil {
		t.Fatal(err)
	}
	restored, err := captureScopeWithPrevious(scope, unreadable)
	if err != nil {
		t.Fatal(err)
	}
	if changes := DiffSnapshots(unreadable, restored); len(changes) != 0 {
		t.Fatalf("readability-only changes: %#v", changes)
	}

}
