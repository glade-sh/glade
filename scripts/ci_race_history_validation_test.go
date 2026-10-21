package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func raceHistoryWorkflowStep(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", ".github", "workflows", "race.yml"))
	if err != nil {
		t.Fatal(err)
	}
	return workflowStepBlockText(string(data), "      - name: "+name)
}

func raceHistoryStepScript(t *testing.T, step string) string {
	t.Helper()
	_, body, ok := strings.Cut(step, "        run: |\n")
	if !ok {
		t.Fatal("missing workflow shell block")
	}
	lines := strings.Split(strings.TrimRight(body, "\n"), "\n")
	for i := range lines {
		lines[i] = strings.TrimPrefix(lines[i], "          ")
	}
	return strings.Join(lines, "\n")
}

func TestCIRaceDownloadedHistoryValidationRoutePropagatesFailure(t *testing.T) {
	step := raceHistoryWorkflowStep(t, "Validate LWC duration history")
	for _, status := range []string{"0", "23"} {
		t.Run("refresh-exit-"+status, func(t *testing.T) {
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o700); err != nil {
				t.Fatal(err)
			}
			refresh := "#!/usr/bin/env bash\nprintf 'validated' >\"$2\"\nexit " + status + "\n"
			if err := os.WriteFile(filepath.Join(root, "scripts", "ci-go-test.sh"), []byte(refresh), 0o700); err != nil {
				t.Fatal(err)
			}
			history := filepath.Join(root, "history.json")
			if err := os.WriteFile(history, []byte("stale"), 0o600); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("bash", "-e", "-o", "pipefail", "-c", raceHistoryStepScript(t, step))
			cmd.Dir = root
			cmd.Env = append(os.Environ(), "CI_LWC_COMPILE_HISTORY_PATH="+history)
			output, err := cmd.CombinedOutput()
			if status == "0" {
				body, readErr := os.ReadFile(history)
				if err != nil || readErr != nil || string(body) != "validated" {
					t.Fatalf("valid history lost: output=%s err=%v body=%q read=%v", output, err, body, readErr)
				}
				return
			}
			if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 23 {
				t.Errorf("invalid downloaded history must fail with refresh status 23: err=%v output=%s", err, output)
			}
			if _, err := os.Stat(history); !os.IsNotExist(err) {
				t.Errorf("invalid downloaded history must remove stale output: %v", err)
			}
		})
	}
}

func TestCIRaceOptionalTimingDownloadFailureRouteStagesFallback(t *testing.T) {
	download := raceHistoryWorkflowStep(t, "Download LWC timing evidence")
	validation := raceHistoryWorkflowStep(t, "Validate LWC duration history")
	if !strings.Contains(download, "id: lwc-timing-download") || !strings.Contains(download, "continue-on-error: true") {
		t.Error("optional timing download must expose its outcome and allow missing artifacts")
	}
	if !strings.Contains(validation, "steps.lwc-timing-download.outcome == 'success'") {
		t.Error("validation must run only after the optional timing download succeeds")
	}
	root := t.TempDir()
	cmd := exec.Command("bash", "-e", "-o", "pipefail", "-c", raceHistoryStepScript(t, raceHistoryWorkflowStep(t, "Stage shared LWC duration history")))
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "CI_LWC_COMPILE_HISTORY_PATH="+filepath.Join(root, "absent.json"))
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("missing optional timing must stage fallback: %v: %s", err, output)
	}
	input := filepath.Join(root, "ci-artifacts", "race-lwc-history-input")
	if _, err := os.Stat(filepath.Join(input, "ready")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(input, "lwc-compile-duration-history.json")); !os.IsNotExist(err) {
		t.Fatalf("fallback must omit timing history: %v", err)
	}
}

func TestCIRaceDownloadedHistoryRouteStagesValidatedInput(t *testing.T) {
	root := t.TempDir()
	history := filepath.Join(root, "history.json")
	const body = "{\"validated\":true}\n"
	if err := os.WriteFile(history, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", "-e", "-o", "pipefail", "-c", raceHistoryStepScript(t, raceHistoryWorkflowStep(t, "Stage shared LWC duration history")))
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "CI_LWC_COMPILE_HISTORY_PATH="+history)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("stage valid timing history: %v: %s", err, output)
	}
	data, err := os.ReadFile(filepath.Join(root, "ci-artifacts", "race-lwc-history-input", "lwc-compile-duration-history.json"))
	if err != nil || string(data) != body {
		t.Fatalf("staged history bytes=%q err=%v", data, err)
	}
}
