package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCICacheCheckScriptRunsWorkflowParser(t *testing.T) {
	command := exec.Command("bash", "ci-cache-check.sh")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("ci-cache-check.sh failed: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "structural cache workflow invariants passed") {
		t.Fatalf("unexpected ci-cache-check.sh output:\n%s", output)
	}
}

func TestCICacheCheckScriptReportsAbsentEvidenceWithoutFailing(t *testing.T) {
	command := exec.Command("bash", "ci-cache-check.sh")
	command.Env = append(os.Environ(), "CI_CACHE_EVIDENCE_PATH="+filepath.Join(t.TempDir(), "missing.json"))
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("ci-cache-check.sh failed without evidence: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "no cache evidence supplied") {
		t.Fatalf("missing evidence decision was not reported:\n%s", output)
	}
}

func TestCICacheCheckScriptUsesSuppliedEvidence(t *testing.T) {
	evidencePath := filepath.Join(t.TempDir(), "evidence.json")
	evidence := `{"version":1,"samples":[
{"cache":"go-build","sample":"one","observed_at":"2026-07-29T00:00:00Z","transfer_seconds":11,"avoided_work_seconds":10},
{"cache":"go-build","sample":"two","observed_at":"2026-07-29T00:01:00Z","transfer_seconds":12,"avoided_work_seconds":10},
{"cache":"go-build","sample":"three","observed_at":"2026-07-29T00:02:00Z","transfer_seconds":13,"avoided_work_seconds":10}]}`
	if err := os.WriteFile(evidencePath, []byte(evidence), 0o600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("bash", "ci-cache-check.sh")
	command.Env = append(os.Environ(), "CI_CACHE_EVIDENCE_PATH="+evidencePath)
	output, err := command.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "strict evidence rejected") {
		t.Fatalf("supplied negative evidence was accepted: %v\n%s", err, output)
	}
}

func TestCIWorkflowRunsCacheInvariantCheck(t *testing.T) {
	workflow, err := os.ReadFile(filepath.Join("..", ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	const step = "      - name: Check cache workflow invariants\n        run: scripts/ci-cache-check.sh"
	if strings.Count(string(workflow), step) != 1 {
		t.Fatalf("CI cache invariant step count = %d, want 1", strings.Count(string(workflow), step))
	}
}
