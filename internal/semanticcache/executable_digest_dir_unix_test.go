//go:build unix

package semanticcache

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCacheBaseUsableForRoot(t *testing.T) {
	base := t.TempDir()
	if !cacheBaseUsableFor(os.Getuid()+1, base) {
		t.Fatal("a non-root process must always try its cache")
	}
	if os.Getuid() == 0 {
		t.Skip("running as root: the base is root owned")
	}
	if cacheBaseUsableFor(0, base) {
		t.Fatal("root must not create a cache inside another user's base directory")
	}
	if cacheBaseUsableFor(0, filepath.Join(base, "missing", "deeper")) {
		t.Fatal("root must not create a missing base inside another user's directory")
	}
}
