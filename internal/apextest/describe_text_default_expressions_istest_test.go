package apextest

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDescribeTextDefaultExpressionsIsTestNative(t *testing.T) {
	const fixture = "testdata/conformance/describe_text_default_expressions"
	f, err := os.Open(filepath.Join(fixture, "org.tsv"))
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
	index := loadTestIndex(t, filepath.Join(fixture, "project"))
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			anonymous := make(map[string]string)
			native := make(map[string]string)
			for _, row := range rows[1:] {
				if len(row) != 4 || row[0] != api {
					continue
				}
				if row[1] == "anonymous" {
					anonymous[row[2]] = row[3]
				}
				if row[1] == "istest" {
					native[row[2]] = row[3]
				}
			}
			if len(native) != 9 || len(anonymous) != 9 {
				t.Fatalf("incomplete native rows: anonymous=%d istest=%d", len(anonymous), len(native))
			}
			for name, want := range anonymous {
				if native[name] != want {
					t.Fatalf("%s route observations differ", name)
				}
			}
			class := "TextDefaultMatrixTest" + strings.TrimSuffix(api, ".0")
			opts := Options{SelectedClasses: []string{class}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1, PerfCounters: true}
			for pass := 0; pass < 2; pass++ {
				if pass == 1 {
					ResetPerfCounters()
				}
				result := Run(index, opts)
				if pass == 1 {
					if phases := SnapshotPerfCounters().Phases; phases.MemoryCacheHits != 1 || phases.CacheMisses != 0 {
						t.Fatalf("warm project cache: %+v", phases)
					}
				}
				if summary := result.Summary(); summary.Total != 1 || summary.Passed != 1 || summary.Failed != 0 || summary.Errors != 0 {
					for _, suite := range result.Suites {
						for _, c := range suite.Cases {
							if c.Problem != nil {
								t.Logf("%s: %+v", c.MethodName, *c.Problem)
							}
						}
					}
					t.Fatalf("native test passed; Glade: %+v", summary)
				}
			}
		})
	}
}
