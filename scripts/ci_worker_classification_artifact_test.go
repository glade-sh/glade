package scripts

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCIWorkerClassificationArtifact(t *testing.T) {
	dir := t.TempDir()
	input, artifact := filepath.Join(dir, "input.json"), filepath.Join(dir, "classification.json")
	if err := os.WriteFile(input, []byte(`[{"jobs":[{"name":"a","status":"completed","conclusion":"cancelled"},{"name":"b","status":"completed","conclusion":"failure"}]}]`), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("python3", "ci-worker-classification.py", input)
	cmd.Env = append(os.Environ(), "CI_CLASSIFICATION_PATH="+artifact, "GITHUB_OUTPUT="+filepath.Join(dir, "output"), "GITHUB_STEP_SUMMARY="+filepath.Join(dir, "summary"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("classification: %v %s", err, out)
	}
	data, err := os.ReadFile(artifact)
	if err != nil || !json.Valid(data) || !strings.Contains(string(data), `"cancelled":["a"]`) || !strings.Contains(string(data), `"failed":["b"]`) {
		t.Fatalf("artifact %s: %v", data, err)
	}
	job := workflowJobBlocks(t, readWorkflowFile(t, "ci.yml"))["required-ci"]
	for _, want := range []string{"CI_CLASSIFICATION_PATH:", "name: ci-worker-classification-${{ github.run_id }}-${{ github.run_attempt }}", "if: ${{ always() }}"} {
		if !strings.Contains(job, want) {
			t.Errorf("missing %s", want)
		}
	}
}
