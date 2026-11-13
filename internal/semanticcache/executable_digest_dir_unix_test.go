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

func TestPersistedGateDirFollowsCacheBaseOwnership(t *testing.T) {
	base := t.TempDir()
	if got := persistedGateDirIn(base, os.Getuid()); got == "" {
		t.Fatal("an owner's base must be usable")
	}
	// Root with a base owned by another user, or a missing base under one,
	// must not get a directory to create.
	if got := persistedGateDirIn(base, 0); got != "" && os.Getuid() != 0 {
		t.Fatalf("root must not create the gate cache in a foreign base, got %q", got)
	}
	if got := persistedGateDirIn(filepath.Join(base, "missing", "deeper"), 0); got != "" && os.Getuid() != 0 {
		t.Fatalf("root must check the nearest existing ancestor, got %q", got)
	}
}
