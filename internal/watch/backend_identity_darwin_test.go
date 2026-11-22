//go:build darwin

package watch

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestWatcherChangesMatchSnapshotDiff(t *testing.T) {
	for _, multipleRoots := range []bool{false, true} {
		name := "single-root"
		if multipleRoots {
			name = "multiple-roots"
		}
		t.Run(name, func(t *testing.T) {
			workspace := t.TempDir()
			root := filepath.Join(workspace, "project")
			class := filepath.Join(root, "Example.cls")
			writeWatchFile(t, class, "before")
			scope := Scope{Roots: []string{root}}
			if multipleRoots {
				second := filepath.Join(workspace, "dependency")
				writeWatchFile(t, filepath.Join(second, "Other.cls"), "other")
				scope.Roots = append(scope.Roots, second)
			}
			initial, err := CaptureScope(scope)
			if err != nil {
				t.Fatal(err)
			}
			watcher, _, err := NewBackendWatcher(context.Background(), Config{Root: root, Scope: scope, Backend: BackendNative, Debounce: 50 * time.Millisecond}, initial)
			if err != nil {
				t.Fatal(err)
			}
			defer watcher.Close()
			replaceWatchFile(t, class, "after")
			current, err := CaptureScope(scope)
			if err != nil {
				t.Fatal(err)
			}
			want := DiffSnapshots(initial, current)
			if len(want) != 1 {
				t.Fatalf("snapshot diff: %#v", want)
			}
			select {
			case got := <-watcher.Changes():
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("changes = %#v, snapshot diff = %#v", got, want)
				}
			case err := <-watcher.Errors():
				t.Fatal(err)
			case <-time.After(2 * time.Second):
				t.Fatal("missing watcher change")
			}
			if err := os.WriteFile(filepath.Join(workspace, "unrelated.cls"), []byte("outside"), 0600); err != nil {
				t.Fatal(err)
			}
			select {
			case got := <-watcher.Changes():
				t.Fatalf("unrelated file produced changes: %#v", got)
			case err := <-watcher.Errors():
				t.Fatal(err)
			case <-time.After(250 * time.Millisecond):
			}
		})
	}
}
