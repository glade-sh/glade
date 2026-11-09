package apextest

import (
	"path/filepath"
	"testing"
)

func TestDescribeDefaultValueNativeConformance(t *testing.T) {
	index := loadTestIndex(t, filepath.Join("testdata", "conformance", "describe_default_value", "project"))
	for _, class := range []string{"DefaultProbeTest62", "DefaultProbeTest67"} {
		t.Run(class, func(t *testing.T) {
			run := Run(index, Options{SelectedClasses: []string{class}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
			if summary := run.Summary(); summary.Total != 1 || summary.Failed != 0 {
				t.Fatalf("native test passed; Glade: %+v, suites: %+v", summary, run.Suites)
			}
		})
	}
}
