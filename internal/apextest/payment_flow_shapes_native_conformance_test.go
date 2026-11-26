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
	if len(rows) != 83 {
		t.Fatalf("expected 82 observations, got %d", len(rows)-1)
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		if len(row) != 8 || (row[1] != "62.0" && row[1] != "67.0") || (row[4] != "true" && row[4] != "false") {
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
				if index.HasErrors() {
					t.Fatalf("invalid class source: %v", index.Diagnostics)
				}
				if !checkPaymentFlowCompilation(t, row[4] == "true", row[6], analysis.Diagnostics) {
					return
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
			if !checkPaymentFlowCompilation(t, row[4] == "true", row[6], prepared.diagnostics) {
				return
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

func checkPaymentFlowCompilation(t *testing.T, compiled bool, want string, diagnostics []diagnostic.Diagnostic) bool {
	t.Helper()
	for _, d := range diagnostics {
		if d.Severity != diagnostic.Error {
			continue
		}
		if compiled {
			t.Fatalf("native compiled; Glade diagnostic: %v", d)
		}
		if d.NativeMessage == want {
			return false
		}
	}
	if !compiled {
		t.Fatalf("native rejected with %q; Glade diagnostics: %v", want, diagnostics)
	}
	return true
}

func TestPaymentFlowEquivalentClassAPIVersions(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			class := "RoadPay" + strings.TrimSuffix(api, ".0")
			path := filepath.Join("testdata", "conformance", "payment_flow_shapes", class+".cls")
			source, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			index := nativeBoundaryIndexWithClassAPIs(t, api, map[string]string{class: string(source)}, nil, map[string]string{class: api + "0"})
			if index.HasErrors() {
				t.Fatalf("invalid class source: %v", index.Diagnostics)
			}
			analysis := sema.Analyze(index)
			if analysis.HasErrors() {
				t.Fatalf("equivalent class API %s rejected: %v", api+"0", analysis.Diagnostics)
			}
			for _, method := range []string{"sale", "void"} {
				t.Run(method, func(t *testing.T) {
					run := Run(index, Options{SelectedClasses: []string{class}, SelectedMethod: method, NoDiskCache: true, Parallelism: 1})
					if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
						t.Fatalf("Glade %+v, suites %+v", summary, run.Suites)
					}
				})
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

func TestPaymentFlowAnonymousContextRequiresRejectedRequestDeclaration(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, flow := range []struct {
			request, args, kind string
		}{
			{"SaleRequest", "amount", "Sale"},
			{"AuthorizationReversalRequest", "amount, ref", "AuthorizationReversal"},
		} {
			t.Run(api+"/"+flow.kind, func(t *testing.T) {
				const prefix = "Double amount = 7.25; String ref = 'ref'; "
				request := "commercepayments." + flow.request
				constructor := "new " + request + "(" + flow.args + "); "
				declared := prefix + request + " req = " + constructor
				context := "commercepayments.PaymentGatewayContext ctx = new commercepayments.PaymentGatewayContext(req, commercepayments.RequestType." + flow.kind + ");"
				index := nativeBoundaryIndex(t, api, nil, nil)
				result := sema.AnalyzeAnonymous(index, declared+context, api)
				if result.HasErrors() {
					t.Fatalf("captured request declaration rejected: %v", result.Diagnostics)
				}
				for name, body := range map[string]string{
					"assignment":   prefix + request + " req; req = " + constructor + context,
					"local-method": "void probe() { " + declared + context + " } probe();",
					"null":         request + " req = null; " + context,
				} {
					t.Run(name, func(t *testing.T) {
						result := sema.AnalyzeAnonymous(index, body, api)
						want := "Constructor not defined: [commercepayments.PaymentGatewayContext].<Constructor>(" + request + ", commercepayments.RequestType)"
						for _, d := range result.Diagnostics {
							if d.Severity == diagnostic.Error && d.Code == "GLADESEMA028" && d.Message == want {
								return
							}
						}
						t.Fatalf("expected context constructor rejection %q, got %v", want, result.Diagnostics)
					})
				}
			})
		}
	}
}
