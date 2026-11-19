package scripts

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func TestCIRaceLWCHistoryBalancesAllDiscoveredTests(t *testing.T) {
	root := t.TempDir()
	head := strings.Repeat("c", 40)
	const packageName = "github.com/glade-sh/glade/internal/lwc/compile"
	names := make([]string, 0, 39)
	historyTests := make([]map[string]any, 0, 27)
	for index := 0; index < 27; index++ {
		name := fmt.Sprintf("TestConformance%02d", index)
		names = append(names, name)
		duration := 1000
		if index < 8 {
			duration = 100000
		}
		historyTests = append(historyTests, map[string]any{"name": name, "durationMillis": duration})
	}
	// Normal CI's conformance history does not contain the twelve tests
	// owned by its Node lane. Race must still discover and execute them all.
	for index := 0; index < 12; index++ {
		names = append(names, fmt.Sprintf("TestNode%02d", index))
	}
	history, err := json.Marshal(map[string]any{"version": 1, "package": packageName, "complete": true, "tests": historyTests})
	if err != nil {
		t.Fatal(err)
	}
	historyPath := filepath.Join(root, "history.json")
	if err := os.WriteFile(historyPath, history, 0o600); err != nil {
		t.Fatal(err)
	}
	planner := filepath.Join(root, "cishard")
	if out, err := exec.Command(realGoCommand(t), "build", "-o", planner, "./internal/cishard").CombinedOutput(); err != nil {
		t.Fatalf("build real planner: %v\n%s", err, out)
	}
	goCommand := writeRaceFixture(t, "fake-go", `#!/usr/bin/env bash
set -euo pipefail
if [[ "$*" == *"-list ."* ]]; then
  printf '%b' "$CI_RACE_TEST_NAMES"
  printf 'ok   github.com/glade-sh/glade/internal/lwc/compile 0.001s\n'
  exit 0
fi
while IFS= read -r name; do
  [[ -n "$name" ]] || continue
  if [[ "$*" == *"$name"* ]]; then
    printf '{"Action":"pass","Package":"github.com/glade-sh/glade/internal/lwc/compile","Test":"%s","Elapsed":0.01}\n' "$name"
  fi
done <<<"$CI_RACE_TEST_NAMES"
printf '{"Action":"pass","Package":"github.com/glade-sh/glade/internal/lwc/compile"}\n'
`)
	resource := writeRaceFixture(t, "fake-resource", raceResourceFixture())
	git := writeRaceFixture(t, "fake-git", "#!/usr/bin/env bash\nprintf '%s\\n' "+head+"\n")
	roots := make([]string, 8)
	for index := range roots {
		workdir := filepath.Join(root, fmt.Sprintf("runner-%d", index))
		if err := os.MkdirAll(workdir, 0o700); err != nil {
			t.Fatal(err)
		}
		out, err := runRacePackage(t, workdir, []string{
			"CI_RACE_GO_COMMAND=" + goCommand, "CI_RACE_SHARD_PLANNER=" + planner,
			"CI_RACE_RESOURCE_RUNNER=" + resource, "CI_RACE_CALL_LOG=" + filepath.Join(workdir, "calls.log"),
			"CI_RACE_GIT_COMMAND=" + git, "CI_RACE_HEAD_SHA=" + head,
			"CI_LWC_COMPILE_HISTORY_PATH=" + historyPath,
			"CI_RACE_TEST_NAMES=" + strings.Join(names, "\n") + "\n",
			fmt.Sprintf("CI_RACE_SHARD_INDEX=%d", index),
		}, "./internal/lwc/compile", "internal-lwc-compile")
		if err != nil {
			t.Fatalf("weighted runner %d: %v\n%s", index, err, out)
		}
		roots[index] = filepath.Join(workdir, "ci-artifacts/race/internal-lwc-compile")
	}
	data, err := os.ReadFile(filepath.Join(roots[0], "plan.json"))
	var plan struct {
		HistoryUsed bool `json:"historyUsed"`
		Shards      []struct {
			Tests    []string `json:"tests"`
			Duration int64    `json:"estimatedDurationMillis"`
		} `json:"shards"`
	}
	if err != nil || json.Unmarshal(data, &plan) != nil || !plan.HistoryUsed || len(plan.Shards) != 8 {
		t.Fatalf("weighted plan: %s err=%v", data, err)
	}
	var union []string
	for index, shard := range plan.Shards {
		longTests := 0
		for _, name := range shard.Tests {
			if name >= "TestConformance00" && name <= "TestConformance07" {
				longTests++
			}
		}
		if longTests != 1 || shard.Duration >= 200000 {
			t.Fatalf("weighted shard %d did not separate long cases: %#v", index, shard)
		}
		union = append(union, shard.Tests...)
	}
	sort.Strings(union)
	if strings.Join(union, "\n") != strings.Join(names, "\n") {
		t.Fatalf("weighted plan changed the complete mixed discovery: %v", union)
	}
	aggregate, err := filepath.Abs("ci-race-shard-aggregate.sh")
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(root, "union.json")
	args := append([]string{aggregate, "./internal/lwc/compile", output, head}, roots...)
	if out, err := exec.Command("bash", args...).CombinedOutput(); err != nil {
		t.Fatalf("weighted aggregate rejected full coverage: %v\n%s", err, out)
	}
	data, err = os.ReadFile(output)
	var result struct {
		Valid      bool `json:"valid"`
		Discovered int  `json:"discovered_count"`
	}
	if err != nil || json.Unmarshal(data, &result) != nil || !result.Valid || result.Discovered != 39 {
		t.Fatalf("weighted union summary=%s err=%v", data, err)
	}
}

func TestCIRaceApextestDeferredPolicy(t *testing.T) {
	for _, fixture := range []struct {
		name, reason, configured, suffix, test string
		valid                                  bool
	}{
		{name: "exact deferred prerequisite", valid: true},
		{name: "wrong reason", reason: "another missing dependency"},
		{name: "configured toolchain", configured: "/explicit/toolchain"},
		{name: "unrecognized test", test: "TestNotAdmitted"},
		{name: "unrecognized subtest", suffix: "/child"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			root := t.TempDir()
			testName := fixture.test
			if testName == "" {
				testName = "TestRunDataWeaveTypedAdmitted"
			}
			names := raceApextestTestNames()
			names[0] = testName
			goCommand := writeRaceFixture(t, "fake-go", raceApextestGoFixture())
			planner := writeRaceFixture(t, "fake-planner", raceApextestPlannerFixture())
			resource := writeRaceFixture(t, "fake-resource", raceResourceFixture())
			out, err := runRacePackage(t, root, []string{
				"CI_RACE_GO_COMMAND=" + goCommand, "CI_RACE_SHARD_PLANNER=" + planner,
				"CI_RACE_RESOURCE_RUNNER=" + resource, "CI_RACE_CALL_LOG=" + filepath.Join(root, "calls.log"),
				"CI_RACE_TEST_NAMES=" + strings.Join(names, "\n") + "\n", "TMPDIR=" + root,
				"CI_RACE_DEFERRED_TEST=" + testName, "CI_RACE_DEFERRED_REASON=" + fixture.reason,
				"CI_RACE_DEFERRED_SUFFIX=" + fixture.suffix, "GLADE_DATAWEAVE_APEX_TEST_HOME=" + fixture.configured,
			}, "./internal/apextest", "internal-apextest")
			if (err == nil) != fixture.valid {
				t.Fatalf("valid=%v, err=%v\n%s", fixture.valid, err, out)
			}
			if fixture.valid {
				data, err := os.ReadFile(filepath.Join(root, "ci-artifacts/race/internal-apextest/union-validation.json"))
				var result struct {
					Discovered int              `json:"discovered_count"`
					Deferred   []map[string]any `json:"deferred"`
					Valid      bool             `json:"valid"`
				}
				if err != nil || json.Unmarshal(data, &result) != nil || !result.Valid || result.Discovered != 16 || len(result.Deferred) != 1 || result.Deferred[0]["test"] != testName {
					t.Fatalf("deferred summary=%s err=%v", data, err)
				}
			}
		})
	}
}

func TestCIRaceApextestAggregateRevalidatesDeferredPrerequisites(t *testing.T) {
	t.Setenv("GLADE_DATAWEAVE_APEX_TEST_HOME", "")
	root := t.TempDir()
	head := strings.Repeat("b", 40)
	names := raceApextestTestNames()
	names[0] = "TestRunDataWeaveTypedAdmitted"
	goCommand := writeRaceFixture(t, "fake-go", raceApextestGoFixture())
	planner := writeRaceFixture(t, "fake-planner", raceApextestPlannerFixture())
	resource := writeRaceFixture(t, "fake-resource", raceResourceFixture())
	git := writeRaceFixture(t, "fake-git", "#!/usr/bin/env bash\nprintf '%s\\n' "+head+"\n")
	partitionDir, _, _ := writeRaceApextestPartitionForFixture(t, root, goCommand, names)
	partitionData, err := os.ReadFile(filepath.Join(partitionDir, "partition.json"))
	if err != nil {
		t.Fatal(err)
	}
	var partition struct {
		Runners []struct {
			Runner              string `json:"runner"`
			BatchTimeoutSeconds int    `json:"batch_timeout_seconds"`
		} `json:"runners"`
	}
	if err := json.Unmarshal(partitionData, &partition); err != nil {
		t.Fatal(err)
	}
	if len(partition.Runners) < 3 {
		t.Fatalf("fixture partition has %d runners", len(partition.Runners))
	}
	runnersRoot := filepath.Join(root, "runners")
	if err := os.MkdirAll(runnersRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, entry := range partition.Runners {
		workdir := filepath.Join(root, entry.Runner)
		if err := os.MkdirAll(workdir, 0o700); err != nil {
			t.Fatal(err)
		}
		out, err := runRacePackage(t, workdir, []string{
			"CI_RACE_GO_COMMAND=" + goCommand, "CI_RACE_SHARD_PLANNER=" + planner,
			"CI_RACE_RESOURCE_RUNNER=" + resource, "CI_RACE_CALL_LOG=" + filepath.Join(workdir, "calls.log"),
			"CI_RACE_GIT_COMMAND=" + git, "CI_RACE_HEAD_SHA=" + head,
			"CI_RACE_TIMEOUT_COMMAND=" + raceBatchTimeoutFixture(t),
			"CI_RACE_PARTITION_DIR=" + partitionDir, "CI_RACE_APEXTEST_RUNNER=" + entry.Runner,
			"CI_RACE_BATCH_TIMEOUT_SECONDS=" + strconv.Itoa(entry.BatchTimeoutSeconds),
			"CI_RACE_TEST_NAMES=" + strings.Join(names, "\n") + "\n", "TMPDIR=" + workdir,
			"CI_RACE_DEFERRED_TEST=" + names[0],
		}, "./internal/apextest", "internal-apextest")
		if err != nil {
			t.Fatalf("runner %s failed: %v\n%s", entry.Runner, err, out)
		}
		if err := os.CopyFS(filepath.Join(runnersRoot, fmt.Sprintf("race-internal-apextest-%s-44158211531-1", entry.Runner)), os.DirFS(filepath.Join(workdir, "ci-artifacts/race/internal-apextest"))); err != nil {
			t.Fatal(err)
		}
	}
	script, err := filepath.Abs("ci-race-apextest-aggregate.sh")
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(root, "union.json")
	args := []string{script, partitionDir, runnersRoot, output, head}
	if out, err := exec.Command("bash", args...).CombinedOutput(); err != nil {
		t.Fatalf("aggregate rejected deferred prerequisite: %v\n%s", err, out)
	}
	data, err := os.ReadFile(output)
	var result struct {
		Deferred []map[string]any `json:"deferred"`
		Valid    bool             `json:"valid"`
	}
	if err != nil || json.Unmarshal(data, &result) != nil || !result.Valid || len(result.Deferred) != 1 || result.Deferred[0]["test"] != names[0] {
		t.Fatalf("aggregate deferred summary=%s err=%v", data, err)
	}
	t.Setenv("GLADE_DATAWEAVE_APEX_TEST_HOME", "/explicit/toolchain")
	if out, err := exec.Command("bash", args...).CombinedOutput(); err == nil {
		t.Fatalf("aggregate accepted configured prerequisite skip: %s", out)
	}
}

func TestCIRaceBrowserRunnersAndAggregateCoverDiscovery(t *testing.T) {
	for _, topology := range []struct {
		pkg, slug string
		count     int
	}{
		{"./internal/visualforce", "internal-visualforce", 4},
		{"./internal/lwc/compile", "internal-lwc-compile", 8},
	} {
		t.Run(topology.slug, func(t *testing.T) {
			packageName := "github.com/glade-sh/glade/" + strings.TrimPrefix(topology.pkg, "./")
			names := make([]string, topology.count)
			wantCounts := make([]int, topology.count)
			for index := range names {
				names[index] = fmt.Sprintf("TestCase%02d", index)
				wantCounts[index] = 1
			}
			if topology.pkg == "./internal/visualforce" {
				// The Visualforce lane plans from its measured race rows, so
				// the discovery must hold one; the LWC leg keeps synthetic
				// names because its optional history file is absent.
				names[0] = measuredRaceHistoryName(t, topology.pkg)
			}

			root := t.TempDir()
			historyPath := filepath.Join(root, "history.json")
			if topology.pkg == "./internal/visualforce" {
				// A history configured for LWC must not reach another package's
				// planner. The fixture rejects any unexpected --history flag.
				if err := os.WriteFile(historyPath, []byte("LWC-only history"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			// LWC exercises the unchanged default plan when its optional
			// history file does not exist yet.
			head := strings.Repeat("a", 40)
			goCommand := writeRaceFixture(t, "fake-go", `#!/usr/bin/env bash
set -euo pipefail
if [[ "$*" == *"-list ."* ]]; then
  printf '%b' "$CI_RACE_TEST_NAMES"
  printf 'ok   %s 0.001s\n' "$CI_RACE_TEST_PACKAGE"
  exit 0
fi
printf 'native %s\n' "$*" >>"$CI_RACE_CALL_LOG"
while IFS= read -r name; do
  [[ -n "$name" ]] || continue
  if [[ "$*" == *"$name"* ]]; then
    printf '{"Action":"pass","Package":"%s","Test":"%s","Elapsed":0.01}\n' "$CI_RACE_TEST_PACKAGE" "$name"
  fi
done <<<"$CI_RACE_TEST_NAMES"
printf '{"Action":"pass","Package":"%s"}\n' "$CI_RACE_TEST_PACKAGE"
`)
			planner := writeRaceFixture(t, "fake-planner", racePlannerFixture())
			resource := writeRaceFixture(t, "fake-resource", raceResourceFixture())
			git := writeRaceFixture(t, "fake-git", "#!/usr/bin/env bash\nprintf '%s\\n' "+head+"\n")
			roots := make([]string, topology.count)
			for index := range roots {
				workdir := filepath.Join(root, fmt.Sprintf("runner-%d", index))
				if err := os.MkdirAll(workdir, 0o700); err != nil {
					t.Fatal(err)
				}
				log := filepath.Join(workdir, "calls.log")
				out, err := runRacePackage(t, workdir, []string{
					"CI_RACE_GO_COMMAND=" + goCommand, "CI_RACE_SHARD_PLANNER=" + planner,
					"CI_RACE_RESOURCE_RUNNER=" + resource, "CI_RACE_CALL_LOG=" + log,
					"CI_RACE_GIT_COMMAND=" + git, "CI_RACE_HEAD_SHA=" + head,
					"CI_LWC_COMPILE_HISTORY_PATH=" + historyPath,
					"CI_RACE_TEST_NAMES=" + strings.Join(names, "\n") + "\n", "CI_RACE_TEST_PACKAGE=" + packageName,
					fmt.Sprintf("CI_RACE_SHARD_INDEX=%d", index),
				}, topology.pkg, topology.slug)
				if err != nil {
					t.Fatalf("runner %d: %v\n%s", index, err, out)
				}
				calls, err := os.ReadFile(log)
				if err != nil || strings.Count(string(calls), "native test -json -race -count=1 -timeout=55m -run") != 1 || !strings.Contains(string(calls), fmt.Sprintf("start race-%s-shard-%d", topology.slug, index)) {
					t.Fatalf("runner %d must run its one shard under the default go test deadline below the batch deadline: %s, %v", index, calls, err)
				}
				roots[index] = filepath.Join(workdir, "ci-artifacts/race", topology.slug)
			}
			aggregate, err := filepath.Abs("ci-race-shard-aggregate.sh")
			if err != nil {
				t.Fatal(err)
			}
			// The planner assigns the sorted discovery to shards in order, so
			// shard 0 carries the alphabetically first name.
			sortedNames := append([]string(nil), names...)
			sort.Strings(sortedNames)
			for _, fixture := range []struct {
				name   string
				mutate func(t *testing.T, roots []string)
				valid  bool
			}{
				{name: "exact union", valid: true},
				{name: "missing shard", mutate: func(t *testing.T, roots []string) { roots[len(roots)-1] = filepath.Join(root, "missing") }},
				{name: "duplicate runner", mutate: func(t *testing.T, roots []string) { roots[len(roots)-1] = roots[0] }},
				{name: "omitted terminal", mutate: func(t *testing.T, roots []string) {
					path := filepath.Join(roots[0], "shard-0/events.json")
					if err := os.WriteFile(path, []byte(fmt.Sprintf("{\"Action\":\"pass\",\"Package\":%q}\n", packageName)), 0o600); err != nil {
						t.Fatal(err)
					}
				}},
				{name: "unreported subtest skip", mutate: func(t *testing.T, roots []string) {
					path := filepath.Join(roots[0], "shard-0/events.json")
					data, err := os.ReadFile(path)
					if err != nil {
						t.Fatal(err)
					}
					data = append([]byte(fmt.Sprintf("{\"Action\":\"skip\",\"Package\":%q,\"Test\":\"%s/subtest\"}\n", packageName, sortedNames[0])), data...)
					if err := os.WriteFile(path, data, 0o600); err != nil {
						t.Fatal(err)
					}
				}},
			} {
				t.Run(fixture.name, func(t *testing.T) {
					copies := make([]string, topology.count)
					for index, source := range roots {
						copies[index] = filepath.Join(t.TempDir(), "artifact")
						if err := os.CopyFS(copies[index], os.DirFS(source)); err != nil {
							t.Fatal(err)
						}
					}
					if fixture.mutate != nil {
						fixture.mutate(t, copies)
					}
					output := filepath.Join(t.TempDir(), "union.json")
					args := append([]string{aggregate, topology.pkg, output, head}, copies...)
					out, err := exec.Command("bash", args...).CombinedOutput()
					if (err == nil) != fixture.valid {
						t.Fatalf("valid=%v, err=%v\n%s", fixture.valid, err, out)
					}
					if fixture.valid {
						data, err := os.ReadFile(output)
						var result struct {
							Valid      bool  `json:"valid"`
							Discovered int   `json:"discovered_count"`
							Counts     []int `json:"shard_counts"`
						}
						if err != nil || json.Unmarshal(data, &result) != nil || !result.Valid || result.Discovered != topology.count || fmt.Sprint(result.Counts) != fmt.Sprint(wantCounts) {
							t.Fatalf("invalid aggregate %s err=%v", data, err)
						}
					}
				})
			}
		})
	}
}

func TestCIRaceWorkflowSchedulesBrowserPackagesSeparately(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", ".github", "workflows", "race.yml"))
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(data)
	start, end := strings.Index(workflow, "  race-browser:"), strings.Index(workflow, "  race-browser-aggregate:")
	if start < 0 || end <= start || start > strings.Index(workflow, "  race:\n") {
		t.Fatal("browser race packages must use a dedicated early matrix")
	}
	job := workflow[start:end]
	for _, marker := range []string{"needs: [plan, test-binaries]", "max-parallel: 8", "CI_RACE_SHARD_INDEX: ${{ matrix.shard }}", "CI_RACE_HEAD_SHA: ${{ github.sha }}", `scripts/ci-race-test.sh "$PACKAGE" "$SLUG"`, "GLADE_LWC_PLAYWRIGHT_MODULE: ${{ github.workspace }}/lwcruntime/node_modules/playwright", `GLADE_LWC_COMPILE: "1"`, `NO_COLOR: "1"`, "has-browser-packages == 'true'", "include: ${{ fromJSON(needs.plan.outputs.browser-matrix).include }}"} {
		if !strings.Contains(job, marker) {
			t.Errorf("browser matrix missing %q", marker)
		}
	}
	// The browser matrix is plan data: its leg count and shard topology come
	// from the selected dedicated packages through the plan's browser-lanes
	// document, cross-checked against the shard aggregate's counts.
	aggregateScript, err := os.ReadFile("ci-race-shard-aggregate.sh")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(aggregateScript), "./internal/visualforce) shard_count=4 ;;") ||
		!strings.Contains(string(aggregateScript), "./internal/lwc/compile) shard_count=8 ;;") {
		t.Fatal("the shard aggregate browser topology moved; update the plan's BROWSER_LANE_SHARDS with it")
	}
	if !strings.Contains(workflow, "scripts/ci-race-shard-aggregate.sh") || !strings.Contains(workflow, "needs: [plan, race-browser]") {
		t.Fatal("browser shards must require an exact union aggregate")
	}
	for _, name := range []string{"race-browser", "race", "race-push"} {
		if !strings.Contains(workflowJobBlocksText(workflow)[name], "GLADE_LWC_PLAYWRIGHT_MODULE: ${{ github.workspace }}/lwcruntime/node_modules/playwright") {
			t.Errorf("%s must resolve the installed Playwright module", name)
		}
	}
}

// The VM package runs as its own bounded shard matrix: every leg executes
// exactly one planned shard with the lane's test deadline, balanced with the
// raced durations (measured history first, the scaled normal table for the
// rest), the plan's isolated giant tests pinned one per trailing shard, and
// the aggregate proves the complete tested union. The package is pure Go,
// so no binary producer or browser setup may gate or provision these lanes.
func TestCIRacePackageRunnerVMBoundedShardsCompleteUnion(t *testing.T) {
	const packageName = "github.com/glade-sh/glade/internal/vm"
	head := strings.Repeat("d", 40)
	// The plan's isolated giant test plus real rows of the checked-in VM
	// duration table, so the raced history the runner passes to the planner
	// carries measured weights.
	var lanes struct {
		ShardCount    int      `json:"shardCount"`
		IsolatedTests []string `json:"isolatedTests"`
		Include       []struct {
			TestTimeoutSeconds  int `json:"testTimeoutSeconds"`
			BatchTimeoutSeconds int `json:"batchTimeoutSeconds"`
		} `json:"include"`
	}
	if err := json.Unmarshal([]byte(runRacePlan(t, "lanes")), &lanes); err != nil {
		t.Fatal(err)
	}
	names := append([]string{}, lanes.IsolatedTests...)
	for _, name := range []string{"TestAssignmentSaveVMAnonymousAndIsTestAPIRoutes", "TestCPUTimeNativeConformance", "TestDMLScopeAliasActualTestProject", "TestOrgObjectAvailabilityPublicNativeRoutes"} {
		names = append(names, name)
	}
	vmTable := readVMDurationTable(t)
	vmRaced := readRaceHistory(t)["github.com/glade-sh/glade/internal/vm"]
	vmMeasured := make(map[string]raceHistoryRow, len(vmRaced.Tests))
	for _, row := range vmRaced.Tests {
		vmMeasured[row.Name] = row
	}
	root := t.TempDir()
	goCommand := writeRaceFixture(t, "fake-go", `#!/usr/bin/env bash
set -euo pipefail
if [[ "$*" == *"-list ."* ]]; then
  printf '%b' "$CI_RACE_TEST_NAMES"
  printf 'ok   github.com/glade-sh/glade/internal/vm 0.001s\n'
  exit 0
fi
printf 'native %s\n' "$*" >>"$CI_RACE_CALL_LOG"
while IFS= read -r name; do
  [[ -n "$name" ]] || continue
  if [[ "$*" == *"$name"* ]]; then
    printf '{"Action":"pass","Package":"github.com/glade-sh/glade/internal/vm","Test":"%s","Elapsed":0.01}\n' "$name"
  fi
done <<<"$CI_RACE_TEST_NAMES"
printf '{"Action":"pass","Package":"github.com/glade-sh/glade/internal/vm"}\n'
`)
	planner := writeRaceFixture(t, "fake-planner", fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
package=""
tests=""
history=""
isolated=""
while [[ "$#" -gt 0 ]]; do case "$1" in
  --package) package="$2"; shift 2;;
  --shards) [[ "$2" == %d ]]; shift 2;;
  --tests) tests="$2"; shift 2;;
  --history) history="$2"; shift 2;;
  --isolate) isolated="$2"; shift 2;;
  *) exit 91;;
esac; done
[[ -n "$history" && -s "$history" ]] || exit 95
[[ "$isolated" == %q ]]
cp "$history" "$CI_RACE_CALL_LOG.history"
names=(); while IFS= read -r name; do names+=("$name"); done <"$tests"
[[ "${#names[@]}" == %d ]]
printf '{"version":1,"package":"%%s","historyUsed":true,"shards":[' "$package"
for ((index=0; index<%d; index++)); do [[ "$index" == 0 ]] || printf ','; printf '{"index":%%s,"tests":["%%s"],"estimatedDurationMillis":0,"regex":"^(?:%%s)$"}' "$index" "${names[$index]}" "${names[$index]}"; done
printf ']}\n'
`, lanes.ShardCount, strings.Join(lanes.IsolatedTests, ","), len(names), lanes.ShardCount))
	resource := writeRaceFixture(t, "fake-resource", raceResourceFixture())
	git := writeRaceFixture(t, "fake-git", "#!/usr/bin/env bash\nprintf '%s\\n' "+head+"\n")
	roots := make([]string, lanes.ShardCount)
	for index := range roots {
		workdir := filepath.Join(root, fmt.Sprintf("runner-%d", index))
		if err := os.MkdirAll(workdir, 0o700); err != nil {
			t.Fatal(err)
		}
		log := filepath.Join(workdir, "calls.log")
		out, err := runRacePackage(t, workdir, []string{
			"CI_RACE_GO_COMMAND=" + goCommand, "CI_RACE_SHARD_PLANNER=" + planner,
			"CI_RACE_RESOURCE_RUNNER=" + resource, "CI_RACE_CALL_LOG=" + log,
			"CI_RACE_GIT_COMMAND=" + git, "CI_RACE_HEAD_SHA=" + head,
			"CI_RACE_TIMEOUT_COMMAND=" + raceBatchTimeoutFixture(t),
			fmt.Sprintf("CI_RACE_SHARD_INDEX=%d", index),
			fmt.Sprintf("CI_RACE_SHARD_COUNT=%d", lanes.ShardCount),
			"CI_RACE_ISOLATED_TESTS=" + strings.Join(lanes.IsolatedTests, ","),
			fmt.Sprintf("CI_RACE_TEST_TIMEOUT_SECONDS=%d", lanes.Include[index].TestTimeoutSeconds),
			fmt.Sprintf("CI_RACE_BATCH_TIMEOUT_SECONDS=%d", lanes.Include[index].BatchTimeoutSeconds),
			"CI_RACE_TEST_NAMES=" + strings.Join(names, "\n") + "\n",
		}, "./internal/vm", "internal-vm")
		if err != nil {
			t.Fatalf("VM leg %d failed: %v\n%s", index, err, out)
		}
		calls, err := os.ReadFile(log)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(calls), fmt.Sprintf("deadline %d", lanes.Include[index].BatchTimeoutSeconds)) {
			t.Fatalf("VM leg %d did not use the planned batch deadline:\n%s", index, calls)
		}
		if !strings.Contains(string(calls), fmt.Sprintf("native test -json -race -count=1 -timeout=%ds -run", lanes.Include[index].TestTimeoutSeconds)) {
			t.Fatalf("VM leg %d did not use the planned shard deadline:\n%s", index, calls)
		}
		if !strings.Contains(string(calls), fmt.Sprintf("start race-internal-vm-shard-%d", index)) {
			t.Fatalf("VM leg %d did not execute its one assigned shard:\n%s", index, calls)
		}
		roots[index] = filepath.Join(workdir, "ci-artifacts/race/internal-vm")
		// Every leg must plan from the same raced VM duration history.
		history, err := os.ReadFile(filepath.Join(roots[index], "vm-race-history.json"))
		if err != nil {
			t.Fatalf("VM leg %d raced history evidence: %v", index, err)
		}
		var raced struct {
			Tests []struct {
				Name           string `json:"name"`
				DurationMillis int64  `json:"durationMillis"`
			} `json:"tests"`
		}
		if err := json.Unmarshal(history, &raced); err != nil || len(raced.Tests) != len(names) {
			t.Fatalf("VM leg %d raced history = %s err=%v", index, history, err)
		}
		for _, row := range raced.Tests {
			want := vmTable[row.Name] * 4
			if measured, ok := vmMeasured[row.Name]; ok {
				if !measured.Killed {
					want = measured.DurationMillis
				} else if measured.DurationMillis > want {
					want = measured.DurationMillis
				}
			}
			if row.DurationMillis != want {
				t.Fatalf("VM leg %d raced %s to %d ms, want %d", index, row.Name, row.DurationMillis, want)
			}
		}
	}
	aggregate, err := filepath.Abs("ci-race-shard-aggregate.sh")
	if err != nil {
		t.Fatal(err)
	}
	runAggregate := func(output string, roots ...string) (string, error) {
		cmd := exec.Command("bash", append([]string{aggregate, "./internal/vm", output, head}, roots...)...)
		cmd.Env = append(os.Environ(), "CI_RACE_VM_SHARDS="+fmt.Sprint(lanes.ShardCount))
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	output := filepath.Join(root, "union.json")
	if out, err := runAggregate(output, roots...); err != nil {
		t.Fatalf("VM aggregate rejected full coverage: %v\n%s", err, out)
	}
	data, err := os.ReadFile(output)
	var result struct {
		Valid      bool `json:"valid"`
		Discovered int  `json:"discovered_count"`
	}
	if err != nil || json.Unmarshal(data, &result) != nil || !result.Valid || result.Discovered != len(names) {
		t.Fatalf("VM union summary=%s err=%v", data, err)
	}
	// A missing leg must fail the complete-union validation.
	copies := make([]string, lanes.ShardCount)
	for index, source := range roots {
		copies[index] = filepath.Join(t.TempDir(), "artifact")
		if err := os.CopyFS(copies[index], os.DirFS(source)); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.RemoveAll(copies[lanes.ShardCount-1]); err != nil {
		t.Fatal(err)
	}
	brokenOutput := filepath.Join(t.TempDir(), "broken.json")
	broken, err := runAggregate(brokenOutput, copies...)
	if err == nil {
		t.Fatalf("VM aggregate accepted an incomplete shard union:\n%s", broken)
	}
	if !strings.Contains(string(broken), fmt.Sprintf("shard %d", lanes.ShardCount-1)) {
		t.Fatalf("incomplete union must name the missing shard:\n%s", broken)
	}
	verdict, err := os.ReadFile(brokenOutput)
	if err != nil {
		t.Fatalf("incomplete union must still write its verdict document: %v", err)
	}
	var rejected struct {
		Valid    bool     `json:"valid"`
		Verdicts []string `json:"verdicts"`
	}
	if json.Unmarshal(verdict, &rejected) != nil || rejected.Valid || len(rejected.Verdicts) != 1 {
		t.Fatalf("incomplete union verdict = %s err=%v", verdict, err)
	}
	if !strings.Contains(rejected.Verdicts[0], fmt.Sprintf("shard %d", lanes.ShardCount-1)) {
		t.Fatalf("verdict for the missing shard = %q", rejected.Verdicts[0])
	}
}

// A shard runner whose go test fails or times out must still write its
// runner-validation record naming the failing test, so the aggregate
// reports that failure instead of crashing on missing evidence.
func TestCIRaceBrowserShardRunnerRecordsFailOutcomeForAggregate(t *testing.T) {
	const packageName = "github.com/glade-sh/glade/internal/visualforce"
	head := strings.Repeat("e", 40)
	// One row the recorded run measured, so the lane history derivation
	// holds real rows; the failing case stays synthetic.
	names := []string{measuredRaceHistoryName(t, "./internal/visualforce"), "TestCase01", "TestCase02", "TestCase03"}
	root := t.TempDir()
	goCommand := writeRaceFixture(t, "fake-go", `#!/usr/bin/env bash
set -euo pipefail
if [[ "$*" == *"-list ."* ]]; then
  printf '%b' "$CI_RACE_TEST_NAMES"
  printf 'ok   github.com/glade-sh/glade/internal/visualforce 0.001s\n'
  exit 0
fi
printf 'native %s\n' "$*" >>"$CI_RACE_CALL_LOG"
while IFS= read -r name; do
  [[ -n "$name" ]] || continue
  if [[ "$*" == *"$name"* ]]; then
    if [[ "$name" == "${CI_RACE_FAILING_TEST:-}" ]]; then
      printf '{"Action":"run","Package":"github.com/glade-sh/glade/internal/visualforce","Test":"%s"}\n' "$name"
      printf '{"Action":"fail","Package":"github.com/glade-sh/glade/internal/visualforce","Test":"%s"}\n' "$name"
    else
      printf '{"Action":"pass","Package":"github.com/glade-sh/glade/internal/visualforce","Test":"%s","Elapsed":0.01}\n' "$name"
    fi
  fi
done <<<"$CI_RACE_TEST_NAMES"
printf '{"Action":"pass","Package":"github.com/glade-sh/glade/internal/visualforce"}\n'
exit "${CI_RACE_GO_STATUS:-0}"
`)
	planner := writeRaceFixture(t, "fake-planner", racePlannerFixture())
	resource := writeRaceFixture(t, "fake-resource", raceResourceFixture())
	git := writeRaceFixture(t, "fake-git", "#!/usr/bin/env bash\nprintf '%s\\n' "+head+"\n")
	// The planner assigns the sorted discovery to shards in order, so the
	// failing case's shard is its position in the sorted names.
	sorted := append([]string(nil), names...)
	sort.Strings(sorted)
	failingShard := 0
	for index, name := range sorted {
		if name == "TestCase01" {
			failingShard = index
		}
	}
	roots := make([]string, 4)
	for index := range roots {
		workdir := filepath.Join(root, fmt.Sprintf("runner-%d", index))
		if err := os.MkdirAll(workdir, 0o700); err != nil {
			t.Fatal(err)
		}
		env := []string{
			"CI_RACE_GO_COMMAND=" + goCommand, "CI_RACE_SHARD_PLANNER=" + planner,
			"CI_RACE_RESOURCE_RUNNER=" + resource, "CI_RACE_CALL_LOG=" + filepath.Join(workdir, "calls.log"),
			"CI_RACE_GIT_COMMAND=" + git, "CI_RACE_HEAD_SHA=" + head,
			"CI_RACE_TEST_NAMES=" + strings.Join(names, "\n") + "\n", "CI_RACE_TEST_PACKAGE=" + packageName,
			fmt.Sprintf("CI_RACE_SHARD_INDEX=%d", index),
		}
		if index == failingShard {
			env = append(env, "CI_RACE_FAILING_TEST=TestCase01", "CI_RACE_GO_STATUS=31")
		}
		out, err := runRacePackage(t, workdir, env, "./internal/visualforce", "internal-visualforce")
		roots[index] = filepath.Join(workdir, "ci-artifacts/race/internal-visualforce")
		if index != failingShard {
			if err != nil {
				t.Fatalf("clean runner %d failed: %v\n%s", index, err, out)
			}
			continue
		}
		if exit := raceExitCode(err); exit != 31 {
			t.Fatalf("failing runner exit = %d, want the go test status 31:\n%s", exit, out)
		}
		record, err := os.ReadFile(filepath.Join(roots[index], "runner-validation.json"))
		if err != nil {
			t.Fatalf("failing runner wrote no validation record: %v\n%s", err, out)
		}
		var validation struct {
			Outcome string `json:"outcome"`
			Failure struct {
				ExitStatus   int      `json:"exit_status"`
				FailingTests []string `json:"failing_tests"`
				RunningTests []string `json:"running_tests"`
			} `json:"failure"`
		}
		if err := json.Unmarshal(record, &validation); err != nil || validation.Outcome != "fail" ||
			validation.Failure.ExitStatus != 31 || fmt.Sprint(validation.Failure.FailingTests) != fmt.Sprint([]string{"TestCase01"}) {
			t.Fatalf("failing runner validation = %s err=%v", record, err)
		}
	}
	aggregate, err := filepath.Abs("ci-race-shard-aggregate.sh")
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(root, "union.json")
	args := append([]string{aggregate, "./internal/visualforce", output, head}, roots...)
	verdict, err := exec.Command("bash", args...).CombinedOutput()
	if err == nil {
		t.Fatalf("aggregate accepted a failed shard:\n%s", verdict)
	}
	if !strings.Contains(string(verdict), fmt.Sprintf("shard %d", failingShard)) || !strings.Contains(string(verdict), "TestCase01") {
		t.Fatalf("aggregate must name the failed shard and its failing test:\n%s", verdict)
	}
	summary, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("aggregate must still write its verdict document: %v", err)
	}
	var rejected struct {
		Valid    bool     `json:"valid"`
		Verdicts []string `json:"verdicts"`
	}
	if json.Unmarshal(summary, &rejected) != nil || rejected.Valid || len(rejected.Verdicts) != 1 ||
		!strings.Contains(rejected.Verdicts[0], "TestCase01") {
		t.Fatalf("aggregate verdict = %s err=%v", summary, err)
	}
}

// readVMDurationTable loads the checked-in VM duration rows the race plan
// scales for shard balancing.
func readVMDurationTable(t *testing.T) map[string]int64 {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "internal", "vm", "testdata", "shard_durations.json"))
	if err != nil {
		t.Fatal(err)
	}
	var table struct {
		Tests []struct {
			Name           string `json:"name"`
			DurationMillis int64  `json:"durationMillis"`
		} `json:"tests"`
	}
	if err := json.Unmarshal(data, &table); err != nil {
		t.Fatal(err)
	}
	durations := make(map[string]int64, len(table.Tests))
	for _, row := range table.Tests {
		durations[row.Name] = row.DurationMillis
	}
	return durations
}
