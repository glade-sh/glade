package apextest

import (
	"encoding/csv"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// Named field-map observations are in standard_describe_api68_drift/P0-expanded/org.tsv.
func TestStandardDescribeAPI68NativeNamedMembership(t *testing.T) {
	f, err := os.Open("testdata/conformance/standard_describe_api68_drift/P0-expanded/org.tsv")
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
	wanted := map[string]bool{
		"AssetStatePeriod.RampUpliftType":   true,
		"Conversation.ActiveModalities":     true,
		"CronJobDetail.CronJobType":         true,
		"WorkloadUnit.IntradayActualVolume": true,
	}
	const classSource = `public class V035NativeNamed { public static Boolean hasField(String objectName, String fieldName) { return Schema.getGlobalDescribe().get(objectName).getDescribe().fields.getMap().containsKey(fieldName); } }`
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			var source strings.Builder
			var want []string
			for _, row := range rows[1:] {
				if len(row) != 10 {
					t.Fatalf("malformed native row: %v", row)
				}
				if row[0] != api || row[1] != "granted" || row[2] != "named" || !wanted[row[3]+"."+row[4]] {
					continue
				}
				fmt.Fprintf(&source, "System.debug(V035NativeNamed.hasField('%s','%s'));\n", row[3], row[4])
				want = append(want, row[6])
			}
			if len(want) != len(wanted) {
				t.Fatalf("native rows for %s = %d, want %d", api, len(want), len(wanted))
			}
			index := standardDescribeDeclaredBoundaryIndex(t, api, map[string]string{"V035NativeNamed": classSource}, nil)
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
