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

func TestPaymentGatewayAdapterNativeConformance(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "payment_gateway_adapter")
	read := func(name string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	f, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 7 {
		t.Fatalf("expected six native rows, got %d", len(rows)-1)
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		if len(row) != 9 {
			t.Fatalf("bad row: %v", row)
		}
		id, api, route := row[0], row[1], row[2]
		key := id + "/" + api
		if seen[key] || (api != "62.0" && api != "67.0") {
			t.Fatalf("unexpected row %v", row)
		}
		seen[key] = true
		t.Run(key, func(t *testing.T) {
			files := map[string]string{"AdapterProbe.cls": read("AdapterProbe.cls"), "AdapterProbeTest.cls": read("AdapterProbeTest.cls")}
			if id == "INTERFACE_CONSTRUCTION" {
				files = nil
			}
			index := nativeBoundaryIndex(t, api, files, nil)
			if id == "DISPATCH_TEST" && route == "istest" {
				analysis := sema.Analyze(index)
				if index.HasErrors() || analysis.HasErrors() {
					t.Fatalf("native compiled; diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
				}
				run := Run(index, Options{SelectedClasses: []string{"AdapterProbeTest"}, SelectedMethod: "dispatch", NoDiskCache: true, Parallelism: 1})
				if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 || row[4] != "Passed" {
					t.Fatalf("native %v; Glade %+v, suites %+v", row, summary, run.Suites)
				}
				return
			}
			if route != "anonymous" || (id != "DISPATCH_ANON" && id != "INTERFACE_CONSTRUCTION") {
				t.Fatalf("unexpected row %v", row)
			}
			source := "dispatch.apex"
			if id == "INTERFACE_CONSTRUCTION" {
				source = "reject.apex"
			}
			prepared := innerShapePrepareAnonymous(t, api, index, read(source))
			var errors []string
			for _, d := range prepared.diagnostics {
				if d.Severity == diagnostic.Error {
					msg := d.NativeMessage
					if msg == "" {
						msg = d.Message
					}
					errors = append(errors, msg)
				}
			}
			if id == "INTERFACE_CONSTRUCTION" {
				if row[3] != "False" || len(errors) == 0 || !strings.Contains(strings.ToLower(strings.Join(errors, " | ")), "construct") {
					t.Fatalf("native %v; Glade errors %v", row, errors)
				}
				return
			}
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() || len(errors) != 0 {
				t.Fatalf("native compiled; diagnostics: %v %v %v", index.Diagnostics, analysis.Diagnostics, errors)
			}
			program, err := vm.CompileAnonymousWithOptions(prepared.body, vm.CompileOptions{APIVersion: api, ApprovedPrefixStatements: sema.ApprovedAnonymousPrefixStatements(prepared.merged, prepared.body, api)})
			if err != nil {
				t.Fatal(err)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			if _, err := runner.execute(program); err != nil || row[3] != "True" || row[4] != "True" {
				t.Fatalf("native %v; Glade error %v", row, err)
			}
		})
	}
}
