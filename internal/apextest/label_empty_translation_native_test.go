package apextest

import (
	"path/filepath"
	"testing"
)

func TestRunEmptyLabelTranslationNativeProject(t *testing.T) {
	root := filepath.Join("testdata", "conformance", "label_translation_empty", "project")
	run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
	if summary := run.Summary(); summary.Total != 4 || summary.Passed != 4 {
		t.Fatalf("summary = %#v, problem = %s", summary, firstRunProblem(run))
	}
}
