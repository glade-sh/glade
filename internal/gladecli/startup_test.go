package gladecli

import (
	"bytes"
	"context"
	"os"
	"testing"
)

func TestLightCommandsReturnWithoutProjectRuntime(t *testing.T) {
	originalCWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	if err := os.WriteFile(tmp+"/sfdx-project.json", []byte("{not-json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(originalCWD); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	})

	for _, args := range [][]string{
		{"version"},
		{"completion", "bash"},
		{"help"},
	} {
		t.Run(args[0], func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(context.Background(), args, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("Run(%v) code = %d stderr=%s", args, code, stderr.String())
			}
			if stdout.Len() == 0 {
				t.Fatalf("Run(%v) wrote no output", args)
			}
		})
	}
}
