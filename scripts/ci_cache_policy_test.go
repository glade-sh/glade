package scripts

import (
	"strings"
	"testing"
)

// Pull request runs restore the Go caches that pushes to main publish; they
// never write them. Each PR run used to save about twenty-five caches of up to
// 340 MB under run-unique keys, which exceeded the repository's 10 GB cache
// budget within two runs and evicted the duration histories that balance the
// Apex, LWC and Visualforce shards. Saving is reserved for non-PR events and,
// in matrix jobs, for one cell whose content every cell can restore.
func TestCIGoCachesAreReadOnlyOnPullRequests(t *testing.T) {
	_, jobs := readCIWorkflow(t)
	const (
		save      = "actions/cache/save@0057852bfaa89a56745cba8c7296529d2fc39830 # v4.3.0"
		restore   = "actions/cache/restore@0057852bfaa89a56745cba8c7296529d2fc39830 # v4.3.0"
		combined  = "actions/cache@caa296126883cff596d87d8935842f9db880ef25 # v5.1.0"
		notPR     = "github.event_name != 'pull_request'"
		sharedKey = "-ci-test-${{ hashFiles('go.sum') }}-"
	)
	writers := map[string]string{
		"vet": "", "node-integration": "", "server-and-playground": "", "test-binaries": "",
		"smoke-runtime": "", "smoke-distribution": "",
		"gladecli": "matrix.shard == 0", "vm": "matrix.shard == 0",
		"test": "matrix.lane == 'remaining-go'",
	}
	for jobName, cell := range writers {
		job := jobs[jobName]
		saves := 0
		for _, step := range workflowStepBlocksText(job) {
			if !strings.Contains(step, save) {
				continue
			}
			saves++
			condition := ""
			for _, line := range strings.Split(step, "\n") {
				if trimmed := strings.TrimSpace(line); strings.HasPrefix(trimmed, "if: ") {
					condition = strings.TrimPrefix(trimmed, "if: ")
				}
			}
			if !strings.HasPrefix(condition, "success()") || !strings.Contains(condition, notPR) {
				t.Errorf("%s cache save condition %q must require success and a non-PR event", jobName, condition)
			}
			if cell != "" && !strings.Contains(condition, cell) {
				t.Errorf("%s cache save condition %q must select the single writer cell %q", jobName, condition, cell)
			}
		}
		if saves != 2 {
			t.Errorf("%s Go cache saves = %d, want module and build", jobName, saves)
		}
	}
	// Prebuilt-binary consumers read the remaining-go lane's caches from main.
	for _, jobName := range []string{"apextest", "sema"} {
		job := jobs[jobName]
		if strings.Contains(job, save) || strings.Contains(job, combined) {
			t.Errorf("%s must not write Go caches", jobName)
		}
		for _, step := range workflowStepBlocksText(job) {
			if !strings.Contains(step, restore) || !strings.Contains(step, "go-") {
				continue
			}
			if !strings.Contains(step, sharedKey) {
				t.Errorf("%s Go cache restore does not fall back to the shared ci-test cache:\n%s", jobName, step)
			}
			if !strings.Contains(step, "path: ~/go/pkg/mod") && !strings.Contains(step, "path: ~/.cache/go-build") {
				t.Errorf("%s Go cache restore must target the default Go cache paths the shared cache was saved from:\n%s", jobName, step)
			}
		}
	}
	for name, job := range jobs {
		if strings.Contains(job, combined) && !strings.Contains(job, "benchmark npm cache") {
			t.Errorf("%s uses the combined cache action, which saves on pull requests", name)
		}
	}
}
