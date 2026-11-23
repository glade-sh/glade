package compile

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestStyleSelectorGoldenAndPreparation(t *testing.T) {
	root, err := FindRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	toolchain := filepath.Join(root, "third_party", "lwc")
	if _, err := os.Stat(filepath.Join(toolchain, "node_modules", "@lwc", "style-compiler")); err != nil && os.Getenv("GLADE_LWC_COMPILE") == "" {
		t.Skipf("LWC compiler toolchain unavailable: %v", err)
	}
	cmd := exec.Command("node", "--test", "--test-concurrency=1", "scripts/prepare-lwc-shared.test.mjs", "scripts/style-selector.test.mjs")
	cmd.Dir = toolchain
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("LWC selector and preparation tests: %v\n%s", err, output)
	}
	t.Logf("%s", output)
}
