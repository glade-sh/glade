package scripts

import (
	"strings"
	"testing"
)

func TestCIRaceWeeklyCoverageAndPrerequisites(t *testing.T) {
	jobs := workflowJobBlocks(t, readWorkflowFile(t, "race.yml"))
	big, browser := jobs["weekly-big"], jobs["weekly-browser"]
	for _, marker := range []string{"./internal/apextest", "^Test[A-L]", "^Test[M-Z]", "./internal/vm", "./internal/gladecli"} {
		if !strings.Contains(big, marker) {
			t.Errorf("weekly-big missing %q", marker)
		}
	}
	if strings.Contains(big, "./internal/server") {
		t.Error("server must run with browser prerequisites")
	}
	for _, marker := range []string{
		"./internal/server", "GLADE_ROOT: ${{ github.workspace }}", "GLADE_LWC_BROWSER: \"1\"",
		"GLADE_LWC_PLAYWRIGHT_MODULE:", "uses: actions/setup-node@", "node-version: \"22\"",
		"npm ci --prefix third_party/lwc", "npm ci --prefix lwcruntime",
		"scripts/ci-install-playwright.sh ./lwcruntime/node_modules/.bin/playwright install --with-deps chromium",
	} {
		if !strings.Contains(browser, marker) {
			t.Errorf("weekly-browser missing %q", marker)
		}
	}
}
