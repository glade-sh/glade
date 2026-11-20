package watch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

func TestCaptureScopeSkipsUnreadableNestedConfigs(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"sfdx-project.json", "glade.yml"} {
		path := filepath.Join(root, "nested", name)
		writeWatchFile(t, path, "excluded")
		if err := os.Chmod(path, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(path, 0600) })
		if _, err := os.ReadFile(path); err == nil {
			t.Skip("file permissions do not deny reads for this user")
		}
	}
	class := filepath.Join(root, "Example.cls")
	writeWatchFile(t, class, "class Example {}")
	snapshot, err := CaptureScope(Scope{Roots: []string{root}})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Files) != 1 || snapshot.Files[class].Path != class {
		t.Fatalf("unexpected files: %#v", snapshot.Files)
	}
}

func TestSnapshotLargeResourceUsesBoundedMemory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "staticresources", "asset.resource")
	writeWatchFile(t, path, strings.Repeat("a", 8<<20))
	result := testing.Benchmark(func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if _, err := CapturePaths([]string{path}); err != nil {
				b.Fatal(err)
			}
		}
	})
	if got := result.AllocedBytesPerOp(); got > 1<<20 {
		t.Fatalf("capture allocated %d bytes for an 8 MiB resource; want at most 1 MiB", got)
	}
}

func TestSnapshotResourceContentAtEnd(t *testing.T) {
	path := filepath.Join(t.TempDir(), "staticresources", "asset.resource")
	prefix := strings.Repeat("a", 128<<10)
	writeWatchFile(t, path, prefix+"x")
	stamp := time.Unix(1600000000, 0)
	if err := os.Chtimes(path, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	before, err := CapturePaths([]string{path})
	if err != nil {
		t.Fatal(err)
	}
	writeWatchFile(t, path, prefix+"y")
	if err := os.Chtimes(path, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	after, err := CapturePaths([]string{path})
	if err != nil {
		t.Fatal(err)
	}
	changes := DiffSnapshots(before, after)
	if len(changes) != 1 || changes[0].Path != path || changes[0].Op != ChangeModified {
		t.Fatalf("changes = %#v", changes)
	}
}

func TestPollingContentStampInvalidation(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Example.cls")
	writeWatchFile(t, path, "class Example { void first() {} }")
	stamp := time.Unix(1600000000, 0)
	if err := os.Chtimes(path, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	scope := Scope{Roots: []string{root}}
	// A freshly written file must not be trusted until the filesystem clock's
	// safety window has passed.
	young, err := CaptureScope(scope)
	if err != nil {
		t.Fatal(err)
	}
	if young.Files[path].stampValid {
		t.Fatal("recent file stamp was trusted")
	}
	time.Sleep(2100 * time.Millisecond)
	before, err := CaptureScope(scope)
	if err != nil {
		t.Fatal(err)
	}
	state := before.Files[path]
	if !state.stampValid {
		t.Skip("filesystem does not provide a trusted stamp")
	}
	// A sentinel digest makes reuse observable without relying on timing.
	sentinel := state
	sentinel.contentHash[0] ^= 0xff
	before.Files[path] = sentinel
	reused, err := captureScopeWithPrevious(scope, before)
	if err != nil {
		t.Fatal(err)
	}
	if reused.Files[path].contentHash != sentinel.contentHash {
		t.Fatal("unchanged trusted stamp did not reuse the digest")
	}
	// Without trust, the previous digest may not stand in for a read.
	untrusted := sentinel
	untrusted.stampValid = false
	before.Files[path] = untrusted
	refreshed, err := captureScopeWithPrevious(scope, before)
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.Files[path].contentHash != state.contentHash {
		t.Fatal("untrusted stamp did not trigger a read")
	}
	before.Files[path] = state
	writeWatchFile(t, path, "class Example { void other() {} }")
	if err := os.Chtimes(path, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	after, err := captureScopeWithPrevious(scope, before)
	if err != nil {
		t.Fatal(err)
	}
	if changes := DiffSnapshots(before, after); len(changes) != 1 || changes[0].Op != ChangeModified {
		t.Fatalf("restored edit: %#v", changes)
	}
	if after.Files[path].stampValid {
		t.Fatal("recent edit was trusted")
	}
	// Atomic replacement at the same path, size and mtime changes inode.
	replacement := filepath.Join(root, "replacement")
	writeWatchFile(t, replacement, "class Example { void third() {} }")
	if err := os.Chtimes(replacement, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, path); err != nil {
		t.Fatal(err)
	}
	replaced, err := captureScopeWithPrevious(scope, after)
	if err != nil {
		t.Fatal(err)
	}
	if changes := DiffSnapshots(after, replaced); len(changes) != 1 || changes[0].Op != ChangeModified {
		t.Fatalf("replacement: %#v", changes)
	}
}
