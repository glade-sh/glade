package visualforce_test

import (
	"os/exec"
	"testing"
)

func TestV09ClickRetryHarness(t *testing.T) {
	out, err := exec.Command("node", "--test", "testdata/v09_click_retry_test.mjs").CombinedOutput()
	if err != nil {
		t.Fatalf("browser retry harness: %v\n%s", err, out)
	}
}
