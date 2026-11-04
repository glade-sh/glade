package scripts

import (
	"os/exec"
	"testing"
)

func TestReleaseStaticPublisher(t *testing.T) {
	if out, err := exec.Command("node", "--test", "release-publish.test.mjs", "release-publisher-authority.test.mjs", "release-verifier.test.mjs", "release-extension-authority.test.mjs").CombinedOutput(); err != nil {
		t.Fatalf("static release publisher: %v\n%s", err, out)
	}
}
