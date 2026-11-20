package watch

import (
	"encoding/json"
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

func TestSnapshotOrdinaryEditWireFields(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Example.cls")
	writeWatchFile(t, path, "class Example {}")
	first := time.Unix(1600000000, 0)
	second := first.Add(time.Second)
	if err := os.Chtimes(path, first, first); err != nil {
		t.Fatal(err)
	}
	before, err := CaptureSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	writeWatchFile(t, path, "class Example { void run() {} }")
	if err := os.Chtimes(path, second, second); err != nil {
		t.Fatal(err)
	}
	after, err := CaptureSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	changes := DiffSnapshots(before, after)
	want := []Change{{Path: path, Op: ChangeModified, Kind: FileKindApexClass, Name: "Example", Before: &FileMetadata{ModTimeUnixNano: first.UnixNano(), Size: int64(len("class Example {}"))}, After: &FileMetadata{ModTimeUnixNano: second.UnixNano(), Size: int64(len("class Example { void run() {} }"))}}}
	gotJSON, err := json.Marshal(changes)
	if err != nil {
		t.Fatal(err)
	}
	wantJSON, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotJSON) != string(wantJSON) {
		t.Fatalf("change JSON = %s, want %s", gotJSON, wantJSON)
	}
}

func TestSnapshotIdentityInvalidationInputs(t *testing.T) {
	for _, change := range []string{"mtime", "size", "delete", "add"} {
		t.Run(change, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "Example.cls")
			stamp := time.Unix(1600000000, 0)
			if change != "add" {
				writeWatchFile(t, path, "class Example {}")
				if err := os.Chtimes(path, stamp, stamp); err != nil {
					t.Fatal(err)
				}
			}
			before, err := CapturePaths([]string{path})
			if err != nil {
				t.Fatal(err)
			}
			want := ChangeModified
			switch change {
			case "mtime":
				if err := os.Chtimes(path, stamp.Add(time.Second), stamp.Add(time.Second)); err != nil {
					t.Fatal(err)
				}
			case "size":
				writeWatchFile(t, path, "class Example { }")
				if err := os.Chtimes(path, stamp, stamp); err != nil {
					t.Fatal(err)
				}
			case "delete":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				want = ChangeDeleted
			case "add":
				writeWatchFile(t, path, "class Example {}")
				want = ChangeAdded
			}
			after, err := CapturePaths([]string{path})
			if err != nil {
				t.Fatal(err)
			}
			changes := DiffSnapshots(before, after)
			if len(changes) != 1 || changes[0].Path != path || changes[0].Op != want {
				t.Fatalf("changes = %#v, want %s", changes, want)
			}
		})
	}
}

func TestSnapshotReportsUnreadableFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Example.cls")
	writeWatchFile(t, path, "class Example {}")
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0600) })
	if _, err := os.ReadFile(path); err == nil {
		t.Skip("file permissions do not deny reads for this user")
	}
	if _, err := CapturePaths([]string{path}); err == nil {
		t.Fatal("snapshot accepted a file whose content could not be read")
	}
}
