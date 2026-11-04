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
			srv, err := NewServer(ServerConfig{Root: root})
			if srv != nil {
				_ = srv.Close()
			}
			if err == nil || !strings.Contains(err.Error(), "insecure") {
				t.Fatalf("default daemon start accepted %s: %v", kind, err)
			}
			if _, err := os.Lstat(socket); !os.IsNotExist(err) {
				t.Fatalf("socket created before rejecting unsafe ancestry: %v", err)
			}
		})
	}
}
