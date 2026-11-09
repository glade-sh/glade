package lwcbrowser

import (
	"os/exec"
	"testing"
)

// This local boundary is deliberate; it is not a claim about a native mutation.
func TestB11GraphMutationHostedBoundary(t *testing.T) {
	cmd := exec.Command("node", "testdata/b11_small_exports/graph-boundary.mjs")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("mutation boundary: %v\n%s", err, out)
	}
}
