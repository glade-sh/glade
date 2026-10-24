package apextest

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
)

// Native capture error-order-inner (deploy of OrdA at API 62.0 and
// 67.0, identical): errors inside an inner type are reported before the outer
// class's own method errors, each scope in source order. The @IsTest route
// deploys the same OrdA beside a test class that uses it; the runner must
// report the compile failure and the analysis keeps the captured OrdA order.
func TestInnerErrorOrderNativeConformance(t *testing.T) {
	raw, err := os.ReadFile("../sema/testdata/inner_error_order/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		ID                   string
		Classes, TestClasses map[string]string
		TestClass            string
	}
	if err := json.Unmarshal(raw, &cases); err != nil || len(cases) != 1 {
		t.Fatalf("native cases: %v %d", err, len(cases))
	}
	c := cases[0]
	raw, err = os.ReadFile("../sema/testdata/inner_error_order/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(string(raw)))
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 3 || strings.Join(rows[0], ",") != "id,api,route,compiled,outcome" {
		t.Fatalf("incomplete native table: %v", err)
	}
	for _, row := range rows[1:] {
		if row[0] != c.ID || row[2] != "named" || row[3] != "false" {
			t.Fatalf("invalid native row %v", row)
		}
		for _, route := range []string{"named", "isTest"} {
			t.Run(row[1]+"/"+route, func(t *testing.T) {
				sources := c.Classes
				if route == "isTest" {
					sources = c.TestClasses
				}
				index := nativeBoundaryIndex(t, row[1], sources, nil)
				var problems []string
				for _, d := range append(append([]diagnostic.Diagnostic{}, index.Diagnostics...), sema.Analyze(index).Diagnostics...) {
					if d.Severity != diagnostic.Error {
						continue
					}
					message := d.NativeMessage
					if message == "" {
						message = d.Message
					}
					problems = append(problems, message)
				}
				if got := strings.Join(problems, " | "); got != row[4] {
					t.Errorf("native %q\nglade  %q", row[4], got)
				}
				if route == "isTest" {
					run := Run(index, Options{SelectedClasses: []string{c.TestClass}, NoDiskCache: true, Parallelism: 1})
					if summary := run.Summary(); summary.Total != 1 || summary.CompileErrors != 1 {
						t.Errorf("rejected deployment ran: summary=%+v", summary)
					}
				}
			})
		}
	}
}
