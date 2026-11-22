package scripts

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCIWorkerClassification(t *testing.T) {
	for _, tc := range []struct{ name, jobs, want, annotation string }{
		{"all pass", `[{"name":"worker 1","status":"completed","conclusion":"success"}]`, `"passed":["worker 1"]`, ""},
		{"failed", `[{"name":"worker 1","status":"completed","conclusion":"failure"}]`, `"failed":["worker 1"]`, ""},
		{"cancelled", `[{"name":"worker 1","status":"completed","conclusion":"cancelled"}]`, `"cancelled":["worker 1"]`, "::warning::Cancelled CI workers: worker 1"},
		{"mixed", `[{"name":"worker 1","status":"completed","conclusion":"success"},{"name":"worker 2","status":"completed","conclusion":"failure"},{"name":"worker 3","status":"completed","conclusion":"cancelled"}]`, `"cancelled":["worker 3"]`, "::warning::Cancelled CI workers: worker 3"},
		{"skip and running", `[{"name":"skipped","status":"completed","conclusion":"skipped"},{"name":"current","status":"in_progress","conclusion":null}]`, `"skipped":["skipped"]`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			input := filepath.Join(dir, "jobs.json")
			summary := filepath.Join(dir, "summary")
			output := filepath.Join(dir, "output")
			if err := os.WriteFile(input, []byte(`[{"jobs":`+tc.jobs+`}]`), 0600); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("python3", "ci-worker-classification.py", input)
			cmd.Env = append(os.Environ(), "GITHUB_STEP_SUMMARY="+summary, "GITHUB_OUTPUT="+output)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("classification failed: %v: %s", err, out)
			}
			data, err := os.ReadFile(output)
			if err != nil {
				t.Fatal(err)
			}
			value := strings.TrimPrefix(strings.TrimSpace(string(data)), "classification=")
			if !json.Valid([]byte(value)) || !strings.Contains(value, tc.want) {
				t.Fatalf("classification: %s; want %s", data, tc.want)
			}
			if tc.annotation != "" && !strings.Contains(string(out), tc.annotation) {
				t.Fatalf("annotation: %s", out)
			}
			if tc.annotation == "" && strings.Contains(string(out), "::warning::") {
				t.Fatalf("unexpected warning: %s", out)
			}
			summaryData, err := os.ReadFile(summary)
			if err != nil || !strings.Contains(string(summaryData), "CI worker classification") {
				t.Fatalf("summary: %s %v", summaryData, err)
			}
		})
	}
	job := workflowJobBlocks(t, readWorkflowFile(t, "ci.yml"))["required-ci"]
	for _, want := range []string{"actions: read", "ci-worker-classification.py", "steps.classify.outputs.classification", "--paginate"} {
		if !strings.Contains(job, want) {
			t.Errorf("Required CI missing %q", want)
		}
	}
}
