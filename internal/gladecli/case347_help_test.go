package gladecli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/cliui"
)

func TestHelpReferenceMatchesDispatchedTestHelp(t *testing.T) {
	var reference, dispatched bytes.Buffer
	if err := cliui.WriteTestHelp(&reference); err != nil {
		t.Fatal(err)
	}
	if err := writeTestHelp(&dispatched); err != nil {
		t.Fatal(err)
	}
	if reference.String() != dispatched.String() {
		t.Error("test help renderers disagree")
	}
	for _, want := range []string{"--read-only", "--shard-count", "--runtime-rest-api-version", "including watch/daemon", "incompatible with explicit --connect", "--no-warm"} {
		if !strings.Contains(dispatched.String(), want) {
			t.Errorf("test help missing %q", want)
		}
	}
}
