package gladecli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestDevDirectHelpSelectionDescription(t *testing.T) {
	var out, errOut bytes.Buffer
	if _, _, err := runDev(context.Background(), []string{"--help"}, &out, &errOut); err != nil {
		t.Fatalf("dev help: %v; stderr: %s", err, errOut.String())
	}
	for _, want := range []string{"case-insensitive substring matching", "watch ignores them"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q", want)
		}
	}
}
