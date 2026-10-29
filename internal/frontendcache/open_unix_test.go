//go:build darwin || linux

package frontendcache

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func TestSnapshotReadRejectsFIFOWithoutWriter(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".glade", "semantic", name)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := unix.Mkfifo(path, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := read(root); err == nil {
		t.Fatal("accepted a FIFO as a snapshot")
	}
}
