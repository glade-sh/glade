package watch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNetworkMetadataGitSinceRoutes(t *testing.T) {
	for _, route := range []string{"modified", "deleted", "renamed", "untracked"} {
		t.Run(route, func(t *testing.T) {
			root := t.TempDir()
			runGit(t, root, "init")
			runGit(t, root, "config", "user.email", "glade@example.test")
			runGit(t, root, "config", "user.name", "GLADE")
			path := filepath.Join(root, "networks/Synthetic Portal.network-meta.xml")
			body := `<Network><status>UnderConstruction</status><urlPathPrefix>synthetic</urlPathPrefix></Network>`
			writeWatchFile(t, path, body)
			runGit(t, root, "add", ".")
			runGit(t, root, "commit", "-m", "baseline")
			wantOp := ChangeModified
			switch route {
			case "modified":
				writeWatchFile(t, path, body+"\n")
			case "deleted":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				wantOp = ChangeDeleted
			case "renamed":
				next := filepath.Join(root, "networks/Renamed Portal.network-meta.xml")
				if err := os.Rename(path, next); err != nil {
					t.Fatal(err)
				}
				runGit(t, root, "add", ".")
				path = next
			case "untracked":
				path = filepath.Join(root, "networks/Another Portal.network-meta.xml")
				writeWatchFile(t, path, body)
				wantOp = ChangeAdded
			}
			changes, err := GitChangesSince(root, "HEAD")
			if err != nil {
				t.Fatal(err)
			}
			if len(changes) != 1 || changes[0].Path != path || changes[0].Op != wantOp || string(changes[0].Kind) != "network_metadata" {
				t.Fatalf("Network git %s changes = %#v, want %s %s", route, changes, wantOp, path)
			}
		})
	}
}

func TestGitNameStatusPreservesNetworkSpacesRoutes(t *testing.T) {
	for _, status := range []string{"M", "A", "D", "R100"} {
		t.Run(status, func(t *testing.T) {
			root := t.TempDir()
			path := "networks/Synthetic Portal.network-meta.xml"
			line := status + "\t" + path + "\n"
			if status == "R100" {
				line = status + "\tnetworks/Previous Portal.network-meta.xml\t" + path + "\n"
			}
			changes := parseGitNameStatus(root, []byte(line))
			if len(changes) != 1 || changes[0].Path != filepath.Join(root, path) {
				t.Fatalf("Network git parser %s = %#v", status, changes)
			}
		})
	}
}
