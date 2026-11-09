package apextest

import (
	"encoding/csv"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// The P0 anonymous global-map results are captured in P0-new-objects/org.tsv.
func TestStandardDescribeAPI68NativeNewObjects(t *testing.T) {
	f, err := os.Open("testdata/conformance/standard_describe_api68_drift/P0-new-objects/org.tsv")
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
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			seen := map[string]bool{}
			var source strings.Builder
			var want []string
			for _, row := range rows[1:] {
				if len(row) != 9 {
					t.Fatalf("malformed native row: %v", row)
				}
				if row[0] != api || row[1] != "default" || row[2] != "anonymous" || seen[row[3]] {
					continue
				}
				seen[row[3]] = true
				fmt.Fprintf(&source, "System.debug(Schema.getGlobalDescribe().containsKey('%s'));\n", row[3])
				want = append(want, row[5])
			}
			if len(want) != 17 {
				t.Fatalf("native objects for %s = %d, want 17", api, len(want))
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
				t.Fatalf("native object membership %v; Glade %v", want, result.Debug)
			}
		})
	}
}
