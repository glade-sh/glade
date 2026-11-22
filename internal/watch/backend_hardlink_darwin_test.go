//go:build darwin

package watch

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatcherHardLinkedFileTargetEdits(t *testing.T) {
	for _, mode := range []string{"existing", "created-in-scope", "created-outside"} {
		t.Run(mode, func(t *testing.T) {
			workspace := t.TempDir()
			root := filepath.Join(workspace, "parent", "project")
			linked := filepath.Join(root, "Linked.cls")
			target := filepath.Join(workspace, "outside", "target.cls")
			writeWatchFile(t, filepath.Join(root, "Other.cls"), "other")
			if mode == "created-outside" {
				writeWatchFile(t, linked, "before")
				if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
					t.Fatal(err)
				}
			} else {
				writeWatchFile(t, target, "before")
				if mode == "existing" {
					if err := os.Link(target, linked); err != nil {
						t.Fatal(err)
					}
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
			if mode == "existing" {
				if _, ok := watcher.(*NativeWatcher); !ok {
					t.Fatalf("existing hard link used %T, want native fallback", watcher)
				}
			}
			if mode == "created-in-scope" {
				if err := os.Link(target, linked); err != nil {
					t.Fatal(err)
				}
				awaitScopedChange(t, watcher, linked, ChangeAdded)
			}
			if mode == "created-outside" {
				if err := os.Link(linked, target); err != nil {
					t.Fatal(err)
				}
			}
			time.Sleep(300 * time.Millisecond)
			if err := os.WriteFile(target, []byte("changed target"), 0600); err != nil {
				t.Fatal(err)
			}
			awaitScopedChange(t, watcher, linked, ChangeModified)
		})
	}
}
