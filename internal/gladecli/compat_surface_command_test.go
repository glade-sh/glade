package gladecli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompatSurfaceRefreshWritesReports(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, "docs")
	out := filepath.Join(root, "out")
	if err := os.MkdirAll(filepath.Join(docs, "apex"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docs, "apex", "system_label.md"), []byte("# Label Class\n\n### get(String section, String key)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tooling := filepath.Join(root, "tooling.json")
	if err := os.WriteFile(tooling, []byte(`{"publicDeclarations":{"System":{"Label":{"methods":[{"name":"get","returnType":"String","parameters":[{"type":"String"},{"type":"String"}]}]}}}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"compat", "surface", "refresh", "--docs", docs, "--tooling-completions", tooling, "--out", out}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "surface refresh: ok") {
		t.Fatalf("compact summary missing: %q", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(out, "SURFACE_LEDGER.json")); err != nil {
		t.Fatalf("ledger missing: %v", err)
	}
}

func TestCompatSurfaceDryRunPrintsTempDir(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, "docs")
	if err := os.MkdirAll(filepath.Join(docs, "apex"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docs, "apex", "system_object.md"), []byte("# Object Class\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"compat", "surface", "refresh", "--docs", docs, "--dry-run"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "dryRunOut=") {
		t.Fatalf("dry-run path missing: %q", stdout.String())
	}
}
