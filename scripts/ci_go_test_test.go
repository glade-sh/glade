package scripts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

type apexHistoryFixture struct {
	root   string
	shards [2]string
	output string
}

func realGoCommand(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func writeJSONFixture(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func newApexHistoryFixture(t *testing.T, shardElapsed [2]float64) apexHistoryFixture {
	t.Helper()
	fixture := apexHistoryFixture{root: t.TempDir()}
	fixture.output = filepath.Join(fixture.root, "history", "apextest-duration-history.json")
	names := make([]string, 279)
	for i := range names {
		names[i] = fmt.Sprintf("TestHistory%03d", i)
	}
	shardTests := [2][]string{names[:140], names[140:]}
	shards := make([]map[string]any, 2)
	for index := range shards {
		regex := "^(?:" + strings.Join(shardTests[index], "|") + ")$"
		shards[index] = map[string]any{
			"index":                   index,
			"tests":                   shardTests[index],
			"estimatedDurationMillis": 0,
			"regex":                   regex,
		}
	}
	plan := map[string]any{
		"version": 1, "package": fixturePackage, "historyUsed": false, "shards": shards,
	}
	for index := 0; index < 2; index++ {
		dir := filepath.Join(fixture.root, fmt.Sprintf("apex-shard-%d", index))
		fixture.shards[index] = dir
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "discovery.txt"), []byte(strings.Join(names, "\n")+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		writeJSONFixture(t, filepath.Join(dir, "plan.json"), plan)
		writeJSONFixture(t, filepath.Join(dir, "selected-shard.json"), shards[index])
		writeJSONFixture(t, filepath.Join(dir, "validation-summary.json"), map[string]any{
			"valid": true, "expected": shardTests[index], "passed": shardTests[index], "errors": []string{},
		})
		var events strings.Builder
		for testIndex, name := range shardTests[index] {
			elapsed := 0.0
			if testIndex == 0 {
				elapsed = shardElapsed[index]
			}
			fmt.Fprintf(&events, "{\"Action\":\"pass\",\"Package\":%q,\"Test\":%q,\"Elapsed\":%g}\n", fixturePackage, name, elapsed)
		}
		if err := os.WriteFile(filepath.Join(dir, "events.json"), []byte(events.String()), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return fixture
}

func runApexHistoryRefresh(t *testing.T, fixture apexHistoryFixture) (string, error) {
	t.Helper()
	cmd := exec.Command("bash", "ci-go-test.sh", "apex-history-refresh", fixture.shards[0], fixture.shards[1], fixture.output)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestCIApexDurationHistoryRefresh(t *testing.T) {
	fixture := newApexHistoryFixture(t, [2]float64{1, 1})
	out, err := runApexHistoryRefresh(t, fixture)
	if err != nil {
		t.Fatalf("refresh failed: %v\n%s", err, out)
	}
	data, err := os.ReadFile(fixture.output)
	if err != nil {
		t.Fatal(err)
	}
	var history struct {
		Version  int    `json:"version"`
		Package  string `json:"package"`
		Complete bool   `json:"complete"`
		Tests    []struct {
			Name           string `json:"name"`
			DurationMillis int64  `json:"durationMillis"`
		} `json:"tests"`
	}
	if err := json.Unmarshal(data, &history); err != nil {
		t.Fatal(err)
	}
	if history.Version != 1 || history.Package != fixturePackage || !history.Complete || len(history.Tests) != 279 {
		t.Fatalf("history header/count = %+v / %d", history, len(history.Tests))
	}
	for i, item := range history.Tests {
		if want := fmt.Sprintf("TestHistory%03d", i); item.Name != want {
			t.Fatalf("history test %d = %q, want %q", i, item.Name, want)
		}
	}
	if history.Tests[0].DurationMillis != 1000 || history.Tests[140].DurationMillis != 1000 {
		t.Fatalf("history durations did not come from shard events: first=%d second=%d", history.Tests[0].DurationMillis, history.Tests[140].DurationMillis)
	}
}

func TestCIApexDurationHistoryProducerCanonicalizesDiscovery(t *testing.T) {
	fixture := newApexHistoryFixture(t, [2]float64{1, 1})
	names := make([]string, 279)
	for i := range names {
		names[i] = fmt.Sprintf("TestHistory%03d", 278-i)
	}
	rawPath := filepath.Join(fixture.root, "go-test-list.txt")
	canonicalPath := filepath.Join(fixture.root, "canonical-discovery.txt")
	raw := strings.Join(names, "\n") + "\nok  \t" + fixturePackage + "\n"
	if err := os.WriteFile(rawPath, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", "-c", `source ./ci-go-test.sh; validate_discovery "$1" "$2"`, "producer-fixture", rawPath, canonicalPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("discovery producer failed: %v\n%s", err, out)
	}
	canonical, err := os.ReadFile(canonicalPath)
	if err != nil {
		t.Fatal(err)
	}
	var expected strings.Builder
	for i := 0; i < 279; i++ {
		fmt.Fprintf(&expected, "TestHistory%03d\n", i)
	}
	if string(canonical) != expected.String() {
		t.Fatalf("producer discovery is not canonical; first lines: %q", strings.Join(strings.Split(string(canonical), "\n")[:3], "\n"))
	}
	for _, dir := range fixture.shards {
		if err := os.WriteFile(filepath.Join(dir, "discovery.txt"), canonical, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	out, err := runApexHistoryRefresh(t, fixture)
	if err != nil {
		t.Fatalf("refresh rejected producer-canonical discovery: %v\n%s", err, out)
	}
}

func TestCIApexDurationHistoryRejectsInvalidEvidence(t *testing.T) {
	mutateBothPlans := func(t *testing.T, fixture apexHistoryFixture, old, replacement string) {
		t.Helper()
		for _, dir := range fixture.shards {
			path := filepath.Join(dir, "plan.json")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			mutated := strings.Replace(string(data), old, replacement, 1)
			if mutated == string(data) {
				t.Fatalf("plan mutation did not replace %q", old)
			}
			if err := os.WriteFile(path, []byte(mutated), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	setPlanField := func(t *testing.T, fixture apexHistoryFixture, mutate func(map[string]any)) {
		t.Helper()
		for _, dir := range fixture.shards {
			path := filepath.Join(dir, "plan.json")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var plan map[string]any
			if err := json.Unmarshal(data, &plan); err != nil {
				t.Fatal(err)
			}
			mutate(plan)
			writeJSONFixture(t, path, plan)
		}
	}
	cases := map[string]func(*testing.T, apexHistoryFixture){
		"duplicate terminal": func(t *testing.T, fixture apexHistoryFixture) {
			path := filepath.Join(fixture.shards[0], "events.json")
			data, _ := os.ReadFile(path)
			data = append(data, []byte(fmt.Sprintf("{\"Action\":\"pass\",\"Package\":%q,\"Test\":\"TestHistory000\",\"Elapsed\":0}\n", fixturePackage))...)
			_ = os.WriteFile(path, data, 0o600)
		},
		"missing terminal": func(t *testing.T, fixture apexHistoryFixture) {
			path := filepath.Join(fixture.shards[0], "events.json")
			data, _ := os.ReadFile(path)
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			_ = os.WriteFile(path, []byte(strings.Join(lines[1:], "\n")+"\n"), 0o600)
		},
		"failed terminal": func(t *testing.T, fixture apexHistoryFixture) {
			path := filepath.Join(fixture.shards[0], "events.json")
			data, _ := os.ReadFile(path)
			_ = os.WriteFile(path, []byte(strings.Replace(string(data), `"Action":"pass"`, `"Action":"fail"`, 1)), 0o600)
		},
		"skipped terminal": func(t *testing.T, fixture apexHistoryFixture) {
			path := filepath.Join(fixture.shards[0], "events.json")
			data, _ := os.ReadFile(path)
			_ = os.WriteFile(path, []byte(strings.Replace(string(data), `"Action":"pass"`, `"Action":"skip"`, 1)), 0o600)
		},
		"wrong package": func(t *testing.T, fixture apexHistoryFixture) {
			path := filepath.Join(fixture.shards[0], "events.json")
			data, _ := os.ReadFile(path)
			_ = os.WriteFile(path, []byte(strings.Replace(string(data), fixturePackage, "example.invalid/apextest", 1)), 0o600)
		},
		"negative duration": func(t *testing.T, fixture apexHistoryFixture) {
			path := filepath.Join(fixture.shards[0], "events.json")
			data, _ := os.ReadFile(path)
			_ = os.WriteFile(path, []byte(strings.Replace(string(data), `"Elapsed":1`, `"Elapsed":-1`, 1)), 0o600)
		},
		"nonfinite duration": func(t *testing.T, fixture apexHistoryFixture) {
			path := filepath.Join(fixture.shards[0], "events.json")
			data, _ := os.ReadFile(path)
			_ = os.WriteFile(path, []byte(strings.Replace(string(data), `"Elapsed":1`, `"Elapsed":NaN`, 1)), 0o600)
		},
		"plan mismatch": func(t *testing.T, fixture apexHistoryFixture) {
			path := filepath.Join(fixture.shards[1], "plan.json")
			data, _ := os.ReadFile(path)
			_ = os.WriteFile(path, []byte(strings.Replace(string(data), `"historyUsed": false`, `"historyUsed": true`, 1)), 0o600)
		},
		"discovery mismatch": func(t *testing.T, fixture apexHistoryFixture) {
			path := filepath.Join(fixture.shards[1], "discovery.txt")
			data, _ := os.ReadFile(path)
			_ = os.WriteFile(path, []byte(strings.Replace(string(data), "TestHistory278", "TestHistory999", 1)), 0o600)
		},
		"swapped noncanonical discovery": func(t *testing.T, fixture apexHistoryFixture) {
			for _, dir := range fixture.shards {
				path := filepath.Join(dir, "discovery.txt")
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				mutated := strings.Replace(string(data), "TestHistory000\nTestHistory001", "TestHistory001\nTestHistory000", 1)
				if err := os.WriteFile(path, []byte(mutated), 0o600); err != nil {
					t.Fatal(err)
				}
			}
		},
		"invalid historyUsed type": func(t *testing.T, fixture apexHistoryFixture) {
			mutateBothPlans(t, fixture, `"historyUsed": false`, `"historyUsed": "false"`)
		},
		"empty regex": func(t *testing.T, fixture apexHistoryFixture) {
			setPlanField(t, fixture, func(plan map[string]any) {
				plan["shards"].([]any)[0].(map[string]any)["regex"] = ""
			})
		},
		"noncanonical regex": func(t *testing.T, fixture apexHistoryFixture) {
			mutateBothPlans(t, fixture, `TestHistory000|TestHistory001`, `TestHistory001|TestHistory000`)
		},
		"string estimate": func(t *testing.T, fixture apexHistoryFixture) {
			mutateBothPlans(t, fixture, `"estimatedDurationMillis": 0`, `"estimatedDurationMillis": "0"`)
		},
		"extra plan field": func(t *testing.T, fixture apexHistoryFixture) {
			mutateBothPlans(t, fixture, `"historyUsed": false,`, `"historyUsed": false, "unexpected": true,`)
		},
		"invalid summary": func(t *testing.T, fixture apexHistoryFixture) {
			writeJSONFixture(t, filepath.Join(fixture.shards[0], "validation-summary.json"), map[string]any{"valid": false})
		},
		"extra summary field": func(t *testing.T, fixture apexHistoryFixture) {
			path := filepath.Join(fixture.shards[0], "validation-summary.json")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			mutated := strings.Replace(string(data), `"valid": true`, `"valid": true, "unexpected": true`, 1)
			if err := os.WriteFile(path, []byte(mutated), 0o600); err != nil {
				t.Fatal(err)
			}
		},
		"selected union duplicate": func(t *testing.T, fixture apexHistoryFixture) {
			path := filepath.Join(fixture.shards[1], "selected-shard.json")
			data, _ := os.ReadFile(path)
			_ = os.WriteFile(path, []byte(strings.Replace(string(data), "TestHistory140", "TestHistory000", 1)), 0o600)
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			fixture := newApexHistoryFixture(t, [2]float64{1, 1})
			mutate(t, fixture)
			out, err := runApexHistoryRefresh(t, fixture)
			if err == nil {
				t.Fatalf("invalid evidence was accepted\n%s", out)
			}
			if _, statErr := os.Stat(fixture.output); !os.IsNotExist(statErr) {
				t.Fatalf("refresh published history on failure: %v", statErr)
			}
		})
	}
}

func TestCIApexDurationHistoryRejectsAggregateOverflow(t *testing.T) {
	// Every individual duration is below MaxInt64 milliseconds, while 279 of
	// them overflow the schema-v1 aggregate. Equal per-test values keep the two
	// shard elapsed totals balanced, isolating the overflow guard.
	fixture := newApexHistoryFixture(t, [2]float64{})
	for _, dir := range fixture.shards {
		selectedData, err := os.ReadFile(filepath.Join(dir, "selected-shard.json"))
		if err != nil {
			t.Fatal(err)
		}
		var selected struct {
			Tests []string `json:"tests"`
		}
		if err := json.Unmarshal(selectedData, &selected); err != nil {
			t.Fatal(err)
		}
		var events strings.Builder
		for _, name := range selected.Tests {
			fmt.Fprintf(&events, "{\"Action\":\"pass\",\"Package\":%q,\"Test\":%q,\"Elapsed\":40000000000000}\n", fixturePackage, name)
		}
		if err := os.WriteFile(filepath.Join(dir, "events.json"), []byte(events.String()), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	out, err := runApexHistoryRefresh(t, fixture)
	if err == nil {
		t.Fatalf("aggregate duration overflow was accepted\n%s", out)
	}
	if _, statErr := os.Stat(fixture.output); !os.IsNotExist(statErr) {
		t.Fatalf("overflow refresh published history: %v", statErr)
	}
}

func TestCIApexDurationHistoryAcceptsExactAggregateLimit(t *testing.T) {
	fixture := newApexHistoryFixture(t, [2]float64{})
	const maxInt64 = int64(^uint64(0) >> 1)
	// Keep the large seconds value exactly representable as float64, calculate
	// the generator's millisecond rounding, then fill the small remainder with
	// a second event in each shard.
	seconds := maxInt64/2000 - 10
	baseMillis := int64(math.Floor(float64(seconds)*1000 + 0.5))
	remainder := maxInt64 - 2*baseMillis
	if remainder < 0 || remainder > 100000 {
		t.Fatalf("unexpected exact-boundary fixture remainder %d", remainder)
	}
	extra := [2]int64{remainder / 2, remainder - remainder/2}
	for index, dir := range fixture.shards {
		selectedData, err := os.ReadFile(filepath.Join(dir, "selected-shard.json"))
		if err != nil {
			t.Fatal(err)
		}
		var selected struct {
			Tests []string `json:"tests"`
		}
		if err := json.Unmarshal(selectedData, &selected); err != nil {
			t.Fatal(err)
		}
		var events strings.Builder
		for testIndex, name := range selected.Tests {
			elapsed := "0"
			if testIndex == 0 {
				elapsed = strconv.FormatInt(seconds, 10)
			} else if testIndex == 1 {
				elapsed = strconv.FormatFloat(float64(extra[index])/1000, 'f', 3, 64)
			}
			fmt.Fprintf(&events, "{\"Action\":\"pass\",\"Package\":%q,\"Test\":%q,\"Elapsed\":%s}\n", fixturePackage, name, elapsed)
		}
		if err := os.WriteFile(filepath.Join(dir, "events.json"), []byte(events.String()), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	out, err := runApexHistoryRefresh(t, fixture)
	if err != nil {
		t.Fatalf("exact MaxInt64 aggregate was rejected: %v\n%s", err, out)
	}
	data, err := os.ReadFile(fixture.output)
	if err != nil {
		t.Fatal(err)
	}
	var history struct {
		Tests []struct {
			DurationMillis int64 `json:"durationMillis"`
		} `json:"tests"`
	}
	if err := json.Unmarshal(data, &history); err != nil {
		t.Fatal(err)
	}
	var total uint64
	for _, test := range history.Tests {
		total += uint64(test.DurationMillis)
	}
	if total != uint64(maxInt64) {
		t.Fatalf("history duration total = %d, want %d", total, uint64(maxInt64))
	}
}

func TestCIApexDurationHistoryImbalanceGuard(t *testing.T) {
	for _, tc := range []struct {
		name    string
		elapsed [2]float64
		wantErr bool
	}{
		{name: "balanced", elapsed: [2]float64{1, 1}},
		{name: "exact 1.5 median boundary", elapsed: [2]float64{3, 1}},
		{name: "above 1.5 median", elapsed: [2]float64{3.001, 1}, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newApexHistoryFixture(t, tc.elapsed)
			out, err := runApexHistoryRefresh(t, fixture)
			if (err != nil) != tc.wantErr {
				t.Fatalf("refresh error = %v, wantErr %v\n%s", err, tc.wantErr, out)
			}
		})
	}
}

func TestCIApexDurationHistoryWorkflowOwnership(t *testing.T) {
	scriptData, err := os.ReadFile("ci-go-test.sh")
	if err != nil {
		t.Fatal(err)
	}
	script := string(scriptData)
	for _, want := range []string{
		`if [[ -s "${CI_APEXTEST_HISTORY_PATH:-}" ]]`,
		`planner+=(--history "${CI_APEXTEST_HISTORY_PATH}")`,
	} {
		if !strings.Contains(script, want) {
			t.Errorf("planner history input wiring missing %q", want)
		}
	}

	workflowPath := filepath.Join("..", ".github", "workflows", "ci.yml")
	data, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(data)
	jobs := workflowJobBlocks(t, workflow)
	matrix := jobs["apextest"]
	refresh := jobs["apextest-history"]
	for _, want := range []string{
		"actions/cache/restore@v4", "apextest-duration-history-v1", "runner.os", "runner.arch", "1.26.5", "hashFiles('go.sum')",
		"CI_APEXTEST_HISTORY_PATH", "github.sha", "github.run_id", "github.run_attempt",
	} {
		if !strings.Contains(matrix, want) {
			t.Errorf("matrix history restore missing %q", want)
		}
	}
	if strings.Contains(matrix, "actions/cache/save") {
		t.Error("matrix job must be read-only for duration history")
	}
	for _, want := range []string{
		"needs: apextest", "if: ${{ success() }}", "actions/download-artifact@v7", "apex-shard-*", "scripts/ci-go-test.sh apex-history-refresh",
		"actions/cache/save@v4", "actions/upload-artifact@v6", "apextest-duration-history-v1", "github.sha", "github.run_id", "github.run_attempt",
	} {
		if !strings.Contains(refresh, want) {
			t.Errorf("single-writer refresh job missing %q", want)
		}
	}
	if strings.Count(workflow, "actions/cache/save@v4") != 3 {
		t.Errorf("cache save count = %d, want existing two plus one history writer", strings.Count(workflow, "actions/cache/save@v4"))
	}
	if strings.Count(workflow, "shard: [0, 1]") != 1 {
		t.Error("history work changed the two-native-shard topology")
	}
	if matches, _ := filepath.Glob(filepath.Join("..", "**", "*duration-history*.json")); len(matches) != 0 {
		t.Fatalf("tracked duration history candidates: %v", matches)
	}
}

func TestCIGoCacheReportsMissingDirectoriesAsZero(t *testing.T) {
	readWorkflow := func(name string) string {
		t.Helper()
		workflowPath := filepath.Join("..", ".github", "workflows", name)
		workflow, err := os.ReadFile(workflowPath)
		if err != nil {
			t.Fatalf("read %s: %v", workflowPath, err)
		}
		return string(workflow)
	}
	extractReportScripts := func(workflow string) []string {
		lines := strings.Split(workflow, "\n")
		var scripts []string
		for i, line := range lines {
			if !strings.Contains(line, "- name: Report Go ") {
				continue
			}
			for j := i + 1; j < len(lines); j++ {
				if strings.TrimSpace(lines[j]) != "run: |" {
					continue
				}
				runIndent := len(lines[j]) - len(strings.TrimLeft(lines[j], " "))
				var script strings.Builder
				for k := j + 1; k < len(lines); k++ {
					indent := len(lines[k]) - len(strings.TrimLeft(lines[k], " "))
					if strings.TrimSpace(lines[k]) != "" && indent <= runIndent {
						break
					}
					script.WriteString(strings.TrimPrefix(lines[k], strings.Repeat(" ", runIndent+2)))
					script.WriteByte('\n')
				}
				scripts = append(scripts, script.String())
				break
			}
		}
		return scripts
	}

	for _, workflow := range []struct {
		name string
		text string
		want int
	}{
		{name: "ci.yml", text: readWorkflow("ci.yml"), want: 2},
		{name: "security.yml", text: readWorkflow("security.yml"), want: 6},
	} {
		scripts := extractReportScripts(workflow.text)
		if len(scripts) != workflow.want {
			t.Fatalf("%s report scripts = %d, want %d", workflow.name, len(scripts), workflow.want)
		}
		for i, script := range scripts {
			cmd := exec.Command("bash", "--noprofile", "--norc", "-e", "-o", "pipefail", "-c", script)
			cmd.Env = []string{
				"HOME=" + t.TempDir(),
				"MATCHED_KEY=",
				"PATH=" + os.Getenv("PATH"),
			}
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("%s report script %d failed for missing cache directory: %v\n%s", workflow.name, i+1, err, out)
			}
			if !strings.Contains(string(out), "bytes=0") {
				t.Fatalf("%s report script %d output missing bytes=0:\n%s", workflow.name, i+1, out)
			}
		}
	}
}

func TestCIGoCacheOwnership(t *testing.T) {
	readWorkflow := func(name string) string {
		t.Helper()
		workflowPath := filepath.Join("..", ".github", "workflows", name)
		workflow, err := os.ReadFile(workflowPath)
		if err != nil {
			t.Fatalf("read %s: %v", workflowPath, err)
		}
		return string(workflow)
	}

	ciWorkflow := readWorkflow("ci.yml")
	ci := workflowJobBlocks(t, ciWorkflow)["test"]
	security := readWorkflow("security.yml")

	if got := strings.Count(ci, "actions/setup-go@v6"); got != 1 {
		t.Fatalf("ci.yml setup-go uses = %d, want 1", got)
	}
	if got := strings.Count(ci, "cache: false"); got != 1 {
		t.Fatalf("ci.yml setup-go cache opt-outs = %d, want 1", got)
	}
	if got := strings.Count(security, "actions/setup-go@v6"); got != 3 {
		t.Fatalf("security.yml setup-go uses = %d, want 3", got)
	}
	if got := strings.Count(security, "cache: false"); got != 3 {
		t.Fatalf("security.yml setup-go cache opt-outs = %d, want 3", got)
	}

	if got := strings.Count(ci, "actions/cache/restore@v4"); got != 2 {
		t.Fatalf("ci.yml cache restores = %d, want 2", got)
	}
	if got := strings.Count(ci, "actions/cache/save@v4"); got != 2 {
		t.Fatalf("ci.yml cache saves = %d, want 2", got)
	}
	if got := strings.Count(security, "actions/cache/restore@v4"); got != 6 {
		t.Fatalf("security.yml cache restores = %d, want 6", got)
	}
	if got := strings.Count(security, "actions/cache/save@v4"); got != 0 {
		t.Fatalf("security.yml cache saves = %d, want 0", got)
	}
	if got := strings.Count(ci, "continue-on-error: true"); got != 4 {
		t.Fatalf("ci.yml non-fatal cache actions = %d, want 4", got)
	}
	if got := strings.Count(security, "continue-on-error: true"); got != 6 {
		t.Fatalf("security.yml non-fatal cache actions = %d, want 6", got)
	}

	for _, workflow := range []struct {
		name string
		text string
	}{
		{name: "ci.yml", text: ci},
		{name: "security.yml", text: security},
	} {
		for _, want := range []string{"path: ~/go/pkg/mod", "path: ~/.cache/go-build", "go-mod-cache-v1", "go-build-cache-v1"} {
			if !strings.Contains(workflow.text, want) {
				t.Errorf("%s missing separate cache marker %q", workflow.name, want)
			}
		}
		for _, line := range strings.Split(workflow.text, "\n") {
			if strings.TrimSpace(line) == "path: |" {
				t.Errorf("%s contains a combined multi-path cache", workflow.name)
			}
		}
	}

	fullKeys := func(workflow string) []string {
		var keys []string
		for _, line := range strings.Split(workflow, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "key: ") && strings.Contains(line, "github.sha") {
				keys = append(keys, strings.TrimPrefix(line, "key: "))
			}
		}
		return keys
	}
	assertFullKeys := func(name, workflow, sumExpression string, want int) {
		t.Helper()
		keys := fullKeys(workflow)
		if len(keys) != want {
			t.Errorf("%s full cache keys = %d, want %d", name, len(keys), want)
		}
		for _, key := range keys {
			for _, dimension := range []string{
				"cache-v1", "1.26.5", "runner.os", "runner.arch", sumExpression,
				"github.sha", "github.run_id", "github.run_attempt",
			} {
				if !strings.Contains(key, dimension) {
					t.Errorf("%s full key %q missing dimension %q", name, key, dimension)
				}
			}
			if !strings.Contains(key, "go-mod-") && !strings.Contains(key, "go-build-") {
				t.Errorf("%s full key %q has no cache-class namespace", name, key)
			}
			if !strings.Contains(key, "ci-test") && !strings.Contains(key, "security-") {
				t.Errorf("%s full key %q has no lane namespace", name, key)
			}
		}
	}
	assertFullKeys("ci.yml", ci, "hashFiles('glade/go.sum')", 2)
	assertFullKeys("security.yml", security, "hashFiles('go.sum')", 6)

	for _, want := range []string{
		"key: ${{ steps.restore-go-mod-cache.outputs.cache-primary-key }}",
		"key: ${{ steps.restore-go-build-cache.outputs.cache-primary-key }}",
	} {
		if !strings.Contains(ci, want) {
			t.Errorf("ci.yml save missing matching restore primary key %q", want)
		}
	}
	for _, lane := range []string{"security-govulncheck", "security-codeql", "security-gosec"} {
		if got := strings.Count(security, lane); got != 2 {
			t.Errorf("security.yml lane %q occurrences = %d, want 2 full keys", lane, got)
		}
	}
	if got := strings.Count(security, "ci-test-${{ hashFiles('go.sum') }}-"); got != 6 {
		t.Errorf("security.yml digest-scoped ci-test restore prefixes = %d, want 6", got)
	}
	if got := strings.Count(security, "1.26.5-ci-test-"); got != 12 {
		t.Errorf("security.yml ci-test restore prefixes = %d, want 12", got)
	}

	for _, workflow := range []struct {
		name string
		text string
		want int
	}{
		{name: "ci.yml", text: ci, want: 2},
		{name: "security.yml", text: security, want: 6},
	} {
		if got := strings.Count(workflow.text, "cache-matched-key"); got != workflow.want {
			t.Errorf("%s restored-key reports = %d, want %d", workflow.name, got, workflow.want)
		}
		if got := strings.Count(workflow.text, " bytes=$bytes"); got != workflow.want {
			t.Errorf("%s byte-size reports = %d, want %d", workflow.name, got, workflow.want)
		}
	}

	if strings.Contains(ci, "cache-dependency-path: glade/go.sum") {
		t.Error("ci.yml retains setup-go cache-dependency-path")
	}
	if strings.Contains(security, "cache-dependency-path: go.sum") {
		t.Error("security.yml retains setup-go cache-dependency-path")
	}
	for _, want := range []string{
		"cache: npm",
		"glade/third_party/lwc/package-lock.json",
		"glade/site/package-lock.json",
	} {
		if !strings.Contains(ci, want) {
			t.Errorf("ci.yml npm cache block missing %q", want)
		}
	}
}

func TestSecurityWorkflowContract(t *testing.T) {
	workflowPath := filepath.Join("..", ".github", "workflows", "security.yml")
	workflow, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read %s: %v", workflowPath, err)
	}
	workflowText := string(workflow)
	if problem := securityWorkflowHeaderProblem(workflowText); problem != "" {
		t.Errorf("security.yml header contract: %s", problem)
	}
	jobs := workflowJobBlocks(t, workflowText)

	wantJobs := []string{"codeql", "dependency-review", "gosec", "govulncheck", "npm-audit", "scorecard"}
	gotJobs := make([]string, 0, len(jobs))
	for name := range jobs {
		gotJobs = append(gotJobs, name)
	}
	sort.Strings(gotJobs)
	if strings.Join(gotJobs, ",") != strings.Join(wantJobs, ",") {
		t.Fatalf("security.yml jobs = %v, want %v", gotJobs, wantJobs)
	}
	for _, jobName := range wantJobs {
		for _, marker := range []string{"runs-on: ubuntu-latest", "uses: actions/checkout@v6"} {
			if !strings.Contains(jobs[jobName], marker) {
				t.Errorf("%s job missing preserved marker %q", jobName, marker)
			}
		}
	}

	const gosecPin = "securego/gosec@9e6a9843d7a4a6e3e9a8539b02612c8a4aa3f889 # v2.27.1"
	if count := strings.Count(workflowText, gosecPin); count != 1 {
		t.Fatalf("security.yml exact gosec pin count = %d, want 1", count)
	}
	if strings.Contains(workflowText, "securego/gosec@v2") {
		t.Fatal("security.yml must not use the moving securego/gosec@v2 tag")
	}

	const scorecardPin = "ossf/scorecard-action@4eaacf0543bb3f2c246792bd56e8cdeffafb205a # v2.4.3"
	if count := strings.Count(workflowText, scorecardPin); count != 1 {
		t.Fatalf("security.yml exact Scorecard pin count = %d, want 1", count)
	}
	if strings.Contains(workflowText, "ossf/scorecard-action@v2") {
		t.Fatal("security.yml must not use the moving ossf/scorecard-action@v2 tag")
	}

	codeql := jobs["codeql"]
	for _, want := range []string{
		"timeout-minutes: 60",
		"languages: go",
		"config: |",
		"- uses: security-extended",
		"id: go/allocation-size-overflow",
		"build-mode: manual",
		`run: go build -o "${RUNNER_TEMP}/glade-codeql" ./cmd/glade`,
		"github/codeql-action/init@v4",
		"github/codeql-action/analyze@v4",
	} {
		if !strings.Contains(codeql, want) {
			t.Errorf("codeql job missing %q", want)
		}
	}
	if strings.Contains(codeql, "queries: +security-extended") {
		t.Error("codeql job must use inline config so the pathological allocation-size query can be filtered")
	}
	if strings.Contains(codeql, "github/codeql-action/autobuild@") {
		t.Error("codeql job must not use autobuild")
	}

	for _, jobName := range []string{"govulncheck", "codeql", "gosec"} {
		setupGo := workflowStepBlock(t, jobs[jobName], "uses: actions/setup-go@v6")
		if !strings.Contains(setupGo, "cache: false") {
			t.Errorf("%s setup-go step missing cache: false", jobName)
		}
		if strings.Contains(setupGo, "cache-dependency-path:") {
			t.Errorf("%s setup-go step retains cache-dependency-path", jobName)
		}
	}
	if count := strings.Count(workflowText, "uses: actions/setup-go@v6"); count != 3 {
		t.Errorf("security.yml setup-go step count = %d, want 3", count)
	}
	if strings.Contains(workflowText, "cache-dependency-path:") {
		t.Error("security.yml must not retain cache-dependency-path")
	}
	coverageMarkers := map[string][]string{
		"govulncheck":       {"go run golang.org/x/vuln/cmd/govulncheck@latest ./..."},
		"codeql":            {"actions: read", "contents: read", "security-events: write"},
		"gosec":             {"actions: read", "contents: read", "security-events: write", gosecPin, "args: -no-fail -fmt sarif -out gosec.sarif ./...", "github/codeql-action/upload-sarif@v4", "sarif_file: gosec.sarif"},
		"npm-audit":         {"actions/setup-node@v6", `node-version: "22"`, "npm audit --omit=dev --audit-level=high", "working-directory: third_party/lwc", "working-directory: contrib/vscode-glade"},
		"dependency-review": {"if: github.event_name == 'pull_request'", "contents: read", "actions/dependency-review-action@v5", "fail-on-severity: high"},
		"scorecard":         {"actions: read", "contents: read", "id-token: write", "security-events: write", scorecardPin, "results_file: scorecard.sarif", "results_format: sarif", "publish_results: true", "github/codeql-action/upload-sarif@v4", "sarif_file: scorecard.sarif"},
	}
	for jobName, markers := range coverageMarkers {
		for _, marker := range markers {
			if !strings.Contains(jobs[jobName], marker) {
				t.Errorf("%s job missing preserved marker %q", jobName, marker)
			}
		}
	}
}

func TestSecurityWorkflowHeaderRejectsNestedPermissionAndTriggerMutations(t *testing.T) {
	workflowPath := filepath.Join("..", ".github", "workflows", "security.yml")
	workflow, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read %s: %v", workflowPath, err)
	}
	workflowText := string(workflow)
	mutations := map[string]string{
		"job permission substitute": strings.Replace(workflowText, "\npermissions:\n  contents: read\n\njobs:\n", "\njobs:\n", 1),
		"changed trigger":           strings.Replace(workflowText, "  pull_request:\n", "  pull_request_target:\n", 1),
	}
	for name, mutated := range mutations {
		t.Run(name, func(t *testing.T) {
			if mutated == workflowText {
				t.Fatal("test fixture did not mutate workflow header")
			}
			if !strings.Contains(mutated, "contents: read") {
				t.Fatal("test mutation unexpectedly removed all job-level contents permissions")
			}
			if problem := securityWorkflowHeaderProblem(mutated); problem == "" {
				t.Fatal("header contract accepted invalid top-level workflow structure")
			}
		})
	}
}

func securityWorkflowHeaderProblem(workflow string) string {
	header, _, found := strings.Cut(workflow, "\njobs:\n")
	if !found {
		return "missing top-level jobs boundary"
	}
	blocks, problem := workflowTopLevelBlocks(header)
	if problem != "" {
		return problem
	}
	if len(blocks) != 3 {
		return fmt.Sprintf("top-level header block count = %d, want 3", len(blocks))
	}
	wantBlocks := map[string]string{
		"name": "name: Security",
		"on": `on:
  pull_request:
  push:
    branches:
      - main
  schedule:
    - cron: "17 9 * * 1"
  workflow_dispatch:`,
		"permissions": `permissions:
  contents: read`,
	}
	for name, want := range wantBlocks {
		got, ok := blocks[name]
		if !ok {
			return fmt.Sprintf("missing top-level %s block", name)
		}
		if got != want {
			return fmt.Sprintf("top-level %s block does not match required structure", name)
		}
	}
	return ""
}

func workflowTopLevelBlocks(header string) (map[string]string, string) {
	lines := strings.Split(header, "\n")
	blocks := make(map[string]string)
	for i := 0; i < len(lines); {
		if lines[i] == "" {
			i++
			continue
		}
		if strings.HasPrefix(lines[i], " ") {
			return nil, fmt.Sprintf("unexpected indented header line %q", lines[i])
		}
		name, _, ok := strings.Cut(lines[i], ":")
		if !ok || name == "" {
			return nil, fmt.Sprintf("invalid top-level header line %q", lines[i])
		}
		start := i
		i++
		for i < len(lines) && (lines[i] == "" || strings.HasPrefix(lines[i], " ")) {
			i++
		}
		if _, duplicate := blocks[name]; duplicate {
			return nil, fmt.Sprintf("duplicate top-level %s block", name)
		}
		blocks[name] = strings.TrimSpace(strings.Join(lines[start:i], "\n"))
	}
	return blocks, ""
}

func workflowJobBlocks(t *testing.T, workflow string) map[string]string {
	t.Helper()
	lines := strings.Split(workflow, "\n")
	jobsLine := -1
	for i, line := range lines {
		if line == "jobs:" {
			jobsLine = i
			break
		}
	}
	if jobsLine < 0 {
		t.Fatal("security.yml missing top-level jobs block")
	}

	blocks := make(map[string]string)
	for i := jobsLine + 1; i < len(lines); {
		line := lines[i]
		if line == "" || strings.HasPrefix(line, "  ") {
			if len(line) >= 3 && strings.HasPrefix(line, "  ") && line[2] != ' ' && strings.HasSuffix(line, ":") {
				name := strings.TrimSuffix(strings.TrimSpace(line), ":")
				start := i
				i++
				for i < len(lines) && (lines[i] == "" || strings.HasPrefix(lines[i], "    ")) {
					i++
				}
				blocks[name] = strings.Join(lines[start:i], "\n")
				continue
			}
			i++
			continue
		}
		break
	}
	return blocks
}

func workflowStepBlock(t *testing.T, jobBlock, marker string) string {
	t.Helper()
	lines := strings.Split(jobBlock, "\n")
	for i, line := range lines {
		if !strings.Contains(line, marker) {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		start := i
		for start > 0 {
			previous := lines[start-1]
			previousIndent := len(previous) - len(strings.TrimLeft(previous, " "))
			if strings.HasPrefix(strings.TrimSpace(previous), "- ") && previousIndent == indent {
				start--
				break
			}
			if previousIndent < indent {
				break
			}
			start--
		}
		end := i + 1
		for end < len(lines) {
			next := lines[end]
			nextIndent := len(next) - len(strings.TrimLeft(next, " "))
			if strings.HasPrefix(strings.TrimSpace(next), "- ") && nextIndent <= indent {
				break
			}
			end++
		}
		return strings.Join(lines[start:end], "\n")
	}
	t.Fatalf("job block missing step marker %q", marker)
	return ""
}

func TestCIGoTestLogWrapperIsWired(t *testing.T) {
	script, err := os.ReadFile("ci-go-test.sh")
	if err != nil {
		t.Fatalf("read ci-go-test.sh: %v", err)
	}
	scriptText := string(script)
	for _, want := range []string{
		`export GOMAXPROCS="${GOMAXPROCS:-2}"`,
		"run_with_heartbeat",
		"run_core_tests",
		"run_full_tests",
		"run_apextest_matrix_shard",
		"heartbeat_pid",
		`kill -0 "${pid}"`,
		"./internal/apextest",
		`go test -list '^Test' "${apex_package}"`,
		`run_json_with_heartbeat "go test Apex shard ${index}" "${events}" -timeout=30m -run "${regex}" "${apex_package}"`,
		"CI_SHARD_PLANNER",
		"validation-summary.json",
		`run_package_lane "gladecli"`,
		`run_package_lane "sema"`,
		"go test -json",
		"run_json_with_heartbeat",
		"CI_VERBOSE",
		"testlog_status_files",
		"owned_child_roots",
		"terminate_owned_tree",
		`trap 'handle_wrapper_signal 143' TERM`,
		`if [[ "${BASH_SOURCE[0]}" == "$0" ]]`,
	} {
		if !strings.Contains(scriptText, want) {
			t.Fatalf("ci-go-test.sh missing %q", want)
		}
	}
	if strings.Contains(scriptText, `grep '^Test' || true`) {
		t.Fatal("ci-go-test.sh must not suppress Apex test discovery failures")
	}
	for _, forbidden := range []string{"apextest.test", "go test -c", "-test.run", "-test.list"} {
		if strings.Contains(scriptText, forbidden) {
			t.Fatalf("ci-go-test.sh retains forbidden direct-binary path %q", forbidden)
		}
	}

	workflowPath := filepath.Join("..", ".github", "workflows", "ci.yml")
	workflow, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read %s: %v", workflowPath, err)
	}
	workflowText := string(workflow)
	for _, want := range []string{
		"timeout-minutes: 30",
		"GOMAXPROCS: \"2\"",
		"go-version: \"1.26.5\"",
		"actions/checkout@v6",
		"client-id: ${{ vars.GLADE_APP_CLIENT_ID }}",
		"actions/setup-go@v6",
		"actions/setup-node@v6",
		"scripts/ci-go-test.sh core",
		"apextest:",
		"matrix:",
		"shard: [0, 1]",
		"cache: false",
		"ci-apextest-${{ matrix.shard }}",
		"go-mod-v1-1.26.5-${{ runner.os }}-${{ runner.arch }}-${{ hashFiles('go.sum') }}-ci-apextest-${{ matrix.shard }}-${{ github.sha }}-${{ github.run_id }}-${{ github.run_attempt }}",
		"go-build-v1-1.26.5-${{ runner.os }}-${{ runner.arch }}-${{ hashFiles('go.sum') }}-ci-apextest-${{ matrix.shard }}-${{ github.sha }}-${{ github.run_id }}-${{ github.run_attempt }}",
		"scripts/ci-go-test.sh apex-shard \"${{ matrix.shard }}\"",
		"if: always()",
		"actions/upload-artifact@v6",
		"CI_APEXTEST_ARTIFACT_DIR: ci-artifacts/apextest-${{ matrix.shard }}",
		"ci-artifacts/apextest-${{ matrix.shard }}/discovery-stderr.txt",
		"ci-artifacts/apextest-${{ matrix.shard }}/discovery-command.txt",
	} {
		if !strings.Contains(workflowText, want) {
			t.Fatalf("ci.yml missing %q", want)
		}
	}
	if strings.Contains(workflowText, "app-id: ${{ vars.GLADE_APP_CLIENT_ID }}") {
		t.Fatalf("ci.yml should use create-github-app-token client-id, not deprecated app-id")
	}
	if strings.Contains(workflowText, "scripts/ci-go-test.sh race") {
		t.Fatalf("ci.yml should not run the full race suite on GitHub-hosted runners")
	}
	if strings.Count(workflowText, "go test -json") != 0 {
		t.Fatal("ci.yml must delegate the native Apex run to the tested wrapper")
	}
	if strings.Count(workflowText, "shard: [0, 1]") != 1 {
		t.Fatal("ci.yml must define exactly one two-cell Apex matrix")
	}
	if strings.Contains(workflowText, ".ci-artifacts") || strings.Contains(scriptText, ".ci-artifacts") {
		t.Fatal("Apex evidence must use a non-hidden path so upload-artifact includes it by default")
	}
}

func TestCIPackageLanesRouteThroughCheckedManifest(t *testing.T) {
	manifest, err := os.ReadFile("ci-package-lanes.json")
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Version int                 `json:"version"`
		Lanes   map[string][]string `json:"lanes"`
	}
	if err := json.Unmarshal(manifest, &document); err != nil {
		t.Fatal(err)
	}
	if document.Version != 1 || len(document.Lanes) != 6 {
		t.Fatalf("manifest version/lane count = %d/%d", document.Version, len(document.Lanes))
	}
	remaining := document.Lanes["remaining-go"]
	if len(remaining) == 0 {
		t.Fatal("remaining-go must be explicitly enumerated")
	}
	for _, pkg := range remaining {
		if strings.Contains(pkg, "...") || strings.ContainsAny(pkg, "*?") {
			t.Fatalf("remaining-go contains catch-all %q", pkg)
		}
	}
	script, err := os.ReadFile("ci-go-test.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, lane := range []string{"gladecli", "sema", "server-and-playground", "repoguard", "remaining-go"} {
		if !strings.Contains(string(script), `run_named_package_lane "`+lane+`"`) {
			t.Errorf("core routing missing lane %q", lane)
		}
	}
}

func TestCIPackageLaneCommandRunsOnlyRequestedManifestPackagesAndPreservesStatus(t *testing.T) {
	manifestData, err := os.ReadFile("ci-package-lanes.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Lanes map[string][]string `json:"lanes"`
	}
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatal(err)
	}
	toArgument := func(pkg string) string {
		return "./" + strings.TrimPrefix(pkg, "github.com/glade-sh/glade/")
	}
	allPackages := make(map[string]string)
	for lane, packages := range manifest.Lanes {
		for _, pkg := range packages {
			allPackages[toArgument(pkg)] = lane
		}
	}

	cases := []struct {
		lane        string
		wantTimeout string
		wantP       string
	}{
		{lane: "gladecli", wantTimeout: "-timeout=30m"},
		{lane: "sema", wantTimeout: "-timeout=30m"},
		{lane: "server-and-playground", wantTimeout: "-timeout=30m"},
		{lane: "repoguard", wantTimeout: "-timeout=30m"},
		{lane: "remaining-go", wantTimeout: "-timeout=20m", wantP: "-p=2"},
	}
	for _, tc := range cases {
		t.Run(tc.lane, func(t *testing.T) {
			dir := t.TempDir()
			calls := filepath.Join(dir, "calls")
			fakeGo := filepath.Join(dir, "go")
			fakeRenderer := filepath.Join(dir, "testlog")
			goScript := `#!/usr/bin/env bash
printf '%s\n' "$*" >> "$FIXTURE_CALLS"
printf '%s\n' '{"Action":"output","Package":"example.test/pkg","Output":"failure\n"}'
exit 23
`
			rendererScript := "#!/usr/bin/env bash\ncat >/dev/null\n"
			if err := os.WriteFile(fakeGo, []byte(goScript), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(fakeRenderer, []byte(rendererScript), 0o700); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("bash", "ci-go-test.sh", "lane", tc.lane)
			cmd.Env = append(os.Environ(),
				"PATH="+dir+":"+os.Getenv("PATH"),
				"FIXTURE_CALLS="+calls,
				"CI_GO_COMMAND="+realGoCommand(t),
				"CI_TESTLOG_RENDERER="+fakeRenderer,
				"CI_GO_TEST_ARTIFACT_DIR="+filepath.Join(dir, "artifacts"),
				"CI_GO_TEST_HEARTBEAT_SECONDS=1",
			)
			out, err := cmd.CombinedOutput()
			exitErr, ok := err.(*exec.ExitError)
			if !ok || exitErr.ExitCode() != 23 {
				t.Fatalf("lane status = %v, want native 23\n%s", err, out)
			}
			callData, err := os.ReadFile(calls)
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(strings.TrimSpace(string(callData)), "\n")
			if len(lines) != 1 {
				t.Fatalf("native executions = %d, want 1; calls:\n%s", len(lines), callData)
			}
			fields := strings.Fields(lines[0])
			var gotPackages []string
			for _, field := range fields {
				if strings.HasPrefix(field, "./") {
					gotPackages = append(gotPackages, field)
				}
			}
			var wantPackages []string
			for _, pkg := range manifest.Lanes[tc.lane] {
				wantPackages = append(wantPackages, toArgument(pkg))
			}
			sort.Strings(gotPackages)
			sort.Strings(wantPackages)
			if !reflect.DeepEqual(gotPackages, wantPackages) {
				t.Fatalf("lane packages = %v, want %v", gotPackages, wantPackages)
			}
			for _, field := range fields {
				if owner, exists := allPackages[field]; exists && owner != tc.lane {
					t.Errorf("lane executed package %s owned by %s", field, owner)
				}
			}
			if !strings.Contains(lines[0], tc.wantTimeout) {
				t.Errorf("lane call missing timeout %s: %s", tc.wantTimeout, lines[0])
			}
			if tc.wantP != "" {
				if !strings.Contains(lines[0], tc.wantP) {
					t.Errorf("lane call missing parallelism %s: %s", tc.wantP, lines[0])
				}
			} else if strings.Contains(lines[0], "-p=") {
				t.Errorf("lane call has unexpected parallelism: %s", lines[0])
			}
		})
	}
}

func TestCIPackageLaneCommandRejectsInvalidArguments(t *testing.T) {
	cases := [][]string{
		{"lane"},
		{"lane", "unknown"},
		{"lane", "apextest"},
		{"lane", "gladecli", "extra"},
	}
	for _, args := range cases {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			cmd := exec.Command("bash", append([]string{"ci-go-test.sh"}, args...)...)
			out, err := cmd.CombinedOutput()
			exitErr, ok := err.(*exec.ExitError)
			if !ok || exitErr.ExitCode() != 2 {
				t.Fatalf("invalid args %v status = %v, want 2\n%s", args, err, out)
			}
		})
	}
}

func TestCIVetHasOneAuthoritativeGateAndNoImplicitLaneWork(t *testing.T) {
	workflow, err := os.ReadFile(filepath.Join("..", ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(workflow), "go vet ./..."); got != 1 {
		t.Fatalf("standalone full vet gates = %d, want 1", got)
	}
	script, err := os.ReadFile("ci-go-test.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`go test -json -vet=off "$@"`,
		`GOFLAGS="${GOFLAGS:+${GOFLAGS} }-vet=off" go test -list '^Test' "${apex_package}"`,
	} {
		if !strings.Contains(string(script), want) {
			t.Errorf("wrapper missing implicit-vet suppression %q", want)
		}
	}
}

func TestCIGoTestLogModesPreserveFullDefaultAndCoreExcludesApex(t *testing.T) {
	for _, tc := range []struct {
		name          string
		args          []string
		wantApex      bool
		wantRace      bool
		wantTestCalls int
	}{
		{name: "default", wantApex: true, wantTestCalls: 6},
		{name: "test", args: []string{"test"}, wantApex: true, wantTestCalls: 6},
		{name: "core", args: []string{"core"}, wantTestCalls: 5},
		{name: "race", args: []string{"race"}, wantApex: true, wantRace: true, wantTestCalls: 6},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			calls := filepath.Join(dir, "calls")
			fakeGo := filepath.Join(dir, "go")
			fakeRenderer := filepath.Join(dir, "testlog")
			script := `#!/usr/bin/env bash
printf '%s\n' "$*" >> "$FIXTURE_CALLS"
if [[ "$1" == "list" ]]; then
  printf '%s\n' github.com/glade-sh/glade/internal/apextest github.com/glade-sh/glade/internal/other
fi
`
			rendererScript := `#!/usr/bin/env bash
while [[ $# -gt 0 ]]; do
  if [[ "$1" == "-output" ]]; then
    output="$2"
    shift 2
  else
    shift
  fi
done
tee "$output"
`
			if err := os.WriteFile(fakeGo, []byte(script), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(fakeRenderer, []byte(rendererScript), 0o700); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("bash", append([]string{"ci-go-test.sh"}, tc.args...)...)
			cmd.Env = append(os.Environ(), "PATH="+dir+":"+os.Getenv("PATH"), "FIXTURE_CALLS="+calls, "CI_GO_COMMAND="+realGoCommand(t), "CI_TESTLOG_RENDERER="+fakeRenderer, "CI_GO_TEST_ARTIFACT_DIR="+filepath.Join(dir, "artifacts"), "CI_GO_TEST_HEARTBEAT_SECONDS=1")
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("mode failed: %v\n%s", err, out)
			}
			b, err := os.ReadFile(calls)
			if err != nil {
				t.Fatal(err)
			}
			callText := string(b)
			gotApex := strings.Contains(callText, "./internal/apextest")
			if gotApex != tc.wantApex {
				t.Fatalf("Apex invocation = %v, want %v; calls:\n%s", gotApex, tc.wantApex, b)
			}
			lines := strings.Split(strings.TrimSpace(callText), "\n")
			var testCalls []string
			for _, line := range lines {
				if strings.HasPrefix(line, "test ") {
					testCalls = append(testCalls, line)
				}
			}
			if len(testCalls) != tc.wantTestCalls {
				t.Fatalf("test calls = %d, want %d; calls:\n%s", len(testCalls), tc.wantTestCalls, b)
			}
			for _, call := range testCalls {
				if !strings.Contains(call, " -json ") {
					t.Errorf("test lane bypassed JSON renderer: %s", call)
				}
				if got := strings.Contains(call, " -race "); got != tc.wantRace {
					t.Errorf("race marker = %v, want %v: %s", got, tc.wantRace, call)
				}
			}
			for _, pkg := range []string{"./internal/gladecli", "./internal/playground", "./internal/sema", "./internal/server", "./cmd/glade"} {
				if got := strings.Count(callText, pkg); got != 1 {
					t.Errorf("package lane %s executions = %d, want 1; calls:\n%s", pkg, got, b)
				}
			}
		})
	}
}

func TestCIGoTestLogAlwaysPreservesNativeStatus(t *testing.T) {
	for _, tc := range []struct {
		name       string
		nativeRC   int
		rendererRC int
		wantRC     int
		wantCalls  int
	}{
		{name: "both success", wantCalls: 5},
		{name: "native success renderer fail", rendererRC: 7, wantCalls: 5},
		{name: "native fail renderer success", nativeRC: 23, wantRC: 23, wantCalls: 1},
		{name: "both fail", nativeRC: 23, rendererRC: 7, wantRC: 23, wantCalls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			calls := filepath.Join(dir, "calls")
			fakeGo := filepath.Join(dir, "go")
			fakeRenderer := filepath.Join(dir, "testlog")
			goScript := `#!/usr/bin/env bash
if [[ "$1" == "list" ]]; then
  exit 0
fi
printf '%s\n' "$*" >> "$FIXTURE_CALLS"
printf '%s\n' '{"Action":"output","Package":"example.test/pkg","Output":"ok  example.test/pkg 0.001s\n"}'
exit "$FIXTURE_NATIVE_RC"
`
			rendererScript := `#!/usr/bin/env bash
while [[ $# -gt 0 ]]; do
  case "$1" in
    -output) output="$2"; shift 2 ;;
    *) shift ;;
  esac
done
tee "$output"
exit "$FIXTURE_RENDERER_RC"
`
			if err := os.WriteFile(fakeGo, []byte(goScript), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(fakeRenderer, []byte(rendererScript), 0o700); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("bash", "ci-go-test.sh", "core")
			cmd.Env = append(os.Environ(),
				"PATH="+dir+":"+os.Getenv("PATH"),
				"FIXTURE_CALLS="+calls,
				fmt.Sprintf("FIXTURE_NATIVE_RC=%d", tc.nativeRC),
				fmt.Sprintf("FIXTURE_RENDERER_RC=%d", tc.rendererRC),
				"CI_TESTLOG_RENDERER="+fakeRenderer,
				"CI_GO_COMMAND="+realGoCommand(t),
				"CI_GO_TEST_ARTIFACT_DIR="+filepath.Join(dir, "artifacts"),
				"CI_GO_TEST_HEARTBEAT_SECONDS=1",
			)
			out, err := cmd.CombinedOutput()
			gotRC := 0
			if err != nil {
				exitErr, ok := err.(*exec.ExitError)
				if !ok {
					t.Fatalf("wrapper error = %v\n%s", err, out)
				}
				gotRC = exitErr.ExitCode()
			}
			if gotRC != tc.wantRC {
				t.Fatalf("wrapper status = %d, want native status %d\n%s", gotRC, tc.wantRC, out)
			}
			callData, readErr := os.ReadFile(calls)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if got := strings.Count(string(callData), "test -json"); got != tc.wantCalls {
				t.Fatalf("native test calls = %d, want %d; calls:\n%s", got, tc.wantCalls, callData)
			}
			if tc.rendererRC != 0 && !strings.Contains(string(out), "renderer failed with status 7") {
				t.Fatalf("renderer failure was not logged:\n%s", out)
			}
		})
	}
}

func TestCIApexShardLogUsesQuietRendererOnce(t *testing.T) {
	dir := t.TempDir()
	renderer := filepath.Join(dir, "testlog")
	if err := os.WriteFile(renderer, []byte("#!/usr/bin/env bash\ncat >/dev/null\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CI_TESTLOG_RENDERER", renderer)
	events := `{"Action":"output","Package":"` + fixturePackage + `","Test":"TestAlpha","Output":"=== RUN   TestAlpha\n"}` + "\n" +
		`{"Action":"output","Package":"` + fixturePackage + `","Test":"TestAlpha","Output":"    alpha_test.go:9: successful detail\n"}` + "\n" +
		passEvent("TestAlpha")
	out, err, artifacts := runApexShardFixture(t, "0", "TestAlpha\nTestBeta\nok  \t"+fixturePackage+"\n", 0, validFixturePlan(), 0, events, 0)
	if err != nil {
		t.Fatalf("apex shard failed: %v\n%s", err, out)
	}
	if strings.Contains(out, "successful detail") || strings.Contains(out, `"Action"`) {
		t.Fatalf("Apex shard bypassed quiet renderer:\n%s", out)
	}
	calls, readErr := os.ReadFile(filepath.Join(filepath.Dir(artifacts), "calls"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if got := strings.Count(string(calls), "test -json"); got != 1 {
		t.Fatalf("Apex native JSON executions = %d, want 1; calls:\n%s", got, calls)
	}
}

func TestCIGoTestLogPreservesOriginalExitCodeAndRunsOnce(t *testing.T) {
	dir := t.TempDir()
	calls := filepath.Join(dir, "calls")
	rendererArgs := filepath.Join(dir, "renderer-args")
	fakeGo := filepath.Join(dir, "go")
	fakeRenderer := filepath.Join(dir, "testlog")
	artifactDir := filepath.Join(dir, "artifacts")
	goScript := `#!/usr/bin/env bash
printf '%s\n' "$*" >> "$FIXTURE_CALLS"
if [[ "$1" == "test" ]]; then
  printf '%s\n' '{"Action":"output","Package":"example.test/pkg","Test":"TestFailure","Output":"    failure_test.go:17: deliberate failure\n"}'
  exit 23
fi
`
	rendererScript := `#!/usr/bin/env bash
printf '%s\n' "$*" > "$FIXTURE_RENDERER_ARGS"
output=
while [[ $# -gt 0 ]]; do
  case "$1" in
    -output) output="$2"; shift 2 ;;
    *) shift ;;
  esac
done
tee "$output"
exit "${FIXTURE_RENDERER_RC:-0}"
`
	if err := os.WriteFile(fakeGo, []byte(goScript), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fakeRenderer, []byte(rendererScript), 0o700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", "ci-go-test.sh", "core")
	cmd.Env = append(os.Environ(),
		"PATH="+dir+":"+os.Getenv("PATH"),
		"FIXTURE_CALLS="+calls,
		"FIXTURE_RENDERER_ARGS="+rendererArgs,
		"FIXTURE_RENDERER_RC=7",
		"CI_TESTLOG_RENDERER="+fakeRenderer,
		"CI_GO_COMMAND="+realGoCommand(t),
		"CI_GO_TEST_ARTIFACT_DIR="+artifactDir,
		"CI_GO_TEST_HEARTBEAT_SECONDS=1",
		"CI_VERBOSE=1",
	)
	out, err := cmd.CombinedOutput()
	exitErr, ok := err.(*exec.ExitError)
	if !ok || exitErr.ExitCode() != 23 {
		t.Fatalf("wrapper exit = %v, want 23\n%s", err, out)
	}
	callData, readErr := os.ReadFile(calls)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if got := strings.Count(string(callData), "test -json"); got != 1 {
		t.Fatalf("underlying JSON test executions = %d, want 1; calls:\n%s", got, callData)
	}
	entries, readErr := os.ReadDir(artifactDir)
	if readErr != nil {
		t.Fatalf("read artifact directory: %v", readErr)
	}
	if len(entries) != 1 {
		t.Fatalf("raw artifacts = %d, want 1", len(entries))
	}
	raw, readErr := os.ReadFile(filepath.Join(artifactDir, entries[0].Name()))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !strings.Contains(string(raw), "deliberate failure") {
		t.Fatalf("raw artifact omitted failure output:\n%s", raw)
	}
	args, readErr := os.ReadFile(rendererArgs)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !strings.Contains(string(args), "-verbose") {
		t.Fatalf("CI_VERBOSE=1 did not reach renderer: %s", args)
	}
	if !strings.Contains(string(out), "renderer failed with status 7") {
		t.Fatalf("renderer failure was not explicit:\n%s", out)
	}
}

func TestCIGoTestLogSignalsTerminateOwnedChildren(t *testing.T) {
	for _, tc := range []struct {
		name       string
		signal     syscall.Signal
		wantRC     int
		jsonRunner bool
	}{
		{name: "ordinary heartbeat TERM", signal: syscall.SIGTERM, wantRC: 143},
		{name: "ordinary heartbeat INT", signal: syscall.SIGINT, wantRC: 130},
		{name: "JSON pipeline TERM", signal: syscall.SIGTERM, wantRC: 143, jsonRunner: true},
		{name: "JSON pipeline INT", signal: syscall.SIGINT, wantRC: 130, jsonRunner: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			unrelated := exec.Command("sleep", "30")
			if err := unrelated.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				_ = unrelated.Process.Kill()
				_ = unrelated.Wait()
			})
			worker := filepath.Join(dir, "worker")
			fakeGo := filepath.Join(dir, "go")
			fakeRenderer := filepath.Join(dir, "renderer")
			workerScript := "#!/usr/bin/env bash\nsleep 30 &\nwait\n"
			goScript := `#!/usr/bin/env bash
if [[ "$1" == "list" ]]; then exit 0; fi
sleep 30 &
wait
`
			rendererScript := "#!/usr/bin/env bash\nsleep 30 &\nwait\n"
			for path, body := range map[string]string{worker: workerScript, fakeGo: goScript, fakeRenderer: rendererScript} {
				if err := os.WriteFile(path, []byte(body), 0o700); err != nil {
					t.Fatal(err)
				}
			}

			var cmd *exec.Cmd
			if tc.jsonRunner {
				cmd = exec.Command("bash", "ci-go-test.sh", "core")
			} else {
				cmd = exec.Command("bash", "-c", `source ./ci-go-test.sh; run_with_heartbeat "fixture heartbeat" "$1"`, "signal-fixture", worker)
			}
			cmd.Env = append(os.Environ(),
				"PATH="+dir+":"+os.Getenv("PATH"),
				"CI_GO_COMMAND="+realGoCommand(t),
				"CI_TESTLOG_RENDERER="+fakeRenderer,
				"CI_GO_TEST_ARTIFACT_DIR="+filepath.Join(dir, "artifacts"),
				"CI_GO_TEST_HEARTBEAT_SECONDS=30",
			)
			var output bytes.Buffer
			cmd.Stdout = &output
			cmd.Stderr = &output
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			descendants := waitForDescendants(t, cmd.Process.Pid, 4)
			if err := cmd.Process.Signal(tc.signal); err != nil {
				t.Fatal(err)
			}
			err := cmd.Wait()
			exitErr, ok := err.(*exec.ExitError)
			if !ok || exitErr.ExitCode() != tc.wantRC {
				t.Fatalf("wrapper exit = %v, want %d\n%s", err, tc.wantRC, output.String())
			}
			assertProcessesExit(t, descendants)
			if err := unrelated.Process.Signal(syscall.Signal(0)); err != nil {
				t.Fatalf("unrelated process was terminated: %v", err)
			}
		})
	}
}

func waitForDescendants(t *testing.T, root, minimum int) []int {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		pids := descendantPIDs(t, root)
		if len(pids) >= minimum {
			return pids
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("process %d did not start %d descendants", root, minimum)
	return nil
}

func descendantPIDs(t *testing.T, root int) []int {
	t.Helper()
	out, err := exec.Command("pgrep", "-P", strconv.Itoa(root)).Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil
		}
		t.Fatalf("pgrep children of %d: %v", root, err)
	}
	var descendants []int
	for _, field := range strings.Fields(string(out)) {
		pid, err := strconv.Atoi(field)
		if err != nil {
			t.Fatalf("parse child pid %q: %v", field, err)
		}
		descendants = append(descendants, pid)
		descendants = append(descendants, descendantPIDs(t, pid)...)
	}
	return descendants
}

func assertProcessesExit(t *testing.T, pids []int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var alive []int
		for _, pid := range pids {
			if err := syscall.Kill(pid, 0); err == nil || err == syscall.EPERM {
				alive = append(alive, pid)
			}
		}
		if len(alive) == 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	var alive []int
	for _, pid := range pids {
		if err := syscall.Kill(pid, 0); err == nil || err == syscall.EPERM {
			alive = append(alive, pid)
		}
	}
	t.Fatalf("owned processes survived wrapper signal: %v", alive)
}
