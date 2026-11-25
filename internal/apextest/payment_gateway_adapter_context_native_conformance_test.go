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

// Native observations: testdata/conformance/payment_gateway_adapter/context/org.tsv.
func TestPaymentGatewayAdapterContextNativeConformance(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "payment_gateway_adapter", "context")
	read := func(name string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	reader := csv.NewReader(strings.NewReader(read("org.tsv")))
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 5 {
		t.Fatalf("expected four native rows, got %d", len(rows)-1)
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		if len(row) != 9 || (row[1] != "62.0" && row[1] != "67.0") {
			t.Fatalf("unexpected native row %v", row)
		}
		key := row[0] + "/" + row[1]
		if seen[key] {
			t.Fatalf("duplicate native row %s", key)
		}
		seen[key] = true
		if row[0] == "CONTEXT_ANON" && row[2] == "anonymous" {
			if row[3] != "True" || row[4] != "False" {
				t.Fatalf("unexpected anonymous construction row %v", row)
			}
			t.Run(key, func(t *testing.T) {
				index := nativeBoundaryIndex(t, row[1], map[string]string{"ContextAdapter": read("ContextAdapter.cls")}, nil)
				prepared := innerShapePrepareAnonymous(t, row[1], index, read("dispatch.apex"))
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
				if _, err = runner.execute(program); err == nil || !strings.Contains(err.Error(), row[5]) {
					t.Fatalf("native %s; Glade: %v", row[5], err)
				}
			})
			continue
		}
		if row[0] != "CONTEXT_TEST" || row[2] != "istest" || row[3] != "true" || row[4] != "Passed" || row[5] != "dispatch=Pass" {
			t.Fatalf("unexpected native row %v", row)
		}
		t.Run(key, func(t *testing.T) {
			index := nativeBoundaryIndex(t, row[1], map[string]string{"ContextAdapter": read("ContextAdapter.cls"), "ContextAdapterTest": read("ContextAdapterTest.cls")}, nil)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native compiled; Glade diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			run := Run(index, Options{SelectedClasses: []string{"ContextAdapterTest"}, SelectedMethod: "dispatch", NoDiskCache: true, Parallelism: 1})
			if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
				t.Fatalf("native %v; Glade %+v, suites %+v", row, summary, run.Suites)
			}
		})
	}
}
