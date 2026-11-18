package apextest

import (
	"os"
	"os/exec"
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

// A shared prefix built before a request must not be reused after the
// platform view changes. Requests persist, so start with a fresh process.
func TestGatedNamespaceRequestsInvalidateRuntimePrefix(t *testing.T) {
	if os.Getenv("GLADE_GATED_PREFIX_TEST") == "" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestGatedNamespaceRequestsInvalidateRuntimePrefix$")
		cmd.Env = append(os.Environ(), "GLADE_GATED_PREFIX_TEST=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("prefix subprocess: %v\n%s", err, output)
		}
		return
	}
	previous := sharedStandardRuntimePrefix()
	if previous == nil {
		t.Fatal("could not build cold runtime prefix")
	}
	for _, name := range []string{"Slack.App", "ConnectApi.FeedItem"} {
		typesys.NoteApexSource(name)
		current := sharedStandardRuntimePrefix()
		if current == nil || current == previous || current.generation <= previous.generation {
			t.Fatalf("%s request did not rebuild the runtime prefix", name)
		}
		previous = current
	}
}
