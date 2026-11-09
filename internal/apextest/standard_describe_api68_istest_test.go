package apextest

import (
	"os"
	"path/filepath"
	"testing"
)

// P0-istest-route/org.tsv records the same three native test-runner results.
func TestStandardDescribeAPI68NativeIsTestRoute(t *testing.T) {
	for _, row := range []struct{ api, version string }{{"62.0", "62"}, {"67.0", "67"}, {"68.0", "68"}} {
		t.Run(row.api, func(t *testing.T) {
			source, err := os.ReadFile(filepath.Join("testdata/conformance/standard_describe_api68_drift/P0-istest-route", "source"+row.version+".cls"))
			if err != nil {
				t.Fatal(err)
			}
			index := standardDescribeDeclaredBoundaryIndex(t, row.api, map[string]string{"V035DescribeRoute" + row.version: string(source)}, nil)
			result := Run(index, Options{Parallelism: 2, ParallelMethods: true, NoDiskCache: true})
			if summary := result.Summary(); summary.Total != 1 || summary.Passed != 1 {
				t.Fatalf("native test-runner assertion did not pass: %+v; suites=%+v", summary, result.Suites)
			}
		})
	}
}
