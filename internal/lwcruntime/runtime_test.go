package lwcruntime_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/lwc/compile"
)

func TestBrowserRuntimeSuite(t *testing.T) {
	if os.Getenv("GLADE_LWC_BROWSER") == "" {
		t.Skip("set GLADE_LWC_BROWSER=1 to run Playwright mount/event tests")
	}
	root, err := compile.FindRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "third_party", "lwc", "node_modules")); err != nil {
		t.Skip("run npm install in third_party/lwc")
	}
	cmd := exec.Command("npm", "test")
	cmd.Dir = filepath.Join(root, "lwcruntime")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("npm test: %v\n%s", err, out)
	}
}
