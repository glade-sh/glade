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
				Timeout string `yaml:"timeout-minutes"`
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
				if strings.TrimSpace(step.Run) != command || step.Name == "" || strings.TrimSpace(step.Timeout) != strconv.Itoa(limit) {
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
	for job, minimum := range map[string]int{"race-browser": 145, "race": 145} {
		timeout, err := strconv.Atoi(workflow.Jobs[job].Timeout)
		if err != nil || timeout < minimum {
			t.Errorf("%s: job budget = %q, want at least %d minutes for provisioning, test, and cleanup", job, workflow.Jobs[job].Timeout, minimum)
		}
	}
	// The measured lanes read their job budgets from the plan's per-leg
	// matrix; the plan's own tests keep those budgets above the
	// provisioning, test and cleanup floor and under the GitHub job cap.
	for _, job := range []string{"race-early", "race-independent", "race-vm", "race-apextest", "race-push"} {
		if got := workflow.Jobs[job].Timeout; got != "${{ matrix.timeoutMinutes }}" {
			t.Errorf("%s: job budget = %q, want the per-leg planned timeout", job, got)
		}
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
		{"race-independent", "Run independent race detector", `scripts/ci-race-test.sh "$PACKAGE" "$SLUG"`, "Upload independent race evidence"},
		{"race-vm", "Run assigned VM race shard", `scripts/ci-race-test.sh ./internal/vm internal-vm`, "Upload VM race evidence"},
		{"race", "Run race detector", `scripts/ci-race-test.sh "$PACKAGE" "$SLUG"`, "Upload race evidence"},
		{"race-apextest", "Run assigned Apex race shards", `scripts/ci-race-test.sh ./internal/apextest internal-apextest`, "Upload Apex runner evidence"},
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

// TestCIRaceProvisioningRoutesCoverDeclaredCapabilities pins the generic race
// route's declared prerequisites. Scheduled run 44158211531 stopped two routes
// before their intended assertions ran: ./internal/lwc's
// TestStandardFieldPolicyConfiguredLWCSchemaImportRoutes failed with "no glade
// checkout with LWC toolchain found" because the Node compiler it drives
// resolves the toolchain through gladehome root validation, which requires
// third_party/lwc/node_modules/@lwc/compiler; and ./internal/lwcruntime's
// TestBrowserRuntimeSuite reported "could not find glade source checkout", the
// same missing compiler surfaced through compile.FindRepoRoot, while its
// Playwright suite also needs the lwcruntime dependencies and Chromium. Every
// package whose tests consume a capability must appear in that capability's
// route selector, and browser setup must stay off the pure-Go routes.
func TestCIRaceProvisioningRoutesCoverDeclaredCapabilities(t *testing.T) {
	var workflow struct {
		Jobs map[string]struct {
			Steps []struct{ Name, Run, If, Uses string }
		}
	}
	if err := yaml.Unmarshal([]byte(readWorkflowFile(t, "race.yml")), &workflow); err != nil {
		t.Fatal(err)
	}
	race := workflow.Jobs["race"]
	if len(race.Steps) == 0 {
		t.Fatal("race workflow lost its generic job steps")
	}
	routeSelector := func(packages []string) string {
		return `contains(fromJSON('["` + strings.Join(packages, `","`) + `"]'), matrix.package)`
	}
	// The declared LWC compiler toolchain consumers. internal/lwc drives the
	// vendored Node compiler from TestStandardFieldPolicyConfiguredLWCSchemaImportRoutes,
	// and internal/lwcruntime's TestBrowserRuntimeSuite resolves the same
	// compiler through compile.FindRepoRoot before it can run.
	compilerPackages := []string{
		"./internal/gladehome",
		"./internal/lwc",
		"./internal/lwcbrowser",
		"./internal/lwcruntime",
		"./internal/server",
	}
	// The declared browser conformance consumers: Playwright rows and Chromium.
	// Pure-Go routes (storage, vm, soql, dml and the rest of the generic
	// matrix) must stay out of this selector.
	browserPackages := []string{
		"./internal/lwcbrowser",
		"./internal/lwcruntime",
		"./internal/server",
	}
	for _, route := range []struct {
		name     string
		selector string
		steps    []string
	}{
		{
			name:     "LWC compiler toolchain",
			selector: routeSelector(compilerPackages),
			steps: []string{
				"actions/setup-node@249970729cb0ef3589644e2896645e5dc5ba9c38",
				"Resolve npm cache path",
				"Restore LWC npm cache",
				"Install LWC toolchain",
			},
		},
		{
			name:     "browser conformance runtime",
			selector: routeSelector(browserPackages),
			steps: []string{
				"Install browser conformance dependencies",
				"Install conformance Chromium",
			},
		},
	} {
		wantIf := "${{ " + route.selector + " }}"
		for _, identity := range route.steps {
			var found *struct{ Name, Run, If, Uses string }
			for i := range race.Steps {
				if race.Steps[i].Name == identity || race.Steps[i].Uses == identity {
					found = &race.Steps[i]
					break
				}
			}
			if found == nil {
				t.Errorf("%s: race job lost its %q prerequisite step", route.name, identity)
				continue
			}
			if found.If != wantIf {
				t.Errorf("%s: %q must use the exact consumer selector %s, got %q", route.name, identity, wantIf, found.If)
			}
		}
	}
	// Browser routes serve the compiled runtime, so every browser consumer
	// must also select the LWC compiler toolchain route.
	compilerSet := make(map[string]bool, len(compilerPackages))
	for _, consumer := range compilerPackages {
		compilerSet[consumer] = true
	}
	for _, consumer := range browserPackages {
		if !compilerSet[consumer] {
			t.Errorf("browser consumer %s must also select the LWC compiler toolchain route", consumer)
		}
	}
	// The two routes blocked in the scheduled run must stay provisioned.
	if !compilerSet["./internal/lwc"] || !compilerSet["./internal/lwcruntime"] {
		t.Error("the LWC compiler and runtime race routes lost their toolchain provisioning")
	}
	browserSet := make(map[string]bool, len(browserPackages))
	for _, consumer := range browserPackages {
		browserSet[consumer] = true
	}
	if !browserSet["./internal/lwcruntime"] {
		t.Error("the lwcruntime browser race route lost its browser provisioning")
	}
}
