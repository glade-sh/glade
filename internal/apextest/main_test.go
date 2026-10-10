package apextest

import (
	"os"
	"strings"
	"sync"
	"testing"
)

func TestMain(m *testing.M) {
	// Tests use unique t.TempDir() roots, so disk cache only adds gob I/O with
	// no hit rate. In-memory runtimeCache still applies.
	disableDiskCache.Store(true)
	code := m.Run()
	removeSharedDiskRuntimeIsolation()
	os.Exit(code)
}

var runtimeCacheOwner struct {
	sync.Mutex
	test    string
	cleanup bool
}

// releaseRuntimeCachesAfterTest keeps the in-memory runtime caches from
// carrying one test's entries into the next. Each test builds its own project,
// so an entry is never read by a later test and only holds memory. A top-level
// test drops the caches when it ends. Subtests never drop their parent's
// entries; the first subtest call of a new top-level test drops what an
// earlier test left behind.
func releaseRuntimeCachesAfterTest(t *testing.T) {
	top, _, subtest := strings.Cut(t.Name(), "/")
	runtimeCacheOwner.Lock()
	changed := runtimeCacheOwner.test != top
	if changed {
		runtimeCacheOwner.test, runtimeCacheOwner.cleanup = top, false
	}
	register := !subtest && !runtimeCacheOwner.cleanup
	if register {
		runtimeCacheOwner.cleanup = true
	}
	runtimeCacheOwner.Unlock()
	if changed && subtest {
		InvalidateRuntimeCaches()
	}
	if register {
		t.Cleanup(func() {
			runtimeCacheOwner.Lock()
			if runtimeCacheOwner.test == top {
				runtimeCacheOwner.test, runtimeCacheOwner.cleanup = "", false
			}
			runtimeCacheOwner.Unlock()
			InvalidateRuntimeCaches()
		})
	}
}
