package gladecli

import (
	"strings"
	"sync"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
)

var runtimeCacheOwner struct {
	sync.Mutex
	test string
}

// releaseStaleRuntimeCaches drops the in-memory compiled-runtime caches the
// first time a different top-level test writes a fixture. Each cached runtime
// is keyed by project path and content, so entries left by an earlier test are
// never read again. Without this the package accumulates about 100 MB per
// project, which roughly triples GC time and exhausts memory on a 16 GB host.
// Subtests of one top-level test still share their caches.
func releaseStaleRuntimeCaches(t *testing.T) {
	top, _, _ := strings.Cut(t.Name(), "/")
	runtimeCacheOwner.Lock()
	changed := runtimeCacheOwner.test != top
	runtimeCacheOwner.test = top
	runtimeCacheOwner.Unlock()
	if changed {
		apextest.InvalidateRuntimeCaches()
	}
}
