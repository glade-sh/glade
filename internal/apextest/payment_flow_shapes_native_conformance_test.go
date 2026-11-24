package apextest

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// Native observations: testdata/conformance/payment_flow_shapes/org.tsv.
func TestPaymentFlowShapesNativeConformance(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "payment_flow_shapes")
	read := func(name string) string {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	reader := csv.NewReader(strings.NewReader(read("org.tsv")))
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 31 {
		t.Fatalf("expected 30 observations, got %d", len(rows)-1)
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		if len(row) != 8 || (row[1] != "62.0" && row[1] != "67.0") || row[4] != "true" {
			t.Fatalf("invalid row: %v", row)
		}
		key := strings.Join(row[:4], "/")
		if seen[key] {
			t.Fatalf("duplicate row: %s", key)
		}
		seen[key] = true
		t.Run(key, func(t *testing.T) {
			if row[2] == "istest" {
				class := strings.TrimSuffix(row[7], ".cls")
				index := nativeBoundaryIndex(t, row[1], map[string]string{class: read(row[7])}, nil)
				analysis := sema.Analyze(index)
				if index.HasErrors() || analysis.HasErrors() {
					t.Fatalf("native compiled; Glade diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
				}
				run := Run(index, Options{SelectedClasses: []string{class}, SelectedMethod: row[0], NoDiskCache: true, Parallelism: 1})
				if summary := run.Summary(); row[5] != "Pass" || summary.Total != 1 || summary.Passed != 1 {
					t.Fatalf("native %v; Glade %+v, suites %+v", row, summary, run.Suites)
				}
				return
			}
			if row[2] != "anonymous" {
				t.Fatalf("invalid route: %v", row)
			}
			index := nativeBoundaryIndex(t, row[1], nil, nil)
			prepared := innerShapePrepareAnonymous(t, row[1], index, read(row[7]))
			for _, d := range prepared.diagnostics {
				if d.Severity == diagnostic.Error {
					t.Fatalf("native compiled; Glade diagnostic: %v", d)
				}
			}
			program, err := vm.CompileAnonymousWithOptions(prepared.body, vm.CompileOptions{APIVersion: row[1], ApprovedPrefixStatements: sema.ApprovedAnonymousPrefixStatements(prepared.merged, prepared.body, row[1])})
			if err != nil {
				t.Fatal(err)
			}
			org := orgFromIndex(index)
			org.APIVersion = row[1]
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			_, err = runner.execute(program)
			if row[5] == "Pass" && err != nil {
				t.Fatalf("native passed; Glade: %v", err)
			}
			if row[5] == "Exception" && (err == nil || !strings.Contains(err.Error(), row[6])) {
				t.Fatalf("native %s; Glade: %v", row[6], err)
			}
		})
	}
}
