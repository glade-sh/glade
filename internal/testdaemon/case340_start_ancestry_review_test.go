//go:build darwin || linux

package testdaemon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCase340DefaultDaemonRejectsUntrustedStartAncestry(t *testing.T) {
	for _, kind := range []string{"project", "ancestor"} {
		t.Run(kind, func(t *testing.T) {
			base, err := os.MkdirTemp(os.TempDir(), "r340-")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(base)
			root := filepath.Join(base, "project")
			if err := os.Mkdir(root, 0700); err != nil {
				t.Fatal(err)
			}
			writeTestProject(t, root)
			socket := ServeSocketPath(root)
			if !strings.HasPrefix(socket, root+string(filepath.Separator)) {
				t.Fatalf("fixture unexpectedly uses fallback socket: %s", socket)
			}
			unsafe := root
			if kind == "ancestor" {
				unsafe = base
			}
			if err := os.Chmod(unsafe, 0777); err != nil {
				t.Fatal(err)
			}
			// An unsafe location is refused and discovery moves to
			// the per-user cache directory instead of failing.
			cache := t.TempDir()
			t.Setenv("HOME", cache)
			t.Setenv("XDG_CACHE_HOME", cache)
			userCache, _ := os.UserCacheDir()
			if moved := ServeSocketPath(root); !strings.HasPrefix(moved, filepath.Join(userCache, "glade")+string(filepath.Separator)) {
				t.Fatalf("unsafe %s did not fall back to the user cache directory: %s", kind, moved)
			}
			srv, err := NewServer(ServerConfig{Root: root})
			if srv != nil {
				_ = srv.Close()
			}
			if err != nil {
				t.Fatalf("default daemon start did not fall back for %s: %v", kind, err)
			}
			if _, err := os.Lstat(socket); !os.IsNotExist(err) {
				t.Fatalf("socket created before rejecting unsafe ancestry: %v", err)
			}
		})
	}
}
