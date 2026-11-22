//go:build darwin

package watch

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatcherCaseInsensitiveRoot(t *testing.T) {
	workspace := t.TempDir()
	root := filepath.Join(workspace, "SampleProject")
	alias := filepath.Join(workspace, "sampleproject")
	class := filepath.Join(root, "Example.cls")
	writeWatchFile(t, class, "before")
	actual, err := os.Stat(root)
	if err != nil {
		t.Fatal(err)
	}
	other, err := os.Stat(alias)
	if os.IsNotExist(err) {
		t.Skip("filesystem distinguishes path case")
	}
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(actual, other) {
		t.Skip("case variant identifies a different directory")
	}
	initial, err := CaptureScope(Scope{Roots: []string{alias}})
	if err != nil {
		t.Fatal(err)
	}
	watcher, _, err := NewBackendWatcher(context.Background(), Config{Root: alias, Backend: BackendNative, Debounce: 20 * time.Millisecond}, initial)
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.Close()
	time.Sleep(300 * time.Millisecond)
	replaceWatchFile(t, class, "after")
	awaitScopedChange(t, watcher, filepath.Join(alias, "Example.cls"), ChangeModified)
}
