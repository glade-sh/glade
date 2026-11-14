package apextest

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

func TestAsyncStopNeighborsNativeConformance(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "async_stop_neighbors")
	f, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 34 {
		t.Fatalf("native rows: %d, %v", len(rows), err)
	}
	index := loadTestIndex(t, filepath.Join(dir, "project"))
	analysis := sema.Analyze(index)
	if index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("source diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
	}
	for _, row := range rows[1:] {
		if len(row) != 6 || row[2] != "istest" || row[3] != "true" || row[4] != "Failed" {
			t.Fatalf("invalid native row: %v", row)
		}
		t.Run(row[0], func(t *testing.T) {
			run := Run(index, Options{SelectedClasses: []string{row[0]}, SelectedMethod: "observe", RuntimeRESTAPIVersion: "68.0", NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
				t.Fatalf("expected one case: %+v", run.Summary())
			}
			result := run.Suites[0].Cases[0]
			if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
				t.Fatalf("missing observation assertion: %+v", result.Problem)
			}
			got := "observe=Fail " + result.Problem.Type + ": " + result.Problem.Message
			if got != row[5] {
				t.Fatalf("native=%s\nGlade =%s", row[5], got)
			}
		})
	}
}
