package scripts

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// The remaining-go matrix row used to run the repository guards (about
// fifteen single-threaded minutes) before the remaining packages (about the
// same). The step now starts the guard lane in the background, runs the
// remaining packages, waits for the guard, prints its log and fails if either
// lane failed. Exercise the extracted step script against a stub wrapper.
func TestCIRemainingGoRowRunsGuardsAlongsidePackages(t *testing.T) {
	_, jobs := readCIWorkflow(t)
	step := workflowStepBlockText(jobs["test"], "      - name: Run repository guards and remaining Go packages")
	script := workflowRunScriptText(step)
	if script == "" {
		t.Fatal("remaining-go row has no combined guard and package step")
	}
	for _, want := range []string{
		"scripts/ci-resource-run.sh ci-artifacts/go-test/resource-repoguard.json repoguard -- scripts/ci-go-test.sh lane repoguard",
		"scripts/ci-resource-run.sh ci-artifacts/go-test/resource-remaining-go.json remaining-go -- scripts/ci-go-test.sh lane remaining-go",
	} {
		if strings.Count(script, want) != 1 {
			t.Fatalf("step script lane command %q count = %d, want 1", want, strings.Count(script, want))
		}
	}

	stub := `#!/usr/bin/env bash
set -euo pipefail
if [[ "$#" -lt 4 || "$3" != "--" ]]; then
  echo "stub wrapper: bad arguments: $*" >&2
  exit 99
fi
lane="$2"
shift 3
printf '%s start %s\n' "$lane" "$*" >>"$STUB_LOG"
# Wait (bounded) until the other lane has started so overlap is observable.
other=remaining-go
if [[ "$lane" == remaining-go ]]; then other=repoguard; fi
for _ in $(seq 1 400); do
  if grep -q "^$other start" "$STUB_LOG" 2>/dev/null; then break; fi
  sleep 0.025
done
echo "stub $lane output"
printf '%s end\n' "$lane" >>"$STUB_LOG"
status_var="STUB_RC_${lane//-/_}"
exit "${!status_var:-0}"
`
	for _, tc := range []struct {
		name        string
		guardRC     int
		remainingRC int
		want        int
	}{
		{name: "both pass", want: 0},
		{name: "guard failure propagates", guardRC: 3, want: 3},
		{name: "remaining failure wins over guard failure", guardRC: 3, remainingRC: 5, want: 5},
		{name: "remaining failure alone", remainingRC: 7, want: 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "scripts", "ci-resource-run.sh"), []byte(stub), 0o700); err != nil {
				t.Fatal(err)
			}
			log := filepath.Join(root, "stub.log")
			cmd := exec.Command("bash", "--noprofile", "--norc", "-e", "-o", "pipefail", "-c", script)
			cmd.Dir = root
			cmd.Env = append(os.Environ(),
				"STUB_LOG="+log,
				"STUB_RC_repoguard="+strconv.Itoa(tc.guardRC),
				"STUB_RC_remaining_go="+strconv.Itoa(tc.remainingRC),
			)
			out, err := cmd.CombinedOutput()
			got := 0
			if err != nil {
				var exitErr *exec.ExitError
				if !errors.As(err, &exitErr) {
					t.Fatalf("step script failed to run: %v\n%s", err, out)
				}
				got = exitErr.ExitCode()
			}
			if got != tc.want {
				t.Fatalf("step exit = %d, want %d\n%s", got, tc.want, out)
			}
			for _, want := range []string{"stub repoguard output", "stub remaining-go output", "::group::repository guard lane (exit " + strconv.Itoa(tc.guardRC) + ")"} {
				if !strings.Contains(string(out), want) {
					t.Errorf("step output missing %q:\n%s", want, out)
				}
			}
			logData, err := os.ReadFile(log)
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(strings.TrimSpace(string(logData)), "\n")
			if len(lines) != 4 {
				t.Fatalf("stub log lines = %d, want both lanes to start and end:\n%s", len(lines), logData)
			}
			if !strings.HasPrefix(lines[2], "repoguard end") && !strings.HasPrefix(lines[2], "remaining-go end") ||
				!strings.HasSuffix(lines[0], "scripts/ci-go-test.sh lane repoguard") && !strings.HasSuffix(lines[1], "scripts/ci-go-test.sh lane repoguard") ||
				!strings.HasSuffix(lines[0], "scripts/ci-go-test.sh lane remaining-go") && !strings.HasSuffix(lines[1], "scripts/ci-go-test.sh lane remaining-go") {
				t.Fatalf("lanes did not overlap or ran unexpected commands:\n%s", logData)
			}
		})
	}
}
