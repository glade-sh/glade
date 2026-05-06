package probe

import (
	"testing"
)

func TestLocalExecutorCaptureLocal(t *testing.T) {
	exec := &LocalExecutor{ProbeDir: "../../probes/sfdx"}
	results, err := exec.CaptureLocal(defaultProbeIDs())
	if err != nil {
		t.Fatalf("capture local failed: %v", err)
	}

	for id, r := range results {
		t.Logf("probe %s => result=%v exception=%v", id, r.Result, r.ExceptionType)
		if r.ProbeID == "" {
			t.Errorf("probe %s returned empty ProbeID", id)
		}
		exc := "<nil>"
		if r.ExceptionType != nil {
			exc = *r.ExceptionType + ": " + coalesce(r.ExceptionMessage)
		}
		t.Logf("  exception detail: %s", exc)
	}

	if len(results) != len(defaultProbeIDs()) {
		t.Errorf("expected %d results, got %d", len(defaultProbeIDs()), len(results))
	}
}
