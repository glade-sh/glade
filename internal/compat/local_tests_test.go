package compat

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunLocalTestsClassifiesBasicFixture(t *testing.T) {
	report, err := RunLocalTests(LocalTestOptions{Project: filepath.Join("..", "..", "testdata", "local-tests", "basic")})
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.Total != 3 || report.Summary.Pass != 1 || report.Summary.Fail != 1 || report.Summary.Unsupported != 1 {
		t.Fatalf("summary = %#v", report.Summary)
	}
	if report.Ready {
		t.Fatalf("ready = true, want false")
	}
}

func TestRunLocalTestsReportsLoadError(t *testing.T) {
	root := t.TempDir()
	writeLocalTestFile(t, filepath.Join(root, "sfdx-project.json"), `{`)
	report, err := RunLocalTests(LocalTestOptions{Project: root})
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.LoadErrors != 1 || report.Outcomes[0].Outcome != "load_error" {
		t.Fatalf("report = %#v", report)
	}
}

func writeLocalTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
