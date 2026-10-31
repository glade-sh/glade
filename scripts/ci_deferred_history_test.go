package scripts

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCIRelease2ApexHistoryRetainsValidatedDeferredOutcomes(t *testing.T) {
	root := t.TempDir()
	output := filepath.Join(root, "history.json")
	packageName := "github.com/glade-sh/glade/internal/apextest"
	names := []string{"TestRunDataWeaveTypedAdmitted", "TestRunExecutesEnumValues"}
	shardNames := [][]string{{names[0]}, {names[1]}}
	planShards := make([]map[string]any, 2)
	dirs := make([]string, 2)
	for index, selectedNames := range shardNames {
		dir := filepath.Join(root, fmt.Sprintf("apex-shard-%d", index))
		dirs[index] = dir
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		selected := map[string]any{
			"index": index, "tests": selectedNames, "estimatedDurationMillis": 0,
			"regex": "^(?:" + selectedNames[0] + ")$",
		}
		planShards[index] = selected
		if err := os.WriteFile(filepath.Join(dir, "discovery.txt"), []byte(strings.Join(names, "\n")+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		writeJSONFixture(t, filepath.Join(dir, "selected-shard.json"), selected)
		passed := []string{names[1]}
		deferred := []string{}
		if index == 0 {
			passed = []string{}
			deferred = names[:1]
		}
		writeJSONFixture(t, filepath.Join(dir, "validation-summary.json"), map[string]any{
			"valid": true, "expected": selectedNames, "passed": passed, "errors": []string{}, "deferred": deferred,
		})
		action := "pass"
		events := ""
		if index == 0 {
			action = "skip"
			events = `{"Action":"output","Package":"` + packageName + `","Test":"` + names[0] + `","Output":"    deferred_test.go:1: requires explicitly installed DataWeave toolchain\n"}` + "\n"
		}
		events += `{"Action":"` + action + `","Package":"` + packageName + `","Test":"` + selectedNames[0] + `","Elapsed":2}` + "\n"
		events += `{"Action":"pass","Package":"` + packageName + `","Elapsed":2}` + "\n"
		if err := os.WriteFile(filepath.Join(dir, "events.json"), []byte(events), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	plan := map[string]any{"version": 1, "package": packageName, "historyUsed": false, "shards": planShards}
	for _, dir := range dirs {
		writeJSONFixture(t, filepath.Join(dir, "plan.json"), plan)
	}
	args := append([]string{"ci-go-test.sh", "apex-history-refresh"}, dirs...)
	args = append(args, output)
	cmd := exec.Command("bash", args...)
	deferredInputs := []string{
		"GLADE_DATAWEAVE_TEST_CLASSPATH", "GLADE_DATAWEAVE_TEST_JAVA_HOME", "GLADE_DATAWEAVE_TEST_ENGINE",
		"GLADE_DATAWEAVE_TEST_INSTALLED_ENGINE", "GLADE_DATAWEAVE_TEST_INSTALL_DIRECTORY", "GLADE_DATAWEAVE_APEX_TEST_HOME",
	}
	for _, variable := range os.Environ() {
		name, _, _ := strings.Cut(variable, "=")
		keep := true
		for _, deferredInput := range deferredInputs {
			if name == deferredInput {
				keep = false
				break
			}
		}
		if keep {
			cmd.Env = append(cmd.Env, variable)
		}
	}
	cmd.Env = append(cmd.Env, "CI_APEXTEST_HISTORY_COMPLETE=1", "GITHUB_EVENT_NAME=pull_request")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("history refresh failed: %v\n%s", err, out)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	var history struct {
		Complete bool `json:"complete"`
		Tests    []struct {
			Name           string `json:"name"`
			DurationMillis int64  `json:"durationMillis"`
		} `json:"tests"`
	}
	if err := json.Unmarshal(data, &history); err != nil {
		t.Fatal(err)
	}
	if !history.Complete || len(history.Tests) != 2 {
		t.Fatalf("history is not a complete two-outcome union: %s", data)
	}
	gotNames := []string{history.Tests[0].Name, history.Tests[1].Name}
	if !reflect.DeepEqual(gotNames, names) || history.Tests[0].DurationMillis != 0 || history.Tests[1].DurationMillis != 2000 {
		t.Fatalf("unexpected deferred/pass scheduling weights: %s", data)
	}
	planner := exec.Command("go", "run", "./internal/cishard", "--package", packageName, "--shards", "2", "--history", output)
	planner.Dir = "."
	planner.Stdin = strings.NewReader(strings.Join(names, "\n") + "\n")
	planData, err := planner.Output()
	if err != nil {
		t.Fatal(err)
	}
	var replanned struct {
		HistoryUsed bool `json:"historyUsed"`
	}
	if err := json.Unmarshal(planData, &replanned); err != nil || !replanned.HistoryUsed {
		t.Fatalf("complete history was not reusable: %s %v", planData, err)
	}
}
