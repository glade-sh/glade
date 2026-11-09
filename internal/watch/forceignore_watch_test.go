package watch

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A nested .forceignore decides which package source loads, so creating,
// editing and removing one must reach the daemon like a project config edit.
func TestScopedBackendsReportNestedForceIgnoreLifecycle(t *testing.T) {
	for _, backend := range []Backend{BackendPoll, BackendNative} {
		t.Run(string(backend), func(t *testing.T) {
			root := t.TempDir()
			writeWatchFile(t, filepath.Join(root, "force-app", "classes", "A.cls"), "public class A {}")
			ignore := filepath.Join(root, "force-app", "classes", ".forceignore")
			scope := Scope{Roots: []string{root}}
			initial, err := CaptureScope(scope)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			watcher, _, err := NewBackendWatcher(ctx, Config{Root: root, Scope: scope, Backend: backend, Debounce: 15 * time.Millisecond}, initial)
			if err != nil {
				if backend == BackendNative {
					t.Skipf("native watcher unavailable: %v", err)
				}
				t.Fatal(err)
			}
			defer watcher.Close()

			writeWatchFile(t, ignore, "A.cls\n")
			awaitScopedChange(t, watcher, ignore, ChangeAdded)
			writeWatchFile(t, ignore, "A.cls*\n")
			awaitScopedChange(t, watcher, ignore, ChangeModified)
			if err := os.Remove(ignore); err != nil {
				t.Fatal(err)
			}
			awaitScopedChange(t, watcher, ignore, ChangeDeleted)
		})
	}
}
