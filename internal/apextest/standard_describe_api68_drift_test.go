package apextest

import (
	"encoding/csv"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// P0 anonymous field-map membership was captured at all three source APIs in
// testdata/conformance/standard_describe_api68_drift/org.tsv.
func TestStandardDescribeAPI68NativeAnonymousMembership(t *testing.T) {
	f, err := os.Open("testdata/conformance/standard_describe_api68_drift/org.tsv")
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
	if len(rows) != 13 {
		t.Fatalf("native rows = %d, want 12 plus header", len(rows))
	}
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			var source strings.Builder
			var want []string
			for _, row := range rows[1:] {
				if len(row) != 5 {
					t.Fatalf("malformed native row: %v", row)
				}
				if row[0] != api || row[1] != "anonymous" {
					continue
				}
				fmt.Fprintf(&source, "System.debug(Schema.getGlobalDescribe().get('%s').getDescribe().fields.getMap().containsKey('%s'));\n", row[2], row[3])
				want = append(want, row[4])
			}
			if len(want) != 4 {
				t.Fatalf("native rows for %s = %d, want 4", api, len(want))
			}
			index := standardDescribeDeclaredBoundaryIndex(t, api, nil, nil)
			program, err := vm.CompileAnonymousWithOptions(source.String(), vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			result, err := runner.execute(program)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Join(result.Debug, ",") != strings.Join(want, ",") {
				t.Fatalf("native field membership %v; Glade %v", want, result.Debug)
			}
		})
	}
}
