package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStaticAccessorNamesNative(t *testing.T) {
	const fixture = "testdata/conformance/static_accessor_names"
	rows, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(rows), "\n") != 7 {
		t.Fatal("expected six native observations")
	}
	run := Run(loadTestIndex(t, filepath.Join(fixture, "project")), Options{NoDiskCache: true, Parallelism: 1})
	for _, suite := range run.Suites {
		for _, result := range suite.Cases {
			if result.Status != "pass" {
				t.Errorf("%s.%s: %+v", result.ClassName, result.MethodName, result.Problem)
			}
		}
	}
	if summary := run.Summary(); summary.Total != 6 || summary.Passed != 6 {
		t.Fatalf("summary=%#v, problem=%s", summary, firstRunProblem(run))
	}
}
