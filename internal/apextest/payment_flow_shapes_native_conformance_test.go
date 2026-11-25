package apextest

import (
	"encoding/csv"
	"fmt"
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
	if len(rows) != 59 {
		t.Fatalf("expected 58 observations, got %d", len(rows)-1)
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

// RUN-FACTS.txt records rejected Decimal constructors on both routes and APIs.
func TestPaymentFlowDecimalConstructorNativeConformance(t *testing.T) {
	requests := map[string]string{
		"AuthorizationRequest": "amount", "CaptureRequest": "amount, ref", "SaleRequest": "amount",
		"ReferencedRefundRequest": "amount, ref", "AuthorizationReversalRequest": "amount, ref",
	}
	for _, api := range []string{"62.0", "67.0"} {
		for name, args := range requests {
			body := fmt.Sprintf("Decimal amount=3.5; String ref='ref'; commercepayments.%s req = new commercepayments.%s(%s);", name, name, args)
			for _, route := range []string{"anonymous", "istest"} {
				t.Run(api+"/"+route+"/"+name, func(t *testing.T) {
					var diagnostics []diagnostic.Diagnostic
					if route == "anonymous" {
						index := nativeBoundaryIndex(t, api, nil, nil)
						diagnostics = innerShapePrepareAnonymous(t, api, index, body).diagnostics
					} else {
						source := "@IsTest private class DecimalProbe { @IsTest static void run() { " + body + " } }"
						index := nativeBoundaryIndex(t, api, map[string]string{"DecimalProbe": source}, nil)
						diagnostics = sema.Analyze(index).Diagnostics
					}
					want := "Constructor not defined: [commercepayments." + name + "].<Constructor>(Decimal"
					for _, d := range diagnostics {
						if d.Severity == diagnostic.Error && strings.Contains(d.Message, want) {
							return
						}
					}
					t.Fatalf("missing native constructor rejection %q: %v", want, diagnostics)
				})
			}
		}
	}
}

func TestPaymentFlowResponseAssignmentRejectsProjectShadow(t *testing.T) {
	const body = `commercepayments.SaleResponse response = new commercepayments.SaleResponse();
commercepayments.GatewayResponse gateway = response;`
	const mismatch = "commercepayments.SaleResponse to commercepayments.GatewayResponse"

	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			index := nativeBoundaryIndex(t, api, map[string]string{
				"commercepayments": "public class commercepayments { public class GatewayResponse {} }",
			}, nil)
			if index.HasErrors() {
				t.Fatalf("invalid project fixture: %v", index.Diagnostics)
			}
			t.Run("sema", func(t *testing.T) {
				result := sema.AnalyzeAnonymous(index, body, api)
				for _, d := range result.Diagnostics {
					if d.Severity == diagnostic.Error && d.Code == "GLADESEMA018" && strings.EqualFold(d.NativeMessage, "Illegal assignment from "+mismatch) {
						return
					}
				}
				t.Fatalf("expected illegal assignment to the project type; got %v", result.Diagnostics)
			})
			t.Run("vm", func(t *testing.T) {
				program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				org.APIVersion = api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				if _, err = runner.execute(program); err == nil || !strings.Contains(strings.ToLower(err.Error()), strings.ToLower("cannot assign "+mismatch)) {
					t.Fatalf("expected assignment failure to the project type; got %v", err)
				}
			})
		})
	}
}

func TestPaymentFlowContextRejectsProjectRequestInterface(t *testing.T) {
	const body = `Double amount = 7.25;
commercepayments.SaleRequest req = new commercepayments.SaleRequest(amount);
commercepayments.PaymentGatewayContext ctx = new commercepayments.PaymentGatewayContext(req, commercepayments.RequestType.Sale);`
	const mismatch = "Constructor not defined: [commercepayments.PaymentGatewayContext].<Constructor>(commercepayments.SaleRequest, commercepayments.RequestType)"

	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			index := nativeBoundaryIndex(t, api, map[string]string{
				"commercepayments": "public class commercepayments { public interface PaymentGatewayRequest {} }",
				"Probe":            "@IsTest private class Probe { @IsTest static void run() { " + body + " } }",
			}, nil)
			if index.HasErrors() {
				t.Fatalf("invalid project fixture: %v", index.Diagnostics)
			}
			for route, analysis := range map[string]sema.Result{
				"anonymous": sema.AnalyzeAnonymous(index, body, api),
				"istest":    sema.Analyze(index),
			} {
				t.Run(route, func(t *testing.T) {
					for _, d := range analysis.Diagnostics {
						if d.Severity == diagnostic.Error && d.Code == "GLADESEMA028" && strings.EqualFold(d.Message, mismatch) {
							return
						}
					}
					t.Fatalf("expected constructor to reject the project interface; got %v", analysis.Diagnostics)
				})
			}
		})
	}
}
