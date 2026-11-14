package typesys

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// The platform view is process-wide, so exercise the cold view in a subprocess
// even when other tests in this package have already requested a namespace.
func TestGatedPlatformNamespacesColdAndRequested(t *testing.T) {
	if os.Getenv("GLADE_GATED_PLATFORM_COLD") == "" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestGatedPlatformNamespacesColdAndRequested$")
		cmd.Env = append(os.Environ(), "GLADE_GATED_PLATFORM_COLD=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("cold subprocess: %v\n%s", err, output)
		}
		return
	}
	count := func(namespace string) int {
		n := 0
		for _, symbol := range StandardPlatformSymbolView() {
			if strings.EqualFold(symbol.Namespace, namespace) || strings.HasPrefix(strings.ToLower(symbol.Namespace), strings.ToLower(namespace)+".") {
				n++
			}
		}
		return n
	}
	if got := count("ConnectApi"); got > 64 {
		t.Fatalf("cold view retained %d ConnectApi types", got)
	}
	if got := count("Slack"); got != 0 {
		t.Fatalf("cold view retained %d Slack types", got)
	}
	before := StandardPlatformSymbolGeneration()
	NoteApexSource("class UsesSlack { Slack.App value; }")
	if got := count("Slack"); got < 100 {
		t.Fatalf("Slack request retained only %d types", got)
	}
	if got := count("ConnectApi"); got > 64 {
		t.Fatalf("Slack request loaded %d ConnectApi types", got)
	}
	if StandardPlatformSymbolGeneration() == before {
		t.Fatal("Slack request did not invalidate the symbol view")
	}
	before = StandardPlatformSymbolGeneration()
	NoteApexSource("class UsesConnect { ConnectApi.FeedItem value; }")
	if got := count("ConnectApi"); got < 100 {
		t.Fatalf("ConnectApi request retained only %d types", got)
	}
	if StandardPlatformSymbolGeneration() == before {
		t.Fatal("ConnectApi request did not invalidate the symbol view")
	}
}

// Each input to the platform snapshot must invalidate derived views, including
// a stub-only inclusion followed by the captured ConnectApi shape.
func TestGatedPlatformViewCacheInvalidation(t *testing.T) {
	if os.Getenv("GLADE_GATED_CACHE_TEST") == "" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestGatedPlatformViewCacheInvalidation$")
		cmd.Env = append(os.Environ(), "GLADE_GATED_CACHE_TEST=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("cold subprocess: %v\n%s", err, output)
		}
		return
	}
	var cache PlatformViewCache[uint64]
	builds := uint64(0)
	get := func() uint64 {
		return cache.GetWithExcluded(func(_ []TypeSymbol, _ ExcludedPlatformTypes) uint64 {
			builds++
			return builds
		})
	}
	previous := get()
	if get() != previous {
		t.Fatal("unchanged view rebuilt cache")
	}
	steps := []struct {
		name  string
		apply func()
	}{
		{"gate disabled", func() { SetPlatformGateDisabled(true) }},
		{"gate enabled", func() { SetPlatformGateDisabled(false) }},
		{"stub inclusion", func() { IncludeGatedPlatformNamespaceFor("ConnectApi.FeedItem") }},
		{"captured shape", func() { RequestGatedPlatformNamespaceFor("ConnectApi.FeedItem") }},
		{"other namespace", func() { NoteApexSource("Slack.App") }},
	}
	for _, step := range steps {
		step.apply()
		got := get()
		if got == previous {
			t.Fatalf("%s did not invalidate cache", step.name)
		}
		previous = got
	}
}
