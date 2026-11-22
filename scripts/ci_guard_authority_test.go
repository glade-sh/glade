package scripts

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCIGuardMacDeferral(t *testing.T) {
	const pkg = "github.com/glade-sh/glade/scripts"
	const test = "TestCILocalReleaseLaneMetricsIncludeApexShards"
	for _, tc := range []struct {
		name, reason, identity, runner string
		ok                             bool
	}{
		{"captured", "local-release resource metrics use macOS /usr/bin/time -l", test, "Linux", true},
		{"different reason", "unrelated", test, "Linux", false},
		{"different test", "local-release resource metrics use macOS /usr/bin/time -l", "TestOther", "Linux", false},
		{"mac runner", "local-release resource metrics use macOS /usr/bin/time -l", test, "macOS", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			events := filepath.Join(dir, "events")
			meta := filepath.Join(dir, "metadata")
			summary := filepath.Join(dir, "summary")
			rows := []map[string]string{{"Action": "output", "Package": pkg, "Test": tc.identity, "Output": "    ci_local_release_lane_test.go:240: " + tc.reason + "\n"}, {"Action": "skip", "Package": pkg, "Test": tc.identity}, {"Action": "pass", "Package": pkg}}
			var text strings.Builder
			for _, row := range rows {
				b, _ := json.Marshal(row)
				text.Write(b)
				text.WriteByte('\n')
			}
			os.WriteFile(events, []byte(text.String()), 0600)
			os.WriteFile(meta, []byte(pkg+"\thas-tests\n"), 0600)
			cmd := exec.Command("bash", "ci-go-test.sh", "package-summary", "remaining-go", "fixture", events, summary, meta, pkg)
			cmd.Env = append(os.Environ(), "RUNNER_OS="+tc.runner)
			out, err := cmd.CombinedOutput()
			if (err == nil) != tc.ok {
				t.Fatalf("err=%v out=%s", err, out)
			}
			if tc.ok {
				b, err := os.ReadFile(summary)
				if err != nil || !strings.Contains(string(b), `"test": "`+test+`"`) {
					t.Fatalf("deferred evidence %s %v", b, err)
				}
			}
		})
	}
}

func TestCIGuardWorkflowUsesAuditedExecution(t *testing.T) {
	_, jobs := readCIWorkflow(t)
	for _, name := range []string{"early-guards", "pr-guards"} {
		if !strings.Contains(jobs[name], "ci-pr-packages.sh") {
			t.Errorf("%s does not audit selected guard terminals", name)
		}
	}
}
