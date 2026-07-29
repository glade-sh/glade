package gladecli

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
)

func TestTestStartupCacheStatusExplainsParallelMethodBypass(t *testing.T) {
	got := testStartupCacheStatus(t.TempDir(), apextest.Options{
		ParallelMethods: true,
		Parallelism:     2,
	})
	for _, want := range []string{
		"bypassed for parallel methods with more than one worker",
		"glade test serve",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("status missing %q: %s", want, got)
		}
	}
}

func TestTestStartupCacheStatusExplainsNoCache(t *testing.T) {
	got := testStartupCacheStatus(t.TempDir(), apextest.Options{NoDiskCache: true})
	for _, want := range []string{
		"disabled by --no-cache",
		"will not be read or written",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("status missing %q: %s", want, got)
		}
	}
}
