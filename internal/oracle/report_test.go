package oracle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadRunsRejectsMalformedAndEmptyRuns(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]string{
		"empty-array.json": `[]`,
		"empty-run.json":   `{}`,
	}
	for name, body := range cases {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadRuns(path); err == nil {
			t.Fatalf("ReadRuns(%s) succeeded, want error", name)
		}
	}
}

func TestReadRunsRejectsUnsupportedSchemaVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad-version.json")
	if err := os.WriteFile(path, []byte(`{
  "schemaVersion": 99,
  "source": "salesforce",
  "testClass": "AccountOracleTest",
  "status": "pass"
}`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := ReadRuns(path)
	if err == nil || !strings.Contains(err.Error(), "schemaVersion") {
		t.Fatalf("err = %v", err)
	}
}

func TestNewReportIsNotReadyWithoutComparisons(t *testing.T) {
	report := NewReport("fixture", "run", "artifacts", nil, 0, 0, false)
	if report.Ready {
		t.Fatalf("empty report marked ready: %#v", report)
	}
}
