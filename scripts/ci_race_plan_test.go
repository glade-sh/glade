package scripts

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// runRacePlan invokes the one race plan authority. The Race workflow, the
// package runner and the Apex aggregate all consume its output, so every
// scheduling rule tested here is a rule the scheduled race route follows.
func runRacePlan(t *testing.T, args ...string) string {
	t.Helper()
	out, err := exec.Command("python3", append([]string{"ci-race-plan.py"}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("ci-race-plan.py %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

func runRacePlanErr(t *testing.T, args ...string) (string, error) {
	t.Helper()
	out, err := exec.Command("python3", append([]string{"ci-race-plan.py"}, args...)...).CombinedOutput()
	return string(out), err
}

const racePlanApexPackage = "github.com/glade-sh/glade/internal/apextest"

type racePlanShardTimeout struct {
	Index              int `json:"index"`
	TestTimeoutSeconds int `json:"test_timeout_seconds"`
}

type racePlanRunner struct {
	Runner              string `json:"runner"`
	ShardIndexes        []int  `json:"shard_indexes"`
	BatchTimeoutSeconds int    `json:"batch_timeout_seconds"`
	StepTimeoutMinutes  int    `json:"step_timeout_minutes"`
	JobTimeoutMinutes   int    `json:"job_timeout_minutes"`
}

type racePlanPartition struct {
	SchemaVersion   int                    `json:"schema_version"`
	Package         string                 `json:"package"`
	Multiplier      int                    `json:"normal_duration_multiplier"`
	ShardCount      int                    `json:"shard_count"`
	DiscoverySHA256 string                 `json:"discovery_sha256"`
	PlanSHA256      string                 `json:"plan_sha256"`
	BinarySHA256    string                 `json:"binary_sha256"`
	BinarySizeBytes int                    `json:"binary_size_bytes"`
	Shards          []racePlanShardTimeout `json:"shards"`
	Runners         []racePlanRunner       `json:"runners"`
}

func racePlanVMConfig(t *testing.T) (shards int) {
	t.Helper()
	data, err := os.ReadFile("ci-race-plan.json")
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		VMLane struct {
			Shards int `json:"shards"`
		} `json:"vmLane"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	return config.VMLane.Shards
}

type raceHistoryRow struct {
	Name           string `json:"name"`
	DurationMillis int64  `json:"durationMillis"`
	Killed         bool   `json:"killed"`
}

type raceHistoryPackage struct {
	Package    string           `json:"package"`
	Tests      []raceHistoryRow `json:"tests"`
	ShardWalls []struct {
		ElapsedSeconds float64 `json:"elapsedSeconds"`
		ExitStatus     int     `json:"exitStatus"`
		Complete       bool    `json:"complete"`
	} `json:"shardWalls"`
}

// readRaceHistory loads the checked-in measured race history keyed by
// package, so tests can derive the same estimates the plan must produce.
func readRaceHistory(t *testing.T) map[string]raceHistoryPackage {
	t.Helper()
	data, err := os.ReadFile("ci-race-history.json")
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Packages []raceHistoryPackage `json:"packages"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	packages := make(map[string]raceHistoryPackage, len(document.Packages))
	for _, entry := range document.Packages {
		packages[entry.Package] = entry
	}
	return packages
}

// The VM lane must stay bounded: one leg per balanced shard plus one leg per
// isolated giant test, each with a test deadline derived from the measured
// race history (the multiplier only for tests the recorded run never
// measured), and a coherent batch, step and job deadline above it.
func TestCIRacePlanLanesBoundVMShardsWithCoherentDeadlines(t *testing.T) {
	output := runRacePlan(t, "lanes")
	var lanes struct {
		ShardCount    int      `json:"shardCount"`
		IsolatedTests []string `json:"isolatedTests"`
		Include       []struct {
			Shard               int `json:"shard"`
			TestTimeoutSeconds  int `json:"testTimeoutSeconds"`
			BatchTimeoutSeconds int `json:"batchTimeoutSeconds"`
			StepMinutes         int `json:"stepMinutes"`
			TimeoutMinutes      int `json:"timeoutMinutes"`
		} `json:"include"`
	}
	if err := json.Unmarshal([]byte(output), &lanes); err != nil {
		t.Fatal(err)
	}
	balanced := racePlanVMConfig(t)
	if len(lanes.Include) != lanes.ShardCount || lanes.ShardCount != balanced+len(lanes.IsolatedTests) {
		t.Fatalf("VM lane has %d legs for %d shards and %d isolated tests", len(lanes.Include), lanes.ShardCount, len(lanes.IsolatedTests))
	}
	if len(lanes.IsolatedTests) == 0 {
		t.Fatal("the recorded race run measured a test whose budget exceeds the balanced shard budget; the lane must isolate it")
	}
	for index, leg := range lanes.Include {
		if leg.Shard != index {
			t.Fatalf("VM leg %d has shard %d", index, leg.Shard)
		}
		if leg.TestTimeoutSeconds < 900 {
			t.Errorf("VM leg %d test timeout = %d below the floor", index, leg.TestTimeoutSeconds)
		}
		if leg.TestTimeoutSeconds > 7200 {
			t.Errorf("VM leg %d test timeout = %d above the shard cap", index, leg.TestTimeoutSeconds)
		}
		if leg.BatchTimeoutSeconds != leg.TestTimeoutSeconds+600 {
			t.Errorf("VM leg %d batch = %d, want test timeout + 600", index, leg.BatchTimeoutSeconds)
		}
		wantStep := (leg.BatchTimeoutSeconds+59)/60 + 5
		if leg.StepMinutes != wantStep {
			t.Errorf("VM leg %d step minutes = %d, want %d", index, leg.StepMinutes, wantStep)
		}
		if leg.TimeoutMinutes != leg.StepMinutes+15 {
			t.Errorf("VM leg %d job minutes = %d, want step + 15", index, leg.TimeoutMinutes)
		}
		if leg.TimeoutMinutes > 355 {
			t.Errorf("VM leg %d job minutes = %d exceeds the GitHub job cap", index, leg.TimeoutMinutes)
		}
	}
	// The isolated tests own the trailing legs with a budget derived from
	// their own measurement, which must exceed every balanced leg's budget.
	balancedBudget := lanes.Include[0].TestTimeoutSeconds
	for index, leg := range lanes.Include[:balanced] {
		if leg.TestTimeoutSeconds != balancedBudget {
			t.Errorf("balanced leg %d budget = %d, want %d", index, leg.TestTimeoutSeconds, balancedBudget)
		}
	}
	history := readRaceHistory(t)
	vmRows := make(map[string]raceHistoryRow, len(history["github.com/glade-sh/glade/internal/vm"].Tests))
	for _, row := range history["github.com/glade-sh/glade/internal/vm"].Tests {
		vmRows[row.Name] = row
	}
	for offset, name := range lanes.IsolatedTests {
		leg := lanes.Include[balanced+offset]
		row, ok := vmRows[name]
		if !ok {
			t.Fatalf("isolated test %s is not in the measured race history", name)
		}
		want := int((row.DurationMillis*5 + 3999) / 4000)
		if want < 900 {
			want = 900
		}
		if leg.TestTimeoutSeconds != want {
			t.Errorf("isolated leg %d budget = %d, want %d from its own measurement", balanced+offset, leg.TestTimeoutSeconds, want)
		}
		if leg.TestTimeoutSeconds <= balancedBudget {
			t.Errorf("isolated leg %d budget = %d does not exceed the balanced budget %d", balanced+offset, leg.TestTimeoutSeconds, balancedBudget)
		}
	}
}

// racePlanFixture writes one canonical discovery file and a matching shard
// plan whose per-shard estimates are supplied by the caller.
func racePlanFixture(t *testing.T, names []string, estimatesMs []int) (dir, discovery, plan string) {
	t.Helper()
	dir = t.TempDir()
	discovery = filepath.Join(dir, "discovery.txt")
	if err := os.WriteFile(discovery, []byte(strings.Join(names, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	shards := make([]map[string]any, len(names))
	for index, name := range names {
		estimate := 0
		if estimatesMs != nil {
			estimate = estimatesMs[index]
		}
		shards[index] = map[string]any{
			"index":                   index,
			"tests":                   []string{name},
			"estimatedDurationMillis": estimate,
			"regex":                   "^(?:" + name + ")$",
		}
	}
	planDocument := map[string]any{"version": 1, "package": racePlanApexPackage, "historyUsed": false, "shards": shards}
	planData, err := json.Marshal(planDocument)
	if err != nil {
		t.Fatal(err)
	}
	plan = filepath.Join(dir, "plan.json")
	if err := os.WriteFile(plan, append(planData, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir, discovery, plan
}

func racePlanSchedule(t *testing.T, discovery, plan string) racePlanPartition {
	t.Helper()
	output := runRacePlan(t, "schedule", discovery, plan, strings.Repeat("a", 64), "1048576")
	var partition racePlanPartition
	if err := json.Unmarshal([]byte(output), &partition); err != nil {
		t.Fatalf("partition JSON: %v\n%s", err, output)
	}
	return partition
}

func TestCIRacePlanScheduleBindsAssignmentsAndDeadlineFloor(t *testing.T) {
	names := make([]string, 16)
	for index := range names {
		names[index] = fmt.Sprintf("TestCase%02d", index)
	}
	_, discovery, plan := racePlanFixture(t, names, nil)
	partition := racePlanSchedule(t, discovery, plan)
	if partition.SchemaVersion != 1 || partition.Package != racePlanApexPackage || partition.Multiplier != 4 {
		t.Fatalf("partition header = %#v", partition)
	}
	if partition.ShardCount != 16 || len(partition.Shards) != 16 {
		t.Fatalf("partition shards = %d, want 16", partition.ShardCount)
	}
	assigned := map[int]bool{}
	runners := partition.Runners
	if len(runners) < 3 {
		t.Fatalf("partition has %d runners, want at least three", len(runners))
	}
	for _, runner := range runners {
		if runner.BatchTimeoutSeconds != len(runner.ShardIndexes)*900+600 {
			t.Errorf("runner %s batch = %d, want %d shard budgets plus reserve", runner.Runner, runner.BatchTimeoutSeconds, len(runner.ShardIndexes)*900)
		}
		wantStep := (runner.BatchTimeoutSeconds+59)/60 + 5
		if runner.StepTimeoutMinutes != wantStep {
			t.Errorf("runner %s step minutes = %d, want %d", runner.Runner, runner.StepTimeoutMinutes, wantStep)
		}
		if runner.JobTimeoutMinutes != runner.StepTimeoutMinutes+15 {
			t.Errorf("runner %s job minutes = %d, want step + 15", runner.Runner, runner.JobTimeoutMinutes)
		}
		for _, index := range runner.ShardIndexes {
			if assigned[index] {
				t.Errorf("shard %d assigned twice", index)
			}
			assigned[index] = true
			if partition.Shards[index].TestTimeoutSeconds != 900 {
				t.Errorf("shard %d timeout = %d, want the 900s floor for a zero estimate", index, partition.Shards[index].TestTimeoutSeconds)
			}
		}
	}
	for index := 0; index < 16; index++ {
		if !assigned[index] {
			t.Errorf("shard %d was not assigned to any runner", index)
		}
	}

	// The workflow matrix entries must carry the same deadlines the
	// partition recorded, so the YAML job and step budgets cannot drift
	// from the plan the runner verifies.
	var matrix struct {
		Include []struct {
			Runner              string `json:"runner"`
			BatchTimeoutSeconds int    `json:"batchTimeoutSeconds"`
			StepMinutes         int    `json:"stepMinutes"`
			TimeoutMinutes      int    `json:"timeoutMinutes"`
		} `json:"include"`
	}
	if err := json.Unmarshal([]byte(runRacePlan(t, "matrix", planPartitionPath(t, discovery, plan))), &matrix); err != nil {
		t.Fatal(err)
	}
	if len(matrix.Include) != len(runners) {
		t.Fatalf("matrix has %d entries, want %d runners", len(matrix.Include), len(runners))
	}
	for index, entry := range matrix.Include {
		if entry.Runner != runners[index].Runner || entry.BatchTimeoutSeconds != runners[index].BatchTimeoutSeconds ||
			entry.StepMinutes != runners[index].StepTimeoutMinutes || entry.TimeoutMinutes != runners[index].JobTimeoutMinutes {
			t.Errorf("matrix entry %d = %#v, want runner %#v", index, entry, runners[index])
		}
	}
}

func planPartitionPath(t *testing.T, discovery, plan string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "partition.json")
	if err := os.WriteFile(path, []byte(runRacePlan(t, "schedule", discovery, plan, strings.Repeat("a", 64), "1048576")), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCIRacePlanScheduleBalancesShardsByRacedEstimate(t *testing.T) {
	names := make([]string, 16)
	estimates := make([]int, 16)
	for index := range names {
		names[index] = fmt.Sprintf("TestCase%02d", index)
		estimates[index] = 1_000_000
	}
	// One heavy shard: its raced estimate dominates, so the schedule must
	// balance the remaining load and keep every job under the deadline cap.
	estimates[0] = 1_400_000
	_, discovery, plan := racePlanFixture(t, names, estimates)
	partition := racePlanSchedule(t, discovery, plan)
	// The plan's estimates are already race-scaled (the history subcommand
	// applies the multiplier), so the budget adds headroom only once.
	if got, want := partition.Shards[0].TestTimeoutSeconds, 1_750; got != want {
		t.Fatalf("heavy shard timeout = %d, want %d", got, want)
	}
	for index := 1; index < 16; index++ {
		if partition.Shards[index].TestTimeoutSeconds != 1_250 {
			t.Fatalf("light shard %d timeout = %d, want 1250", index, partition.Shards[index].TestTimeoutSeconds)
		}
	}
	loads := map[string]int{}
	for _, runner := range partition.Runners {
		for _, index := range runner.ShardIndexes {
			loads[runner.Runner] += partition.Shards[index].TestTimeoutSeconds
		}
		if runner.JobTimeoutMinutes > 355 {
			t.Errorf("runner %s job minutes = %d exceeds the cap", runner.Runner, runner.JobTimeoutMinutes)
		}
	}
	if len(loads) < 3 {
		t.Fatalf("heavy plan used %d runners, want at least three", len(loads))
	}
	var minLoad, maxLoad int
	for position, load := range sortedRacePlanLoads(loads) {
		if position == 0 {
			minLoad, maxLoad = load, load
		} else if load < minLoad {
			minLoad = load
		} else if load > maxLoad {
			maxLoad = load
		}
	}
	if maxLoad-minLoad > partition.Shards[0].TestTimeoutSeconds {
		t.Fatalf("assignment is unbalanced: loads %v", loads)
	}
}

func sortedRacePlanLoads(loads map[string]int) []int {
	keys := make([]string, 0, len(loads))
	for key := range loads {
		keys = append(keys, key)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	values := make([]int, 0, len(keys))
	for _, key := range keys {
		values = append(values, loads[key])
	}
	return values
}

// A single test whose own raced budget exceeds the per-shard cap must fail
// the schedule naming that test: no shard count can schedule around it.
func TestCIRacePlanScheduleRejectsOversizedShards(t *testing.T) {
	names := make([]string, 16)
	estimates := make([]int, 16)
	for index := range names {
		names[index] = fmt.Sprintf("TestCase%02d", index)
	}
	// 6,100,000 ms of raced estimate is 7,625 s of budget after headroom,
	// above the 7,200 s cap, so the schedule must name TestCase00.
	estimates[0] = 6_100_000
	_, discovery, plan := racePlanFixture(t, names, estimates)
	out, err := runRacePlanErr(t, "schedule", discovery, plan, strings.Repeat("a", 64), "1048576")
	if err == nil {
		t.Fatalf("oversized shard accepted:\n%s", out)
	}
	if !strings.Contains(out, "TestCase00") {
		t.Fatalf("oversized shard failure must name the unshardable test:\n%s", out)
	}
}

// A package whose total raced budget exceeds the cap must still schedule:
// the plan spreads it across shards, so the total never lands in one shard.
func TestCIRacePlanScheduleSpreadsPackageTotalOverCapIntoShards(t *testing.T) {
	names := make([]string, 16)
	estimates := make([]int, 16)
	for index := range names {
		names[index] = fmt.Sprintf("TestCase%02d", index)
		estimates[index] = 1_500_000
	}
	_, discovery, plan := racePlanFixture(t, names, estimates)
	// The package total is 24,000 s of raced estimate, far above the 7,200 s
	// per-shard cap, yet every shard must stay under it.
	partition := racePlanSchedule(t, discovery, plan)
	for index, shard := range partition.Shards {
		if shard.TestTimeoutSeconds > 7200 {
			t.Fatalf("shard %d budget = %d, want at most the 7200s cap", index, shard.TestTimeoutSeconds)
		}
		if shard.TestTimeoutSeconds != 1_875 {
			t.Fatalf("shard %d budget = %d, want 1875s of raced estimate plus headroom", index, shard.TestTimeoutSeconds)
		}
	}
}

// Hosted runners cost the nightly run nothing extra while every dispatch
// waits on its slowest job, so the schedule must spread the shards over the
// runner count that minimizes the slowest runner's job instead of packing
// them onto the fewest runners that fit the job cap.
func TestCIRacePlanScheduleMinimizesSlowestRunnerJob(t *testing.T) {
	slowestJobMinutes := func(t *testing.T, partition racePlanPartition) int {
		t.Helper()
		slowest := 0
		for _, runner := range partition.Runners {
			if runner.JobTimeoutMinutes > slowest {
				slowest = runner.JobTimeoutMinutes
			}
		}
		return slowest
	}
	t.Run("sixteen equal shards get one runner each", func(t *testing.T) {
		names := make([]string, 16)
		estimates := make([]int, 16)
		for index := range names {
			names[index] = fmt.Sprintf("TestCase%02d", index)
			estimates[index] = 1_000_000
		}
		_, discovery, plan := racePlanFixture(t, names, estimates)
		partition := racePlanSchedule(t, discovery, plan)
		if len(partition.Runners) != 16 {
			t.Fatalf("plan used %d runners for 16 equal shards, want one shard per runner", len(partition.Runners))
		}
		assigned := map[int]string{}
		for _, runner := range partition.Runners {
			if len(runner.ShardIndexes) != 1 {
				t.Fatalf("runner %s holds %v, want exactly one shard", runner.Runner, runner.ShardIndexes)
			}
			if other, ok := assigned[runner.ShardIndexes[0]]; ok {
				t.Fatalf("shard %d is held by runners %s and %s", runner.ShardIndexes[0], other, runner.Runner)
			}
			assigned[runner.ShardIndexes[0]] = runner.Runner
		}
		if len(assigned) != 16 {
			t.Fatalf("runners cover %d distinct shards, want 16", len(assigned))
		}
		// The slowest job is one shard budget (1250s) plus the reserves:
		// batch 1850s, step ceil(batch/60)+5, job step+15.
		if got, want := slowestJobMinutes(t, partition), (1250+600+59)/60+5+15; got != want {
			t.Fatalf("slowest job = %d minutes, want %d (one shard budget plus reserves)", got, want)
		}
	})
	t.Run("a dominant shard stops adding runners at its own budget", func(t *testing.T) {
		names := make([]string, 16)
		estimates := make([]int, 16)
		for index := range names {
			names[index] = fmt.Sprintf("TestCase%02d", index)
		}
		// One shard's own 4000s budget exceeds every balanced load the
		// remaining floor-budget shards can form, so the fifth runner puts
		// it alone; no further runner can reduce the slowest job below it.
		estimates[0] = 3_200_000
		_, discovery, plan := racePlanFixture(t, names, estimates)
		partition := racePlanSchedule(t, discovery, plan)
		if len(partition.Runners) != 5 {
			t.Fatalf("plan used %d runners, want the five that isolate the dominant shard", len(partition.Runners))
		}
		dominant := false
		for _, runner := range partition.Runners {
			if len(runner.ShardIndexes) == 1 && runner.ShardIndexes[0] == 0 {
				dominant = true
				if got, want := runner.JobTimeoutMinutes, (4000+600+59)/60+5+15; got != want {
					t.Fatalf("dominant runner job = %d minutes, want %d", got, want)
				}
			}
		}
		if !dominant {
			t.Fatalf("no runner holds the dominant shard alone: %#v", partition.Runners)
		}
		if got, want := slowestJobMinutes(t, partition), (4000+600+59)/60+5+15; got != want {
			t.Fatalf("slowest job = %d minutes, want %d (the dominant shard alone plus reserves)", got, want)
		}
	})
	t.Run("a plan past the runner ceiling keeps the fewest minimal runners", func(t *testing.T) {
		names := make([]string, 20)
		estimates := make([]int, 20)
		for index := range names {
			names[index] = fmt.Sprintf("TestCase%02d", index)
			estimates[index] = 1_000_000
		}
		_, discovery, plan := racePlanFixture(t, names, estimates)
		partition := racePlanSchedule(t, discovery, plan)
		// Twenty shards cannot reach one shard per runner under the
		// sixteen-runner ceiling, so the minimum slowest load is two shard
		// budgets and the plan keeps the first count that reaches it.
		if len(partition.Runners) != 10 {
			t.Fatalf("plan used %d runners, want the first ten that pair up the shards", len(partition.Runners))
		}
		for _, runner := range partition.Runners {
			if len(runner.ShardIndexes) > 2 {
				t.Fatalf("runner %s holds %v, want at most two shards", runner.Runner, runner.ShardIndexes)
			}
		}
		if got, want := slowestJobMinutes(t, partition), (2500+600+59)/60+5+15; got != want {
			t.Fatalf("slowest job = %d minutes, want %d (two shard budgets plus reserves)", got, want)
		}
	})
	t.Run("a plan no ceiling count fits fails naming the ceiling", func(t *testing.T) {
		names := make([]string, 40)
		estimates := make([]int, 40)
		for index := range names {
			names[index] = fmt.Sprintf("TestCase%02d", index)
			estimates[index] = 5_760_000
		}
		_, discovery, plan := racePlanFixture(t, names, estimates)
		out, err := runRacePlanErr(t, "schedule", discovery, plan, strings.Repeat("a", 64), "1048576")
		if err == nil {
			t.Fatalf("unfittable plan accepted:\n%s", out)
		}
		if !strings.Contains(out, "up to 16 runners") {
			t.Fatalf("failure must name the runner ceiling:\n%s", out)
		}
	})
}

func racePlanApexNormalHistory(t *testing.T) map[string]int {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "internal", "apextest", "testdata", "shard_durations.json"))
	if err != nil {
		t.Fatal(err)
	}
	var table struct {
		Tests []struct {
			Name           string `json:"name"`
			DurationMillis int    `json:"durationMillis"`
		} `json:"tests"`
	}
	if err := json.Unmarshal(data, &table); err != nil {
		t.Fatal(err)
	}
	if len(table.Tests) == 0 {
		t.Fatal("checked-in Apex normal history is empty")
	}
	history := make(map[string]int, len(table.Tests))
	for _, row := range table.Tests {
		history[row.Name] = row.DurationMillis
	}
	return history
}

// The raced history prefers each test's measured race duration, so the
// multiplier fallback binds only the tests the recorded run never measured:
// those keep the checked-in normal duration scaled by the documented
// multiplier, and a discovery with no coverage at all fails closed instead of
// planning budgets with no weights behind them.
func TestCIRacePlanHistoryScalesUnmeasuredTestsFromTheNormalTable(t *testing.T) {
	history := racePlanApexNormalHistory(t)
	raceRows := readRaceHistory(t)[racePlanApexPackage].Tests
	measured := make(map[string]bool, len(raceRows))
	for _, row := range raceRows {
		measured[row.Name] = true
	}
	names := make([]string, 0, 8)
	for name := range history {
		if !measured[name] && history[name] > 0 && len(names) < 8 {
			names = append(names, name)
		}
	}
	if len(names) < 8 {
		t.Fatal("the recorded race run measured the whole normal table; repoint this fallback test at unmeasured rows")
	}
	sort.Strings(names)
	discovery := filepath.Join(t.TempDir(), "discovery.txt")
	if err := os.WriteFile(discovery, []byte(strings.Join(names, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var scaled struct {
		Version  int  `json:"version"`
		Complete bool `json:"complete"`
		Tests    []struct {
			Name           string `json:"name"`
			DurationMillis int    `json:"durationMillis"`
		} `json:"tests"`
	}
	if err := json.Unmarshal([]byte(runRacePlan(t, "history", discovery)), &scaled); err != nil {
		t.Fatal(err)
	}
	if scaled.Version != 1 || !scaled.Complete || len(scaled.Tests) != len(names) {
		t.Fatalf("raced history header = %#v", scaled)
	}
	for _, row := range scaled.Tests {
		if want := history[row.Name] * 4; row.DurationMillis != want {
			t.Errorf("%s raced to %d ms, want the unmeasured fallback %d", row.Name, row.DurationMillis, want)
		}
	}
	// A discovery with no table coverage must fail closed instead of
	// planning budgets with no weights behind them.
	unknown := filepath.Join(t.TempDir(), "discovery.txt")
	if err := os.WriteFile(unknown, []byte("TestUnknownToTheTable\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := runRacePlanErr(t, "history", unknown); err == nil {
		t.Fatalf("unknown discovery accepted:\n%s", out)
	}
}

// racePlanScaledHistoryFixture writes one canonical discovery file and a
// matching race-scaled history document for the shard-count derivation.
func racePlanScaledHistoryFixture(t *testing.T, names []string, scaledMillis int) (dir, discovery, history string) {
	t.Helper()
	dir = t.TempDir()
	discovery = filepath.Join(dir, "discovery.txt")
	if err := os.WriteFile(discovery, []byte(strings.Join(names, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	rows := make([]map[string]any, 0, len(names))
	for _, name := range names {
		rows = append(rows, map[string]any{"name": name, "durationMillis": scaledMillis})
	}
	historyData, err := json.Marshal(map[string]any{"version": 1, "package": racePlanApexPackage, "complete": true, "tests": rows})
	if err != nil {
		t.Fatal(err)
	}
	history = filepath.Join(dir, "race-history.json")
	if err := os.WriteFile(history, append(historyData, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir, discovery, history
}

func racePlanShardCount(t *testing.T, discovery, history string) int {
	t.Helper()
	var document struct {
		ShardCount int `json:"shard_count"`
	}
	if err := json.Unmarshal([]byte(runRacePlan(t, "shards", discovery, history)), &document); err != nil {
		t.Fatal(err)
	}
	return document.ShardCount
}

// The plan owns the Apex shard count: at least sixteen shards, and more when
// the suite's raced total no longer fits under the per-shard cap at sixteen.
// A single test over the cap fails the derivation with its name instead.
func TestCIRacePlanShardsCommandDerivesCountFromScaledHistory(t *testing.T) {
	t.Run("floor", func(t *testing.T) {
		names := make([]string, 20)
		for index := range names {
			names[index] = fmt.Sprintf("TestCase%02d", index)
		}
		_, discovery, history := racePlanScaledHistoryFixture(t, names, 1_500_000)
		if got := racePlanShardCount(t, discovery, history); got != 16 {
			t.Fatalf("shard count = %d, want the sixteen-shard floor", got)
		}
	})
	t.Run("missing tests weigh the median", func(t *testing.T) {
		// Four discovered tests have no history row; they must count at the
		// history median, not at zero, so the derived count stays bounded.
		names := make([]string, 20)
		for index := range names {
			names[index] = fmt.Sprintf("TestCase%02d", index)
		}
		_, discovery, history := racePlanScaledHistoryFixture(t, names[:16], 1_500_000)
		if got := racePlanShardCount(t, discovery, history); got != 16 {
			t.Fatalf("shard count = %d, want sixteen with median-weighted gaps", got)
		}
	})
	t.Run("grows past the floor", func(t *testing.T) {
		names := make([]string, 40)
		for index := range names {
			names[index] = fmt.Sprintf("TestCase%02d", index)
		}
		// Forty tests at 2,000 s of raced estimate give an LPT bound over the
		// cap at sixteen shards; the derivation must grow to twenty-two.
		_, discovery, history := racePlanScaledHistoryFixture(t, names, 2_000_000)
		if got := racePlanShardCount(t, discovery, history); got != 22 {
			t.Fatalf("shard count = %d, want 22 shards for a total over the cap", got)
		}
	})
	t.Run("single test over the cap fails with its name", func(t *testing.T) {
		names := []string{"TestASlowSeed", "TestBeta", "TestDelta", "TestEpsilon", "TestEta", "TestGamma", "TestIota", "TestKappa", "TestLambda", "TestMu", "TestNu", "TestOmicron", "TestPi", "TestRho", "TestTheta", "TestZeta"}
		_, discovery, history := racePlanScaledHistoryFixture(t, names, 1_000)
		if err := os.WriteFile(history, []byte(strings.Replace(string(mustRead(t, history)), `"durationMillis":1000`, `"durationMillis":6100000`, 1)), 0o600); err != nil {
			t.Fatal(err)
		}
		out, err := runRacePlanErr(t, "shards", discovery, history)
		if err == nil {
			t.Fatalf("single oversized test accepted:\n%s", out)
		}
		if !strings.Contains(out, "TestASlowSeed") {
			t.Fatalf("oversized test failure must name the test:\n%s", out)
		}
	})
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func writeRacePlanPartition(t *testing.T, names []string, estimates []int) string {
	t.Helper()
	dir, discovery, plan := racePlanFixture(t, names, estimates)
	if err := os.WriteFile(filepath.Join(dir, "partition.json"), []byte(runRacePlan(t, "schedule", discovery, plan, strings.Repeat("a", 64), "1048576")), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := runRacePlanErr(t, "verify", dir); err != nil {
		t.Fatalf("produced partition does not verify: %v\n%s", err, out)
	}
	return dir
}

// verify re-derives the whole partition from its own discovery and plan, so
// any drift between the published manifest and the policy is rejected before
// a runner spends hours executing it.
func TestCIRacePlanVerifyRejectsTamperedPartitions(t *testing.T) {
	names := make([]string, 16)
	for index := range names {
		names[index] = fmt.Sprintf("TestCase%02d", index)
	}
	mutations := map[string]func(t *testing.T, root string){
		"tightened shard deadline": func(t *testing.T, root string) {
			path := filepath.Join(root, "partition.json")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(strings.Replace(string(data), `"test_timeout_seconds":900`, `"test_timeout_seconds":600`, 1)), 0o600); err != nil {
				t.Fatal(err)
			}
		},
		"tightened batch deadline": func(t *testing.T, root string) {
			path := filepath.Join(root, "partition.json")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(strings.Replace(string(data), `"batch_timeout_seconds":1500`, `"batch_timeout_seconds":100`, 1)), 0o600); err != nil {
				t.Fatal(err)
			}
		},
		"removed runner": func(t *testing.T, root string) {
			path := filepath.Join(root, "partition.json")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var partition map[string]any
			if err := json.Unmarshal(data, &partition); err != nil {
				t.Fatal(err)
			}
			partition["runners"] = partition["runners"].([]any)[:2]
			updated, err := json.Marshal(partition)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, append(updated, '\n'), 0o600); err != nil {
				t.Fatal(err)
			}
		},
		"swapped multiplier": func(t *testing.T, root string) {
			path := filepath.Join(root, "partition.json")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(strings.Replace(string(data), `"normal_duration_multiplier":4`, `"normal_duration_multiplier":2`, 1)), 0o600); err != nil {
				t.Fatal(err)
			}
		},
		"tampered discovery": func(t *testing.T, root string) {
			if err := os.WriteFile(filepath.Join(root, "discovery.txt"), []byte("TestIntruder\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		},
		"tampered plan union": func(t *testing.T, root string) {
			path := filepath.Join(root, "plan.json")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(strings.Replace(string(data), `"TestCase00"`, `"TestCase99"`, 1)), 0o600); err != nil {
				t.Fatal(err)
			}
		},
		"missing partition": func(t *testing.T, root string) {
			if err := os.Remove(filepath.Join(root, "partition.json")); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := writeRacePlanPartition(t, names, nil)
			mutate(t, root)
			if out, err := runRacePlanErr(t, "verify", root); err == nil {
				t.Fatalf("tampered partition verified:\n%s", out)
			}
		})
	}
}
