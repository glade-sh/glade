package scripts

import (
	"os/exec"
	"testing"
)

func TestReleaseStaticPublisher(t *testing.T) {
	if out, err := exec.Command("node", "--test", "release-publish.test.mjs").CombinedOutput(); err != nil {
		t.Fatalf("static release publisher: %v\n%s", err, out)
	}
}
