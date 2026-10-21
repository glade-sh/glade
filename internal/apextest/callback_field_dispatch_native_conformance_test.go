package apextest

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCallbackFieldDispatchNativeConformance(t *testing.T) {
	const dir = "testdata/conformance/callback_field_dispatch"
	f, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil || len(rows) != 7 {
		t.Fatalf("native rows=%d err=%v", len(rows), err)
	}
	for _, row := range rows[1:] {
		if len(row) != 3 || row[2] != "Pass" {
			t.Fatalf("invalid native runtime row: %v", row)
		}
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
			root := filepath.Join(dir, "api"+strings.TrimSuffix(row[0], ".0"))
			run := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{"CallbackDispatchProbe"}, SelectedMethod: row[1], NoDiskCache: true, Parallelism: 1})
			if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
				t.Fatalf("native passed; Glade summary=%+v suites=%+v", summary, run.Suites)
			}
		})
	}
}
