package apextest

import (
	"path/filepath"
	"testing"
)

func TestRunFlowInterviewScalarContracts(t *testing.T) {
	root := filepath.Join("testdata", "flow_interview_probe")
	run := Run(loadTestIndex(t, root), Options{
		SelectedClasses: []string{"SproutScalarProof"}, NoDiskCache: true,
	})
	if got := run.Summary(); got.Total != 3 || got.Passed != 3 {
		t.Fatalf("summary = %#v; problem = %s", got, firstRunProblem(run))
	}
}
