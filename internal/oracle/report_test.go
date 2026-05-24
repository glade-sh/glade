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

func TestCheckCorpusDiffsRelativeFixtureRuns(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "salesforce.json"), []byte(`{
  "schemaVersion": 1,
  "source": "salesforce",
  "project": "fixture",
  "testClass": "AccountOracleTest",
  "testMethod": "createsRecord",
  "status": "pass",
  "events": [{"type": "soql", "sequence": 1, "query": "SELECT Id FROM Account"}]
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "local.json"), []byte(`{
  "schemaVersion": 1,
  "source": "glade",
  "project": "fixture",
  "testClass": "AccountOracleTest",
  "testMethod": "createsRecord",
  "status": "pass",
  "events": [{"type": "soql", "sequence": 1, "query": "SELECT Id FROM Account"}]
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	baseline := filepath.Join(dir, "oracle-corpus.json")
	if err := os.WriteFile(baseline, []byte(`{
  "target": "oracle parity corpus",
  "cases": [
    {
      "name": "passing-fixture",
      "project": "fixture",
      "salesforceRun": "salesforce.json",
      "localRun": "local.json",
      "ready": true,
      "summary": {"total": 1, "pass": 1},
      "outcomes": [
        {"class": "AccountOracleTest", "method": "createsRecord", "outcome": "pass"}
      ]
    }
  ]
}`), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := CheckCorpus(baseline)
	if err != nil {
		t.Fatalf("CheckCorpus error = %v, report = %#v", err, report)
	}
	if !report.Ready || len(report.Cases) != 1 || report.Cases[0].Summary.Pass != 1 {
		t.Fatalf("report = %#v", report)
	}
}
