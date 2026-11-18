package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStaticGetterShapesNative(t *testing.T) {
	const fixture = "testdata/conformance/static_getter_shapes"
	rows, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(rows), "\n"); n != 13 {
		t.Fatalf("expected twelve native rows, got %d", n-1)
	}
	root := filepath.Join(fixture, "project")
	run := Run(loadTestIndex(t, root), Options{NoDiskCache: true, Parallelism: 1})
	if summary := run.Summary(); summary.Total != 12 || summary.Passed != 12 {
		t.Fatalf("summary=%#v, problem=%s", summary, firstRunProblem(run))
	}
}
