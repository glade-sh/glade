package scripts

import (
	"os"
	"strings"
	"testing"
)

func TestReleasePublisherAuthoritySuitesAreGated(t *testing.T) {
	data, err := os.ReadFile("release_publish_test.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, suite := range []string{"release-publisher-authority.test.mjs", "release-verifier.test.mjs", "release-extension-authority.test.mjs"} {
		if !strings.Contains(string(data), suite) {
			t.Errorf("release publisher Go gate does not run %s", suite)
		}
	}
}
