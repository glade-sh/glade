package apextest

import (
	"path/filepath"
	"testing"
)

func TestSchemaClassShadowingProject(t *testing.T) {
	index := loadTestIndex(t, filepath.Join("testdata", "schema_class_shadowing"))
	if index.HasErrors() {
		t.Fatalf("synthetic project index: %v", index.Diagnostics)
	}
	if index.Project.Namespace != "sample" {
		t.Fatalf("project namespace = %q", index.Project.Namespace)
	}
	run := Run(index, Options{SelectedClasses: []string{"SchemaNameProbeTest"}, NoDiskCache: true})
	if summary := run.Summary(); summary.Total != 5 || summary.Passed != 5 {
		t.Fatalf("schema/class resolution: summary=%#v problem=%s", summary, firstRunProblem(run))
	}
}
