package watch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNetworkFileClassificationRoutes(t *testing.T) {
	for _, suffix := range []string{".network", ".NETWORK", ".network-meta.xml", ".NETWORK-META.XML"} {
		t.Run(suffix, func(t *testing.T) {
			path := filepath.Join("networks", "Synthetic Portal"+suffix)
			got := ClassifyPath(path)
			if !got.Watchable || got.Kind != FileKindNetworkMeta || got.Name != "Synthetic Portal" {
				t.Fatalf("ClassifyPath = %#v", got)
			}
			scoped := classifyScopePath(Scope{}, path)
			if scoped != got {
				t.Fatalf("scope classification = %#v, want %#v", scoped, got)
			}
		})
	}
}

func TestNetworkFileClassificationRejectsUnsupportedSuffixRoutes(t *testing.T) {
	for _, suffix := range []string{".network.bak", ".network-meta.xml.bak", ".networks", ".xml"} {
		t.Run(suffix, func(t *testing.T) {
			path := filepath.Join("networks", "Synthetic Portal"+suffix)
			got := ClassifyPath(path)
			if got.Watchable || got.Kind != FileKindIgnored {
				t.Fatalf("unsupported Network suffix classification = %#v", got)
			}
			if scoped := classifyScopePath(Scope{}, path); scoped != got {
				t.Fatalf("unsupported scoped suffix classification = %#v, want %#v", scoped, got)
			}
		})
	}
}

func TestNetworkFileSnapshotLifecycleRoutes(t *testing.T) {
	for _, route := range []string{"snapshot", "scope_roots", "scope_files", "paths", "native_pending", "native_full"} {
		for _, op := range []string{"modified", "deleted", "renamed", "added"} {
			t.Run(route+"/"+op, func(t *testing.T) {
				root := t.TempDir()
				path := filepath.Join(root, "networks/Synthetic Portal.network")
				next := filepath.Join(root, "networks/Renamed Portal.network")
				body := `<Network><status>UnderConstruction</status></Network>`
				writeWatchFile(t, path, body)
				paths := []string{path, next}
				scope := Scope{Roots: []string{root}}
				if route == "scope_files" {
					scope = Scope{Files: paths}
				}
				capture := func() (Snapshot, error) {
					switch route {
					case "snapshot":
						return CaptureSnapshot(root)
					case "paths":
						return CapturePaths(paths)
					default:
						return CaptureScope(scope)
					}
				}
				before, err := capture()
				if err != nil {
					t.Fatal(err)
				}
				want := map[string]ChangeOp{path: ChangeModified}
				switch op {
				case "modified":
					writeWatchFile(t, path, body+"\n")
				case "deleted":
					if err := os.Remove(path); err != nil {
						t.Fatal(err)
					}
					want[path] = ChangeDeleted
				case "renamed":
					if err := os.Rename(path, next); err != nil {
						t.Fatal(err)
					}
					want = map[string]ChangeOp{path: ChangeDeleted, next: ChangeAdded}
				case "added":
					writeWatchFile(t, next, body)
					want = map[string]ChangeOp{next: ChangeAdded}
				}
				var changes []Change
				if route == "native_pending" || route == "native_full" {
					changes, _, err = snapshotPendingScope(scope, before, paths, route == "native_full")
				} else {
					var after Snapshot
					after, err = capture()
					changes = DiffSnapshots(before, after)
				}
				if err != nil {
					t.Fatal(err)
				}
				if len(changes) != len(want) {
					t.Fatalf("changes = %#v, want %#v", changes, want)
				}
				for _, change := range changes {
					if change.Op != want[change.Path] || change.Kind != FileKindNetworkMeta || change.Name != trimSuffixFold(filepath.Base(change.Path), ".network") {
						t.Errorf("change = %#v, want %#v", change, want)
					}
				}
			})
		}
	}
}

func TestNetworkFileGitSelectionRoutes(t *testing.T) {
	for _, route := range []string{"unstaged_edit", "staged_edit", "deleted", "renamed", "untracked"} {
		t.Run(route, func(t *testing.T) {
			root := t.TempDir()
			runGit(t, root, "init")
			runGit(t, root, "config", "user.email", "glade@example.test")
			runGit(t, root, "config", "user.name", "GLADE")
			path := filepath.Join(root, "networks/Synthetic Portal.network")
			body := `<Network><status>UnderConstruction</status></Network>`
			writeWatchFile(t, path, body)
			runGit(t, root, "add", ".")
			runGit(t, root, "commit", "-m", "baseline")
			wantOp := ChangeModified
			switch route {
			case "unstaged_edit", "staged_edit":
				writeWatchFile(t, path, body+"\n")
				if route == "staged_edit" {
					runGit(t, root, "add", ".")
				}
			case "deleted":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				wantOp = ChangeDeleted
			case "renamed":
				next := filepath.Join(root, "networks/Renamed Portal.network")
				if err := os.Rename(path, next); err != nil {
					t.Fatal(err)
				}
				runGit(t, root, "add", ".")
				path = next
			case "untracked":
				path = filepath.Join(root, "networks/Another Portal.network")
				writeWatchFile(t, path, body)
				wantOp = ChangeAdded
			}
			changes, err := GitChangesSince(root, "HEAD")
			if err != nil {
				t.Fatal(err)
			}
			if len(changes) != 1 || changes[0].Path != path || changes[0].Op != wantOp || changes[0].Kind != FileKindNetworkMeta || changes[0].Name != trimSuffixFold(filepath.Base(path), ".network") {
				t.Fatalf("GitChangesSince = %#v, want %s %s", changes, wantOp, path)
			}
		})
	}
}
