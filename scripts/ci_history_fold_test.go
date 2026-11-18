package scripts

import (
	"os/exec"
	"strings"
	"testing"
)

// The five history-input jobs and six history jobs are one job each. This pins
// that the folded jobs keep the contracts the separate jobs carried: every input
// artifact is still uploaded, and a producer that was skipped, failed or
// cancelled still fails Required CI.
func TestCIFoldedHistoryInputUploadsEveryInputArtifact(t *testing.T) {
	_, jobs := readCIWorkflow(t)
	input := jobs["history-input"]
	for _, name := range []string{"gladecli", "vm", "lwc-compile", "visualforce", "apextest"} {
		if strings.Count(input, "          name: "+name+"-history-input\n") != 1 {
			t.Errorf("history-input must upload %s-history-input exactly once", name)
		}
	}
	for _, retired := range []string{"gladecli", "vm", "lwc-compile", "visualforce", "apextest"} {
		if _, found := jobs[retired+"-history-input"]; found {
			t.Errorf("job %s-history-input was folded into history-input", retired)
		}
	}
	for _, retired := range []string{"gladecli", "vm", "sema", "apextest", "lwc-compile", "visualforce"} {
		if _, found := jobs[retired+"-history"]; found {
			t.Errorf("job %s-history was folded into history", retired)
		}
	}
	seen := map[string]bool{}
	for _, step := range workflowStepBlocksText(input) {
		if id := strings.TrimSpace(workflowStepField(step, "id")); id != "" {
			if seen[id] {
				t.Errorf("history-input repeats step id %q", id)
			}
			seen[id] = true
		}
	}
}

func workflowStepField(step, field string) string {
	for _, line := range strings.Split(step, "\n") {
		trimmed := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "- "))
		if value, found := strings.CutPrefix(trimmed, field+":"); found {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func TestCIFoldedHistoryJobFailsRequiredCIForAnyUnsuccessfulProducer(t *testing.T) {
	_, jobs := readCIWorkflow(t)
	history := jobs["history"]
	producers := []string{"gladecli", "vm", "sema", "apextest", "lwc-vf"}
	if !strings.Contains(history, "needs: [gladecli, vm, sema, apextest, lwc-vf]\n") {
		t.Fatal("history must need all five producers")
	}
	if !strings.Contains(history, "    if: ${{ !cancelled() && ") {
		t.Error("history must run after a failed producer, so a successful producer's timing is still recorded and the failure is reported")
	}
	for _, producer := range producers {
		if !strings.Contains(jobs["required-ci"], "      - "+producer+"\n") {
			t.Errorf("Required CI must keep requiring producer %s", producer)
		}
	}
	if !strings.Contains(jobs["required-ci"], "      - history\n") {
		t.Error("Required CI must require history")
	}
	// Every refresh, save and upload step stays gated on its own producer.
	for _, step := range workflowStepBlocksText(history) {
		name := workflowStepField(step, "name")
		if name == "" || strings.HasPrefix(name, "Require every history producer") || strings.Contains(name, "after balance failure") {
			continue
		}
		if !strings.Contains(step, "needs.") || !strings.Contains(step, ".result == 'success'") {
			t.Errorf("history step %q is not gated on its producer's result", name)
		}
	}
	var require string
	for _, step := range workflowStepBlocksText(history) {
		if strings.Contains(step, "name: Require every history producer") {
			require = step
		}
	}
	if require == "" {
		t.Fatal("history lacks the final producer check")
	}
	for _, producer := range producers {
		if !strings.Contains(require, "needs."+producer+".result != 'success'") {
			t.Errorf("final producer check ignores %s", producer)
		}
	}
	if !strings.Contains(require, "if: ${{ !cancelled() && (") || !strings.Contains(require, "exit 1") {
		t.Error("final producer check must run after failures and exit nonzero")
	}
	// Run the check's script for a skipped producer.
	script := require[strings.Index(require, "        run: |\n")+len("        run: |\n"):]
	var lines []string
	for _, line := range strings.Split(script, "\n") {
		lines = append(lines, strings.TrimPrefix(line, "          "))
	}
	command := exec.Command("bash", "-c", strings.Join(lines, "\n"))
	command.Env = append(command.Environ(), `PRODUCER_RESULTS={"gladecli":{"result":"success"},"lwc-vf":{"result":"skipped"},"apextest":{"result":"failure"}}`)
	out, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("check accepted skipped and failed producers:\n%s", out)
	}
	for _, want := range []string{"lwc-vf=skipped", "apextest=failure"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("check output lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(string(out), "gladecli=success") {
		t.Errorf("check reported a successful producer:\n%s", out)
	}
}
