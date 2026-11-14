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
