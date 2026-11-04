package scripts

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A VM shard summary lists deferred selected top-level tests. A captured
// native-boundary subtest skip stays inside its passing parent, so the
// refresh must not demand it among the shard's top-level deferrals.
func TestCIVMHistoryRefreshAcceptsNativeBoundarySubtestsInsidePassingParent(t *testing.T) {
	root := t.TempDir()
	output := filepath.Join(root, "history.json")
	packageName := "github.com/glade-sh/glade/internal/vm"
	dataWeave := "TestExecDataWeaveRecipeConversionsReturnStructuredValues"
	boundary := "TestSOQLSafeNavigationBindNativeConformance"
	others := []string{"TestVMHistoryFixtureA", "TestVMHistoryFixtureB", "TestVMHistoryFixtureC"}
	shardNames := [][]string{{dataWeave, boundary}, {others[0]}, {others[1]}, {others[2]}}
	discovery := append([]string{dataWeave, boundary}, others...)
	planShards := make([]map[string]any, len(shardNames))
	dirs := make([]string, len(shardNames))
	for index, selectedNames := range shardNames {
		dir := filepath.Join(root, fmt.Sprintf("vm-shard-%d", index))
		dirs[index] = dir
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		selected := map[string]any{
			"index": index, "tests": selectedNames, "estimatedDurationMillis": 0,
			"regex": "^(?:" + strings.Join(selectedNames, "|") + ")$",
		}
		planShards[index] = selected
		if err := os.WriteFile(filepath.Join(dir, "discovery.txt"), []byte(strings.Join(discovery, "\n")+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		writeJSONFixture(t, filepath.Join(dir, "selected-shard.json"), selected)
		passed := selectedNames
		deferred := []string{}
		var events strings.Builder
		event := func(action, test, extra string) {
			fmt.Fprintf(&events, `{"Action":%q,"Package":%q,"Test":%q%s}`+"\n", action, packageName, test, extra)
		}
		if index == 0 {
			passed = []string{boundary}
			deferred = []string{dataWeave}
			event("run", dataWeave, "")
			event("output", dataWeave, `,"Output":"    stdlib_test.go:1: requires explicitly installed DataWeave toolchain\n"`)
			event("skip", dataWeave, `,"Elapsed":0`)
			event("run", boundary, "")
			for _, api := range []string{"62.0", "67.0"} {
				leaf := boundary + "/" + api + "/left_member"
				event("run", leaf, "")
				event("output", leaf, `,"Output":"    soql_safe_navigation_bind_conformance_test.go:55: native boundary is captured but outside this bind replay\n"`)
				event("skip", leaf, `,"Elapsed":0`)
			}
			event("pass", boundary, `,"Elapsed":2`)
		} else {
			event("run", selectedNames[0], "")
			event("pass", selectedNames[0], `,"Elapsed":2`)
		}
		fmt.Fprintf(&events, `{"Action":"pass","Package":%q,"Elapsed":2}`+"\n", packageName)
		writeJSONFixture(t, filepath.Join(dir, "validation-summary.json"), map[string]any{
			"valid": true, "expected": selectedNames, "passed": passed, "errors": []string{}, "deferred": deferred,
		})
		if err := os.WriteFile(filepath.Join(dir, "events.json"), []byte(events.String()), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	plan := map[string]any{"version": 1, "package": packageName, "historyUsed": false, "shards": planShards}
	for _, dir := range dirs {
		writeJSONFixture(t, filepath.Join(dir, "plan.json"), plan)
	}
	cmd := exec.Command("bash", append([]string{"ci-go-test.sh", "vm-history-refresh", output}, dirs...)...)
	for _, variable := range os.Environ() {
		if name, _, _ := strings.Cut(variable, "="); !strings.HasPrefix(name, "GLADE_DATAWEAVE_") {
			cmd.Env = append(cmd.Env, variable)
		}
	}
	cmd.Env = append(cmd.Env, "CI_VM_HISTORY_COMPLETE=1", "GITHUB_EVENT_NAME=pull_request")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("VM history refresh failed: %v\n%s", err, out)
	}
	if _, err := os.Stat(output); err != nil {
		t.Fatal(err)
	}
}
