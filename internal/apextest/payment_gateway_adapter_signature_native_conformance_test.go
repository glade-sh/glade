package apextest

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
)

// NATIVE missing method testdata/conformance/payment_gateway_adapter/signature_rejections/org.tsv:2 istest 62.0
// NATIVE missing method testdata/conformance/payment_gateway_adapter/signature_rejections/org.tsv:3 istest 67.0
// NATIVE wrong parameter testdata/conformance/payment_gateway_adapter/signature_rejections/org.tsv:4 istest 62.0
// NATIVE wrong parameter testdata/conformance/payment_gateway_adapter/signature_rejections/org.tsv:5 istest 67.0
func TestPaymentGatewayAdapterSignatureNativeConformance(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "payment_gateway_adapter", "signature_rejections")
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
	if len(rows) != 5 {
		t.Fatalf("expected four native rows, got %d", len(rows)-1)
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		if len(row) != 9 || row[2] != "istest" || row[3] != "false" || row[4] != "false" || (row[1] != "62.0" && row[1] != "67.0") {
			t.Fatalf("unexpected native row %v", row)
		}
		class := ""
		switch row[0] {
		case "MISSING_METHOD":
			class = "MissingAdapterTest"
		case "WRONG_PARAMETER":
			class = "WrongAdapterTest"
		default:
			t.Fatalf("unexpected native case %v", row)
		}
		key := row[0] + "/" + row[1]
		if seen[key] {
			t.Fatalf("duplicate native row %s", key)
		}
		seen[key] = true
		t.Run(key, func(t *testing.T) {
			index := nativeBoundaryIndex(t, row[1], map[string]string{class: read(class + ".cls"), "AdapterSupport": read("AdapterSupport.cls")}, nil)
			analysis := sema.Analyze(index)
			var messages []string
			for _, d := range analysis.Diagnostics {
				if d.Severity == diagnostic.Error {
					msg := d.NativeMessage
					if msg == "" {
						msg = d.Message
					}
					messages = append(messages, msg)
				}
			}
			want := strings.TrimPrefix(row[5], class+": ")
			if len(messages) != 1 || messages[0] != want {
				t.Fatalf("native %q; Glade %v", want, messages)
			}
			run := Run(index, Options{SelectedClasses: []string{class}, SelectedMethod: "verify", NoDiskCache: true, Parallelism: 1})
			if summary := run.Summary(); summary.Total != 1 || summary.CompileErrors != 1 {
				t.Fatalf("native rejects deployment; Glade %+v, suites %+v", summary, run.Suites)
			}
		})
	}
}
