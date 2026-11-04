package scripts

import (
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestCIRaceProvisioningStepsBounded(t *testing.T) {
	var workflow struct {
		Jobs map[string]struct {
			Timeout string `yaml:"timeout-minutes"`
			Steps   []struct {
				Name    string
				Run     string
				If      string
				Timeout int `yaml:"timeout-minutes"`
			}
		}
	}
	if err := yaml.Unmarshal([]byte(readWorkflowFile(t, "race.yml")), &workflow); err != nil {
		t.Fatal(err)
	}
	routes := map[string][]string{
		"plan":         {"npm ci --prefix third_party/lwc"},
		"race-browser": {"npm ci --prefix third_party/lwc", "npm ci --prefix lwcruntime", "scripts/ci-install-playwright.sh ./lwcruntime/node_modules/.bin/playwright install --with-deps chromium"},
		"race-early":   {"npm ci --prefix third_party/lwc"},
		"race":         {"npm ci --prefix third_party/lwc", "npm ci --prefix lwcruntime", "scripts/ci-install-playwright.sh ./lwcruntime/node_modules/.bin/playwright install --with-deps chromium"},
		"race-push":    {"npm ci --prefix third_party/lwc", "npm ci --prefix lwcruntime", "scripts/ci-install-playwright.sh ./lwcruntime/node_modules/.bin/playwright install --with-deps chromium"},
	}
	for job, commands := range routes {
		body := workflow.Jobs[job]
		for _, command := range commands {
			matches := 0
			for _, step := range body.Steps {
				if !strings.Contains(step.Run, command) {
					continue
				}
				matches++
				limit := 5
				if strings.Contains(command, "playwright") {
					limit = 65
				}
				if strings.TrimSpace(step.Run) != command || step.Name == "" || step.Timeout != limit {
					t.Errorf("%s: %q must have its own named %d-minute step: %+v", job, command, limit, step)
				}
				if job == "race-push" && (!strings.Contains(step.If, "./internal/gladecli") || !strings.Contains(step.If, "./internal/server")) {
					t.Errorf("%s: %q lost its route condition", job, command)
				}
			}
			if matches != 1 {
				t.Errorf("%s: %q has %d steps, want 1", job, command, matches)
			}
		}
	}
	for job, minimum := range map[string]int{"race-browser": 145, "race": 145, "race-early": 80} {
		timeout, err := strconv.Atoi(workflow.Jobs[job].Timeout)
		if err != nil || timeout < minimum {
			t.Errorf("%s: job budget = %q, want at least %d minutes for provisioning, test, and cleanup", job, workflow.Jobs[job].Timeout, minimum)
		}
	}
	if got := workflow.Jobs["race-push"].Timeout; got != "${{ matrix.timeoutMinutes }}" {
		t.Errorf("race-push: job budget = %q, want the per-batch planned timeout", got)
	}
}

func TestCIRaceProvisioningRetainsTestAndEvidenceRoutes(t *testing.T) {
	var workflow struct {
		Jobs map[string]struct {
			Steps []struct{ Name, Run, If, Uses string }
		}
	}
	if err := yaml.Unmarshal([]byte(readWorkflowFile(t, "race.yml")), &workflow); err != nil {
		t.Fatal(err)
	}
	for _, route := range []struct{ job, test, command, upload string }{
		{"race-browser", "Run assigned browser race shard", `scripts/ci-race-test.sh "$PACKAGE" "$SLUG"`, "Upload browser race evidence"},
		{"race-early", "Run early race detector", `scripts/ci-race-test.sh "$PACKAGE" "$SLUG"`, "Upload early race evidence"},
		{"race", "Run race detector", `scripts/ci-race-test.sh "$PACKAGE" "$SLUG"`, "Upload race evidence"},
		{"race-push", "Run concurrent Go tests", `python3 scripts/ci-race-push.py run "${{ matrix.index }}"`, "Upload push race evidence"},
	} {
		testIndex, uploadIndex := -1, -1
		for i, step := range workflow.Jobs[route.job].Steps {
			if step.Name == route.test {
				testIndex = i
				if step.Run != route.command {
					t.Errorf("%s: unexpected test command %q", route.job, step.Run)
				}
			}
			if step.Name == route.upload {
				uploadIndex = i
				if step.If != "always()" || !strings.HasPrefix(step.Uses, "actions/upload-artifact@") {
					t.Errorf("%s: evidence upload no longer runs unconditionally", route.job)
				}
			}
		}
		if testIndex < 0 || uploadIndex <= testIndex {
			t.Errorf("%s: test and evidence steps missing or out of order", route.job)
		}
	}
}
