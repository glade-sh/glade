//go:build linux

package server

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestCase340DifferentUIDCannotReplaceCache(t *testing.T) {
	if err := exec.Command("sudo", "-n", "-u", "nobody", "true").Run(); err != nil {
		t.Skip("noninteractive different-UID helper unavailable")
	}
	current, err1 := os.Stat(os.TempDir())
	alternate, err2 := os.Stat("/var/tmp")
	if err1 != nil || err2 != nil {
		t.Skip("no accessible runner temporary directory")
	}
	a, ok1 := current.Sys().(*syscall.Stat_t)
	b, ok2 := alternate.Sys().(*syscall.Stat_t)
	if !ok1 || !ok2 || a.Dev != b.Dev {
		t.Skip("accessible temporary directory is on another filesystem")
	}
	var space syscall.Statfs_t
	if err := syscall.Statfs("/var/tmp", &space); err != nil || space.Bavail*uint64(space.Bsize) < 50*1024*1024*1024 {
		t.Skip("temporary filesystem has less than 50 GiB available")
	}
	t.Setenv("TMPDIR", "/var/tmp")
	cache := lightningCacheRoot(project.Project{})
	if cache == "" {
		t.Fatal("failed to create private cache")
	}
	defer os.RemoveAll(cache)
	before, err := os.Lstat(cache)
	if err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(cache, "marker.js")
	if err := os.WriteFile(marker, []byte("safe"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("sudo", "-n", "-u", "nobody", "mv", cache, cache+"-replaced").Run(); err == nil {
		if moved, statErr := os.Lstat(cache + "-replaced"); statErr == nil && os.SameFile(before, moved) {
			defer os.RemoveAll(cache + "-replaced")
		}
		t.Fatal("different UID replaced private cache")
	}
	after, err := os.Lstat(cache)
	if err != nil || !os.SameFile(before, after) {
		t.Fatalf("cache identity changed: %v", err)
	}
	content, err := os.ReadFile(marker)
	if err != nil || string(content) != "safe" {
		t.Fatalf("cache content changed: %q %v", content, err)
	}
}
