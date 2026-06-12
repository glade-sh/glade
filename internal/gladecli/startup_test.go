package gladecli

import (
	"bytes"
	"context"
	"testing"
	"time"
)

func TestLightCommandsReturnWithoutProjectRuntime(t *testing.T) {
	for _, args := range [][]string{
		{"version"},
		{"completion", "bash"},
		{"help"},
	} {
		t.Run(args[0], func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			started := time.Now()
			code := Run(context.Background(), args, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("Run(%v) code = %d stderr=%s", args, code, stderr.String())
			}
			if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
				t.Fatalf("Run(%v) took %s; light commands should not build a project runtime", args, elapsed)
			}
			if stdout.Len() == 0 {
				t.Fatalf("Run(%v) wrote no output", args)
			}
		})
	}
}
