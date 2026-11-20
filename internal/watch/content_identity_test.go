package watch

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSnapshotContentIdentityRoutes(t *testing.T) {
	for _, route := range []string{"snapshot", "paths", "scope", "pending", "full"} {
		t.Run(route, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "Example.cls")
			writeWatchFile(t, path, "class Example { void first() {} }")
			stamp := time.Unix(1600000000, 0)
			if err := os.Chtimes(path, stamp, stamp); err != nil {
				t.Fatal(err)
			}
			scope := NormalizeScope(Scope{Roots: []string{root}})
			capture := func() (Snapshot, error) {
				switch route {
				case "snapshot":
					return CaptureSnapshot(root)
				case "paths":
					return CapturePaths([]string{path})
				default:
					return CaptureScope(scope)
				}
			}
			before, err := capture()
			if err != nil {
				t.Fatal(err)
			}
			for _, body := range []string{"class Example { void other() {} }", "class Example { void first() {} }"} {
				writeWatchFile(t, path, body)
				if err := os.Chtimes(path, stamp, stamp); err != nil {
					t.Fatal(err)
				}
				var after Snapshot
				var changes []Change
				if route == "pending" || route == "full" {
					changes, after, err = snapshotPendingScope(scope, before, []string{path}, route == "full")
				} else {
					after, err = capture()
					if err == nil {
						changes = DiffSnapshots(before, after)
					}
				}
				if err != nil {
					t.Fatal(err)
				}
				if len(changes) != 1 || changes[0].Op != ChangeModified || changes[0].Path != path {
					t.Fatalf("changes = %#v, want one modification", changes)
				}
				unchanged, err := capture()
				if err != nil {
					t.Fatal(err)
				}
				if got := DiffSnapshots(after, unchanged); len(got) != 0 {
					t.Fatalf("duplicate event: %#v", got)
				}
				before = after
			}
		})
	}
}
