package visualforce_test

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

func TestV09ClickRowClock(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is required for the click driver test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", "--test", "testdata/v09_click_row_frame_test.mjs", "testdata/v09_clock_regression_test.mjs")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("row clock test: %v\n%s", err, output)
	}
}
