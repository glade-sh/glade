//go:build darwin

package watch

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatcherLinkedFileTargetEdits(t *testing.T) {
	for _, createLater := range []bool{false, true} {
		name := "existing"
		if createLater {
			name = "created-after-start"
		}
		t.Run(name, func(t *testing.T) {
			workspace := t.TempDir()
			root := filepath.Join(workspace, "parent", "project")
			linked := filepath.Join(root, "Linked.cls")
			target := filepath.Join(workspace, "outside", "target.cls")
			writeWatchFile(t, filepath.Join(root, "Other.cls"), "other")
			writeWatchFile(t, target, "before")
			if !createLater {
				if err := os.Symlink(target, linked); err != nil {
					t.Fatal(err)
				}
			}
			initial, err := CaptureScope(Scope{Roots: []string{root}})
			if err != nil {
				t.Fatal(err)
			}
			watcher, _, err := NewBackendWatcher(context.Background(), Config{Root: root, Backend: BackendNative, Debounce: 20 * time.Millisecond}, initial)
			if err != nil {
				t.Fatal(err)
			}
			defer watcher.Close()
			if createLater {
				if err := os.Symlink(target, linked); err != nil {
					t.Fatal(err)
				}
				awaitScopedChange(t, watcher, linked, ChangeAdded)
			}
			if err := os.WriteFile(target, []byte("changed target"), 0600); err != nil {
				t.Fatal(err)
			}
			awaitScopedChange(t, watcher, linked, ChangeModified)
		})
	}
}
