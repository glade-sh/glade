package apextest

import (
	"encoding/csv"
	"encoding/json"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/vm"
	"os"
	"strings"
	"testing"
)

// Native commerce_extension_shapes/org.tsv pins abstract calculators, virtual
// dispatch, and CheckoutPlaceOrder.validate at API62 and API67. Hosted service
// execution is outside these owned callback bodies.
func TestCommerceExtensionDispatchNativeProjectRoutes(t *testing.T) {
	const dir = "../../testdata/conformance/commerce_extension_shapes/"
	raw, err := os.ReadFile(dir + "cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct{ ID, Name, Source string }
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	sources := map[string]struct{ ID, Name, Source string }{}
	for _, c := range cases {
		sources[c.ID] = c
	}
	raw, err = os.ReadFile(dir + "org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(string(raw)))
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, row := range rows[1:] {
		if row[1] != "istest" || (row[2] != "inventory_override" && row[2] != "pricing_override" && row[2] != "shipping_override" && row[2] != "checkout_validate_override") {
			continue
		}
		if row[3] != "Pass" {
			t.Fatalf("native dispatch did not pass: %v", row)
		}
		checked++
		t.Run(row[0]+"/"+row[2], func(t *testing.T) {
			c := sources[row[2]]
			body := "System.assertEquals(true," + c.Name + ".observe());"
			files := map[string]string{c.Name: c.Source, "OwnedCommerceTest": "@IsTest private class OwnedCommerceTest { @IsTest static void sample(){" + body + "} }"}
			index := nativeBoundaryIndex(t, row[0], files, nil)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native compiled: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: row[0]})
			if err != nil {
				t.Fatal(err)
			}
			org := orgFromIndex(index)
			org.APIVersion = row[0]
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			for _, state := range []string{"cold", "warm"} {
				t.Run(state, func(t *testing.T) {
					if _, err := runner.execute(program); err != nil {
						t.Fatal(err)
					}
					run := Run(index, Options{SelectedClasses: []string{"OwnedCommerceTest"}, SelectedMethod: "sample", NoDiskCache: true, Parallelism: 1})
					if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Status != testreport.StatusPass {
						t.Fatalf("native test passed; Glade: %+v", run)
					}
				})
			}
		})
	}
	if checked != 8 {
		t.Fatalf("native dispatch rows=%d, want 8", checked)
	}
}
