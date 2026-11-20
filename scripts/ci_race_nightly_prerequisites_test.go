package scripts

import (
	"strings"
	"testing"
)

// The nightly race jobs must install what the packages they run require.
// Evidence comes from each package's own test helpers on a bare checkout (no
// npm install, no Playwright): internal/lwc drives the vendored LWC compiler
// through node and its standard-field-policy provenance routes fail the
// compile without third_party/lwc/node_modules; internal/lwcbrowser serves
// Playwright suites that also need the lwcruntime npm dependencies and
// Chromium; internal/lwcruntime runs the lwcruntime npm test suite under
// GLADE_LWC_BROWSER=1; the regex-matched internal/server concurrency tests
// serve compiled Lightning modules, which skip without the vendored LWC
// compiler. The regex-matched gladecli tests synthesize their own toolchain
// layout and no regex-matched test drives Playwright, so concurrent-tests
// needs Node and the LWC toolchain but neither the lwcruntime dependencies
// nor Chromium.
func TestCIRaceNightlyJobsInstallPackagePrerequisites(t *testing.T) {
	jobs := workflowJobBlocks(t, readWorkflowFile(t, "race.yml"))
	small := jobs["small-packages"]
	browser := jobs["browser-packages"]
	concurrent := jobs["concurrent-tests"]
	if small == "" || browser == "" || concurrent == "" {
		t.Fatal("race.yml must keep the small-packages, browser-packages and concurrent-tests nightly jobs")
	}

	// The browser packages cannot ride the bare small-packages halves: their
	// tests would skip or fail there, so the halves must exclude them.
	if !strings.Contains(small, "lwc|lwcbrowser|lwcruntime(/embed)?") {
		t.Error("small-packages must exclude the lwc, lwcbrowser and lwcruntime packages that need Node, the LWC toolchain and Chromium")
	}

	// browser-packages carries those packages with the full prerequisite set.
	for _, marker := range []string{
		"GLADE_LWC_BROWSER: \"1\"",
		"uses: actions/setup-node@",
		"npm ci --prefix third_party/lwc",
		"npm ci --prefix lwcruntime",
		"scripts/ci-install-playwright.sh ./lwcruntime/node_modules/.bin/playwright install --with-deps chromium",
	} {
		if !strings.Contains(browser, marker) {
			t.Errorf("browser-packages lacks %q", marker)
		}
	}
	runStep := workflowStepBlock(t, browser, "go test -race")
	for _, pkg := range []string{"./internal/lwc", "./internal/lwcbrowser", "./internal/lwcruntime"} {
		if !strings.Contains(runStep, pkg) {
			t.Errorf("browser-packages run step missing %s", pkg)
		}
	}

	// concurrent-tests regex-runs internal/server, whose matched tests serve
	// compiled Lightning modules; without the vendored LWC toolchain they
	// silently skip every night.
	for _, marker := range []string{
		"uses: actions/setup-node@",
		"npm ci --prefix third_party/lwc",
	} {
		if !strings.Contains(concurrent, marker) {
			t.Errorf("concurrent-tests lacks %q for the regex-matched server Lightning compile routes", marker)
		}
	}
	serverStep := workflowStepBlock(t, concurrent, "./internal/server")
	if !strings.Contains(serverStep, "go test -race") {
		t.Error("concurrent-tests must keep running the internal/server concurrency selection")
	}
}
