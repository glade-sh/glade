package oracle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateScriptsWritesNonStoppingFullRunLaunchers(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "scripts")
	queue := WorkQueue{}
	if err := GenerateScripts(queue, "oracle-run", ".glade/oracle/runs", "glade-probe-lab", outDir, 2); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"nightly-full.sh", "07-run-all-shards.sh"} {
		raw, err := os.ReadFile(filepath.Join(outDir, name))
		if err != nil {
			t.Fatal(err)
		}
		content := string(raw)
		if !strings.Contains(content, "FAILURES=0") || !strings.Contains(content, "GLADE_ORACLE_STRICT") {
			t.Fatalf("%s content = %s", name, content)
		}
		if strings.Contains(content, "set -euo pipefail") {
			t.Fatalf("%s should not stop on first shard failure: %s", name, content)
		}
	}
}
