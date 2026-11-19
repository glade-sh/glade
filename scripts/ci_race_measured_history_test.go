package scripts

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// writeHistoryLane writes one lane directory holding test2json events and an
// optional resource wall, the shape every race lane artifact uploads.
func writeHistoryLane(t *testing.T, root, lane, events, wall string) string {
	t.Helper()
	dir := filepath.Join(root, lane)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if events != "" {
		if err := os.WriteFile(filepath.Join(dir, "events.json"), []byte(events), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if wall != "" {
		if err := os.WriteFile(filepath.Join(dir, "resource.json"), []byte(wall), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func runRaceHistory(t *testing.T, args ...string) (string, error) {
	t.Helper()
	out, err := exec.Command("python3", append([]string{"ci-race-history.py"}, args...)...).CombinedOutput()
	return string(out), err
}

const historyLaneEvents = `{"Time":"2026-11-19T14:00:00.000000000Z","Action":"run","Package":"github.com/glade-sh/glade/internal/demo","Test":"TestMeasured"}
{"Time":"2026-11-19T14:00:02.500000000Z","Action":"pass","Package":"github.com/glade-sh/glade/internal/demo","Test":"TestMeasured","Elapsed":2.5}
{"Time":"2026-11-19T14:00:02.600000000Z","Action":"run","Package":"github.com/glade-sh/glade/internal/demo","Test":"TestKilled"}
{"Time":"2026-11-19T14:00:04.000000000Z","Action":"run","Package":"github.com/glade-sh/glade/internal/demo","Test":"TestKilled/sub"}
{"Time":"2026-11-19T14:00:05.000000000Z","Action":"pass","Package":"github.com/glade-sh/glade/internal/demo","Test":"TestKilled/sub","Elapsed":1}
`

const historyLaneEventsComplete = `{"Time":"2026-11-19T15:00:00.000000000Z","Action":"run","Package":"github.com/glade-sh/glade/internal/demo","Test":"TestOther"}
{"Time":"2026-11-19T15:00:01.000000000Z","Action":"pass","Package":"github.com/glade-sh/glade/internal/demo","Test":"TestOther","Elapsed":1}
`

// The generator records measured durations from terminal events, killed
// tests at their elapsed time at the lane's last recorded event, complete
// shard walls from the lanes whose every started test terminated, and its
// source run; regenerating the same artifacts writes the same bytes.
func TestCIRaceHistoryRecordsMeasuredKilledAndWallsDeterministically(t *testing.T) {
	root := t.TempDir()
	writeHistoryLane(t, root, "lane-a", historyLaneEvents,
		`{"schema_version":1,"lane":"race-demo-lane-a","elapsed_seconds":5.5,"exit_status":1,"user_seconds":9,"system_seconds":1,"max_rss_kb":1024}`)
	writeHistoryLane(t, root, "lane-b", historyLaneEventsComplete,
		`{"schema_version":1,"lane":"race-demo-lane-b","elapsed_seconds":1.2,"exit_status":0,"user_seconds":1.2,"system_seconds":0,"max_rss_kb":512}`)
	output := filepath.Join(t.TempDir(), "history.json")
	if out, err := runRaceHistory(t, "44541836806", root, output); err != nil {
		t.Fatalf("ci-race-history.py failed: %v\n%s", err, out)
	}
	var document struct {
		SchemaVersion int    `json:"schema_version"`
		SourceRunID   int    `json:"source_run_id"`
		SourceRunURL  string `json:"source_run_url"`
		Packages      []struct {
			Package string `json:"package"`
			Tests   []struct {
				Name           string `json:"name"`
				DurationMillis int64  `json:"durationMillis"`
				Killed         bool   `json:"killed"`
			} `json:"tests"`
			ShardWalls []struct {
				ElapsedSeconds float64 `json:"elapsedSeconds"`
				ExitStatus     int     `json:"exitStatus"`
				Complete       bool    `json:"complete"`
			} `json:"shardWalls"`
		} `json:"packages"`
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("history JSON: %v\n%s", err, data)
	}
	if document.SchemaVersion != 1 || document.SourceRunID != 44541836806 ||
		document.SourceRunURL != "https://github.com/glade-sh/glade/actions/runs/44541836806" {
		t.Fatalf("history header = %#v", document)
	}
	if len(document.Packages) != 1 || document.Packages[0].Package != "github.com/glade-sh/glade/internal/demo" {
		t.Fatalf("history packages = %#v", document.Packages)
	}
	package0 := document.Packages[0]
	rows := make(map[string]struct {
		DurationMillis int64
		Killed         bool
	}, len(package0.Tests))
	for _, row := range package0.Tests {
		rows[row.Name] = struct {
			DurationMillis int64
			Killed         bool
		}{row.DurationMillis, row.Killed}
	}
	if len(rows) != 3 {
		t.Fatalf("history rows = %#v", rows)
	}
	if rows["TestMeasured"].DurationMillis != 2500 || rows["TestMeasured"].Killed {
		t.Fatalf("measured row = %#v, want 2500ms complete", rows["TestMeasured"])
	}
	if rows["TestOther"].DurationMillis != 1000 || rows["TestOther"].Killed {
		t.Fatalf("second lane row = %#v, want 1000ms complete", rows["TestOther"])
	}
	// The killed test counts at least its elapsed time at the kill: its run
	// to the lane's last recorded event, subtest events included.
	if rows["TestKilled"].DurationMillis != 2400 || !rows["TestKilled"].Killed {
		t.Fatalf("killed row = %#v, want 2400ms killed lower bound", rows["TestKilled"])
	}
	if len(package0.ShardWalls) != 2 || package0.ShardWalls[0].Complete || package0.ShardWalls[0].ElapsedSeconds != 5.5 ||
		package0.ShardWalls[0].ExitStatus != 1 || !package0.ShardWalls[1].Complete || package0.ShardWalls[1].ElapsedSeconds != 1.2 {
		t.Fatalf("history walls = %#v, want the killed lane incomplete and the passing lane complete", package0.ShardWalls)
	}
	first, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if out, err := runRaceHistory(t, "44541836806", root, output); err != nil {
		t.Fatalf("regeneration failed: %v\n%s", err, out)
	}
	second, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatal("regenerating the same run must write identical bytes")
	}
}

// A lane whose resource evidence did not survive the batch kill still
// records its rows; the wall is absent rather than wrong.
func TestCIRaceHistoryRecordsBatchKilledLanesWithoutWalls(t *testing.T) {
	root := t.TempDir()
	writeHistoryLane(t, root, "lane-killed", `{"Time":"2026-11-19T14:00:00.000000000Z","Action":"run","Package":"github.com/glade-sh/glade/internal/demo","Test":"TestSlow"}
{"Time":"2026-11-19T14:00:00.100000000Z","Action":"pass","Package":"github.com/glade-sh/glade/internal/demo","Test":"TestSlow/sub","Elapsed":0.1}
`, "")
	output := filepath.Join(t.TempDir(), "history.json")
	if out, err := runRaceHistory(t, "44541836806", root, output); err != nil {
		t.Fatalf("ci-race-history.py failed: %v\n%s", err, out)
	}
	var document struct {
		Packages []struct {
			Tests []struct {
				Name           string `json:"name"`
				DurationMillis int64  `json:"durationMillis"`
				Killed         bool   `json:"killed"`
			} `json:"tests"`
			ShardWalls []json.RawMessage `json:"shardWalls"`
		} `json:"packages"`
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("history JSON: %v\n%s", err, data)
	}
	rows := document.Packages[0].Tests
	if len(rows) != 1 || rows[0].Name != "TestSlow" || rows[0].DurationMillis != 100 || !rows[0].Killed {
		t.Fatalf("batch-killed rows = %#v, want TestSlow 100ms killed", rows)
	}
	if len(document.Packages[0].ShardWalls) != 0 {
		t.Fatalf("a killed lane without resource evidence must record no wall: %#v", document.Packages[0].ShardWalls)
	}
}

// The raced history prefers measurements, scales only unmeasured tests, and
// leaves tests unknown to both sources to the planner's median.
func TestCIRacePlanHistoryPrefersMeasuredRaceDurations(t *testing.T) {
	history := readRaceHistory(t)
	apexRows := make(map[string]raceHistoryRow, len(history[racePlanApexPackage].Tests))
	for _, row := range history[racePlanApexPackage].Tests {
		apexRows[row.Name] = row
	}
	var measuredName, killedName string
	for _, row := range history[racePlanApexPackage].Tests {
		if row.Killed && killedName == "" {
			killedName = row.Name
		}
		if !row.Killed && row.DurationMillis > 1000 && measuredName == "" {
			measuredName = row.Name
		}
	}
	if measuredName == "" || killedName == "" {
		t.Fatal("the checked-in race history must hold complete and killed rows")
	}
	table := readJSONRows(t, filepath.Join("..", "internal", "apextest", "testdata", "shard_durations.json"), racePlanApexPackage)
	var scaledName string
	for name := range table {
		if _, measured := apexRows[name]; !measured && scaledName == "" {
			scaledName = name
		}
	}
	if scaledName == "" {
		t.Fatal("the checked-in normal table must hold a test the race run never measured")
	}
	discovery := filepath.Join(t.TempDir(), "discovery.txt")
	discoveryNames := []string{measuredName, killedName, scaledName, "TestNeverRecorded"}
	sort.Strings(discoveryNames)
	if err := os.WriteFile(discovery, []byte(strings.Join(discoveryNames, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var raced struct {
		Tests []struct {
			Name           string `json:"name"`
			DurationMillis int64  `json:"durationMillis"`
		} `json:"tests"`
	}
	if err := json.Unmarshal([]byte(runRacePlan(t, "history", discovery)), &raced); err != nil {
		t.Fatal(err)
	}
	rows := make(map[string]int64, len(raced.Tests))
	for _, row := range raced.Tests {
		rows[row.Name] = row.DurationMillis
	}
	if got := rows[measuredName]; got != apexRows[measuredName].DurationMillis {
		t.Errorf("measured %s raced to %d ms, want its measurement %d", measuredName, got, apexRows[measuredName].DurationMillis)
	}
	// A killed row is a lower bound; it keeps the multiplier estimate
	// wherever that is larger.
	wantKilled := apexRows[killedName].DurationMillis
	if scaled := table[killedName] * 4; scaled > wantKilled {
		wantKilled = scaled
	}
	if got := rows[killedName]; got != wantKilled {
		t.Errorf("killed %s raced to %d ms, want %d", killedName, got, wantKilled)
	}
	if got := rows[scaledName]; got != table[scaledName]*4 {
		t.Errorf("unmeasured %s raced to %d ms, want the table scaled to %d", scaledName, got, table[scaledName]*4)
	}
	if _, known := rows["TestNeverRecorded"]; known {
		t.Error("a test unknown to both sources must be left to the planner median")
	}
}

func readJSONRows(t *testing.T, path, packageName string) map[string]int64 {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var table struct {
		Package string `json:"package"`
		Tests   []struct {
			Name           string `json:"name"`
			DurationMillis int64  `json:"durationMillis"`
		} `json:"tests"`
	}
	if err := json.Unmarshal(data, &table); err != nil {
		t.Fatal(err)
	}
	if table.Package != packageName {
		t.Fatalf("%s holds package %s", path, table.Package)
	}
	rows := make(map[string]int64, len(table.Tests))
	for _, row := range table.Tests {
		rows[row.Name] = row.DurationMillis
	}
	return rows
}

// The early lane runs every shard sequentially in one job behind a batch
// derived from the average complete measured shard wall.
func TestCIRacePlanEarlyLaneBudgetComesFromMeasuredWalls(t *testing.T) {
	var lanes struct {
		Include []struct {
			Package             string `json:"package"`
			BatchTimeoutSeconds int    `json:"batchTimeoutSeconds"`
			StepMinutes         int    `json:"stepMinutes"`
			TimeoutMinutes      int    `json:"timeoutMinutes"`
		} `json:"include"`
	}
	if err := json.Unmarshal([]byte(runRacePlan(t, "early-lanes")), &lanes); err != nil {
		t.Fatal(err)
	}
	if len(lanes.Include) != 1 || lanes.Include[0].Package != "./internal/gladecli" {
		t.Fatalf("early lane legs = %#v", lanes.Include)
	}
	leg := lanes.Include[0]
	history := readRaceHistory(t)["github.com/glade-sh/glade/internal/gladecli"]
	var walls []float64
	for _, wall := range history.ShardWalls {
		if wall.Complete {
			walls = append(walls, wall.ElapsedSeconds)
		}
	}
	if len(walls) == 0 {
		t.Fatal("the checked-in race history holds no complete early-lane wall")
	}
	total := 0.0
	for _, wall := range walls {
		total += wall
	}
	// The batch scales by the lane's shard count, which must mirror the
	// shard lanes the runner actually creates for the package.
	config, err := os.ReadFile("ci-race-plan.json")
	if err != nil {
		t.Fatal(err)
	}
	var planConfig struct {
		EarlyLane struct {
			Package string `json:"package"`
			Shards  int    `json:"shards"`
		} `json:"earlyLane"`
	}
	if err := json.Unmarshal(config, &planConfig); err != nil {
		t.Fatal(err)
	}
	shards := planConfig.EarlyLane.Shards
	if planConfig.EarlyLane.Package != "./internal/gladecli" || shards < 1 || shards > 16 {
		t.Fatalf("early lane config = %#v", planConfig.EarlyLane)
	}
	script, err := os.ReadFile("ci-race-test.sh")
	if err != nil {
		t.Fatal(err)
	}
	laneNames := "lane_names=(" + strings.Join(
		func() []string {
			names := make([]string, 0, shards)
			for index := 0; index < shards; index++ {
				names = append(names, fmt.Sprintf("shard-%d", index))
			}
			return names
		}(), " ") + ")"
	if !strings.Contains(string(script), laneNames) {
		t.Fatalf("the runner's gladecli lane contract %q moved; update the early lane shard count with it", laneNames)
	}
	want := int(math.Ceil(total/float64(len(walls))*float64(shards)*1.25)) + 600
	if leg.BatchTimeoutSeconds != want {
		t.Errorf("early lane batch = %d, want %d from %d measured walls", leg.BatchTimeoutSeconds, want, len(walls))
	}
	if wantStep := (leg.BatchTimeoutSeconds+59)/60 + 5; leg.StepMinutes != wantStep {
		t.Errorf("early lane step = %d, want %d", leg.StepMinutes, wantStep)
	}
	if leg.TimeoutMinutes != leg.StepMinutes+15 {
		t.Errorf("early lane job minutes = %d, want step + 15", leg.TimeoutMinutes)
	}
	if leg.TimeoutMinutes > 355 {
		t.Errorf("early lane job minutes = %d exceeds the GitHub job cap", leg.TimeoutMinutes)
	}
	// The lane's own provisioning (npm ci, toolchain) plus cleanup still
	// needs the old flat floor even when the measured walls shrink.
	if leg.TimeoutMinutes < 80 {
		t.Errorf("early lane job minutes = %d below the provisioning, test and cleanup floor", leg.TimeoutMinutes)
	}
	// The batch that killed the lane was the flat 3600s default; the
	// measured budget must clear it.
	if leg.BatchTimeoutSeconds <= 3600 {
		t.Errorf("early lane batch = %d does not clear the flat 3600s batch that killed run 44541836806", leg.BatchTimeoutSeconds)
	}
}

// The independent lane gets the multiplier on its measured warm normal wall
// with the shard floor; the repository guard has never completed a race run
// on a cold scan cache, so no race measurement exists for it.
func TestCIRacePlanIndependentLaneBudgetFromMeasuredWarmWall(t *testing.T) {
	var lanes struct {
		Include []struct {
			Package            string `json:"package"`
			TestTimeoutSeconds int    `json:"testTimeoutSeconds"`
			StepMinutes        int    `json:"stepMinutes"`
			TimeoutMinutes     int    `json:"timeoutMinutes"`
		} `json:"include"`
	}
	if err := json.Unmarshal([]byte(runRacePlan(t, "independent-lanes")), &lanes); err != nil {
		t.Fatal(err)
	}
	if len(lanes.Include) != 1 || lanes.Include[0].Package != "./internal/repoguard" {
		t.Fatalf("independent lane legs = %#v", lanes.Include)
	}
	leg := lanes.Include[0]
	if leg.TestTimeoutSeconds < 900 {
		t.Errorf("independent lane test timeout = %d below the shard floor", leg.TestTimeoutSeconds)
	}
	if wantStep := (leg.TestTimeoutSeconds+59)/60 + 5; leg.StepMinutes != wantStep {
		t.Errorf("independent lane step = %d, want %d", leg.StepMinutes, wantStep)
	}
	if leg.TimeoutMinutes != leg.StepMinutes+15 {
		t.Errorf("independent lane job minutes = %d, want step + 15", leg.TimeoutMinutes)
	}
}

// Heavy generic lanes whose measured batch passes the flat default get their
// own deadline; lanes under it keep the default and are omitted.
func TestCIRacePlanGenericBatchesCoverOnlyLanesPastTheDefault(t *testing.T) {
	var batches map[string]int
	if err := json.Unmarshal([]byte(runRacePlan(t, "generic-batches")), &batches); err != nil {
		t.Fatal(err)
	}
	if len(batches) == 0 {
		t.Fatal("the recorded run measured a heavy generic lane past the flat default; the plan must cover it")
	}
	for _, lane := range []string{"./internal/testdaemon", "./internal/playground"} {
		if _, ok := batches[lane]; ok {
			t.Errorf("%s measured under the flat default and must keep it", lane)
		}
	}
	for lane, batch := range batches {
		if batch <= 3600 {
			t.Errorf("%s batch = %d does not exceed the flat default", lane, batch)
		}
	}
}

// Lanes without a normal duration table plan from their measured race rows
// alone; tests the run never measured are left to the planner median.
func TestCIRacePlanLaneHistoryEmitsOnlyMeasuredRows(t *testing.T) {
	history := readRaceHistory(t)["github.com/glade-sh/glade/internal/gladecli"]
	if len(history.Tests) == 0 {
		t.Fatal("the checked-in race history holds no gladecli rows")
	}
	measured := history.Tests[0].Name
	unmeasured := "TestNeverRecordedAnywhere"
	discovery := filepath.Join(t.TempDir(), "discovery.txt")
	if err := os.WriteFile(discovery, []byte(strings.Join([]string{measured, unmeasured}, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var raced struct {
		Package string `json:"package"`
		Tests   []struct {
			Name           string `json:"name"`
			DurationMillis int64  `json:"durationMillis"`
		} `json:"tests"`
	}
	if err := json.Unmarshal([]byte(runRacePlan(t, "lane-history", "internal-gladecli", discovery)), &raced); err != nil {
		t.Fatal(err)
	}
	if raced.Package != "github.com/glade-sh/glade/internal/gladecli" || len(raced.Tests) != 1 {
		t.Fatalf("lane history = %#v, want only the measured row", raced)
	}
	if raced.Tests[0].Name != measured {
		t.Fatalf("lane history row = %s, want %s", raced.Tests[0].Name, measured)
	}
}

// The browser lane matrices come from the plan: only the selected
// dedicated browser packages schedule legs, non-browser dedicated packages
// schedule none, and the shard topology mirrors the shard aggregate the
// union validation uses, so a narrowed dispatched run schedules no shard of
// an unselected package while a scheduled run keeps every leg.
func TestCIRacePlanBrowserLanesEmitOnlySelectedPackages(t *testing.T) {
	full := strings.TrimSpace(runShell(t, "ci-race-packages.sh", "full"))
	lines := strings.Split(strings.TrimSpace(runShell(t, "ci-race-packages.sh", "partition", full)), "\n")
	if len(lines) != 4 {
		t.Fatalf("partition lines = %#v", lines)
	}
	var dedicated []string
	if err := json.Unmarshal([]byte(lines[3]), &dedicated); err != nil {
		t.Fatalf("dedicated packages: %v\n%s", err, lines[3])
	}
	want := []string{"./internal/apextest", "./internal/visualforce", "./internal/lwc/compile", "./internal/vm"}
	if len(dedicated) != len(want) {
		t.Fatalf("dedicated packages = %#v, want exactly the browser and non-browser dedicated set", dedicated)
	}
	for index, name := range want {
		if dedicated[index] != name {
			t.Fatalf("dedicated packages = %#v, want %q at %d", dedicated, name, index)
		}
	}
	// The plan's browser topology must mirror the shard aggregate that
	// validates the union of the browser shards.
	aggregateScript, err := os.ReadFile("ci-race-shard-aggregate.sh")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(aggregateScript), "./internal/visualforce) shard_count=4 ;;") ||
		!strings.Contains(string(aggregateScript), "./internal/lwc/compile) shard_count=8 ;;") {
		t.Fatal("the shard aggregate browser topology moved; update the plan's BROWSER_LANE_SHARDS with it")
	}
	writePackages := func(t *testing.T, packages string) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), "packages.json")
		if err := os.WriteFile(path, []byte(packages), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	var lanes struct {
		Runners struct {
			Include []struct {
				Package string `json:"package"`
				Slug    string `json:"slug"`
				Shard   int    `json:"shard"`
			} `json:"include"`
		} `json:"runners"`
		Aggregate struct {
			Include []struct {
				Package string `json:"package"`
				Slug    string `json:"slug"`
				Shards  int    `json:"shards"`
			} `json:"include"`
		} `json:"aggregate"`
	}
	if err := json.Unmarshal([]byte(runRacePlan(t, "browser-lanes", writePackages(t, lines[3]))), &lanes); err != nil {
		t.Fatal(err)
	}
	if len(lanes.Runners.Include) != 12 || len(lanes.Aggregate.Include) != 2 {
		t.Fatalf("full browser matrices = %d runner legs, %d aggregate legs, want 12 and 2",
			len(lanes.Runners.Include), len(lanes.Aggregate.Include))
	}
	for _, leg := range lanes.Runners.Include[:4] {
		if leg.Package != "./internal/visualforce" || leg.Slug != "internal-visualforce" ||
			leg.Shard < 0 || leg.Shard > 3 {
			t.Fatalf("visualforce leg = %#v, want one of the four shards", leg)
		}
	}
	for _, leg := range lanes.Runners.Include[4:] {
		if leg.Package != "./internal/lwc/compile" || leg.Slug != "internal-lwc-compile" ||
			leg.Shard < 0 || leg.Shard > 7 {
			t.Fatalf("lwc compile leg = %#v, want one of the eight shards", leg)
		}
	}
	if lanes.Aggregate.Include[0].Shards != 4 || lanes.Aggregate.Include[1].Shards != 8 {
		t.Fatalf("aggregate shard counts = %#v, want 4 then 8", lanes.Aggregate.Include)
	}
	// Non-browser dedicated packages schedule no browser leg, so a run
	// narrowed to them skips both browser jobs instead of running empty
	// shards.
	if err := json.Unmarshal([]byte(runRacePlan(t, "browser-lanes",
		writePackages(t, `["./internal/apextest","./internal/vm"]`))), &lanes); err != nil {
		t.Fatal(err)
	}
	if len(lanes.Runners.Include) != 0 || len(lanes.Aggregate.Include) != 0 {
		t.Fatalf("non-browser dedicated packages = %#v %#v, want empty matrices",
			lanes.Runners.Include, lanes.Aggregate.Include)
	}
	// An entry outside the dedicated partition fails the plan instead of
	// silently dropping browser coverage, as do a duplicate and a non-list.
	for name, packages := range map[string]string{
		"unknown package": `["./internal/notapackage"]`,
		"duplicate":       `["./internal/visualforce","./internal/visualforce"]`,
		"non-list":        `{"package":"./internal/visualforce"}`,
	} {
		if out, err := runRacePlanErr(t, "browser-lanes", writePackages(t, packages)); err == nil {
			t.Errorf("%s must fail the browser lane plan, got %s", name, out)
		}
	}
}

// A dispatched run may narrow the race to requested packages, validated
// against the manifest: an unknown package fails the run, and the partition
// covers exactly the requested subset.
func TestCIRacePackagesSelectAndPartitionSubsets(t *testing.T) {
	full := strings.TrimSpace(runShell(t, "ci-race-packages.sh", "full"))
	var fullList []string
	if err := json.Unmarshal([]byte(full), &fullList); err != nil {
		t.Fatalf("full packages: %v\n%s", err, full)
	}
	if len(fullList) < 4 {
		t.Fatalf("full manifest = %s", full)
	}
	selected, err := runShellErr(t, "ci-race-packages.sh", "select", full, "./internal/repoguard,./internal/vm,./internal/repoguard")
	if err != nil {
		t.Fatalf("select failed: %v\n%s", err, selected)
	}
	if strings.TrimSpace(selected) != `["./internal/repoguard","./internal/vm"]` {
		t.Fatalf("select = %s", selected)
	}
	if _, err := runShellErr(t, "ci-race-packages.sh", "select", full, "./internal/repoguard,./internal/notapackage"); err == nil {
		t.Fatal("an unknown package must fail the selection")
	}
	lines := strings.Split(strings.TrimSpace(runShell(t, "ci-race-packages.sh", "partition", `["./internal/repoguard","./internal/vm"]`)), "\n")
	if len(lines) != 4 {
		t.Fatalf("partition lines = %#v", lines)
	}
	if lines[0] != "[]" || lines[1] != `["./internal/repoguard"]` || lines[2] != "[]" || lines[3] != `["./internal/vm"]` {
		t.Fatalf("partition = %#v", lines)
	}
	if _, err := runShellErr(t, "ci-race-packages.sh", "partition", `["./internal/notapackage"]`); err == nil {
		t.Fatal("a partition of unknown packages must fail")
	}
}

func runShell(t *testing.T, args ...string) string {
	t.Helper()
	out, err := runShellErr(t, args...)
	if err != nil {
		t.Fatalf("%s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
	return out
}

// measuredRaceHistoryName picks one recorded row of the checked-in race
// history for a package, so a runner test's synthetic discovery still holds
// the measured row its lane history requires. Deriving the name from the
// recorded history keeps the fixtures following every refresh instead of
// pinning one test name that a refresh could drop.
func measuredRaceHistoryName(t *testing.T, pkg string) string {
	t.Helper()
	rows := readRaceHistory(t)["github.com/glade-sh/glade/"+strings.TrimPrefix(pkg, "./")].Tests
	if len(rows) == 0 {
		t.Fatalf("the checked-in race history holds no rows for %s", pkg)
	}
	return rows[0].Name
}

func runShellErr(t *testing.T, args ...string) (string, error) {
	t.Helper()
	out, err := exec.Command("bash", args...).CombinedOutput()
	return string(out), err
}

// The generic lane shard counts in the plan config mirror the lane counts
// the runner creates, so a change to one must be a change to both.
func TestCIRaceGenericLaneShardCountsMatchTheRunner(t *testing.T) {
	data, err := os.ReadFile("ci-race-plan.json")
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		GenericLanes map[string]struct {
			Shards int `json:"shards"`
		} `json:"genericLanes"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	script, err := os.ReadFile("ci-race-test.sh")
	if err != nil {
		t.Fatal(err)
	}
	body := string(script)
	if !strings.Contains(body, "lane_names=(group-0 group-1 group-2 group-3 ordinary-0 ordinary-1 ordinary-2 ordinary-3 ordinary-4)") {
		t.Fatal("the playground lane_names contract moved; update this test with it")
	}
	if !strings.Contains(body, "lane_names=(shard-0 shard-1 shard-2 shard-3 shard-4 shard-5 shard-6 shard-7)") {
		t.Fatal("the eight-lane lane_names contract moved; update this test with it")
	}
	if !strings.Contains(body, "lane_names=(shard-0 shard-1 shard-2 shard-3)") {
		t.Fatal("the four-lane lane_names contract moved; update this test with it")
	}
	wantLanes := map[string]int{
		"./internal/server":     8,
		"./internal/testdaemon": 4,
		"./internal/playground": 9,
	}
	for lane, want := range wantLanes {
		if config.GenericLanes[lane].Shards != want {
			t.Errorf("%s config shards = %d, want %d runner lanes", lane, config.GenericLanes[lane].Shards, want)
		}
	}
}

// The Race workflow wires the measured budgets: the dispatch input narrows
// the run, the independent lane restores the scan cache it depends on, the
// VM lane carries the plan's shard count and isolated tests, and the generic
// lane reads its batch from the plan.
func TestCIRaceWorkflowWiresMeasuredLaneBudgets(t *testing.T) {
	workflow := raceWorkflowText(t)
	jobs := workflowJobBlocksText(workflow)
	if !strings.Contains(workflowHeader(workflow), "default: 'all'") {
		t.Fatal("the race workflow must expose the packages dispatch input")
	}
	classify := workflowStepBlockText(jobs["plan"], "      - name: Classify race packages")
	if !strings.Contains(classify, "scripts/ci-race-packages.sh select") {
		t.Fatal("the plan job must validate the dispatch selection against the manifest")
	}
	if !strings.Contains(classify, "RACE_PACKAGES_INPUT: ${{ github.event_name == 'workflow_dispatch' && inputs.packages || '' }}") {
		t.Fatal("the dispatch selection must only apply to dispatched runs")
	}
	independent := jobs["race-independent"]
	if !strings.Contains(independent, "Restore Go build cache") ||
		!strings.Contains(independent, "path: ~/.cache/go-build") ||
		!strings.Contains(independent, "go-build-cache-v1-${{ runner.os }}-${{ runner.arch }}-1.26.11-ci-test-${{ hashFiles('go.sum') }}-") {
		t.Fatal("the independent lane must restore the persisted Go build cache family that carries the repository scan records")
	}
	if !strings.Contains(independent, "CI_RACE_TEST_TIMEOUT_SECONDS: ${{ matrix.testTimeoutSeconds }}") {
		t.Fatal("the independent lane must run behind its measured test budget")
	}
	vmJob := jobs["race-vm"]
	if !strings.Contains(vmJob, "CI_RACE_SHARD_COUNT: ${{ needs.plan.outputs.vm-shard-count }}") ||
		!strings.Contains(vmJob, "CI_RACE_ISOLATED_TESTS: ${{ needs.plan.outputs.vm-isolated-tests }}") {
		t.Fatal("the VM lane must carry the plan's shard count and isolated tests")
	}
	aggregate := jobs["race-vm-aggregate"]
	if !strings.Contains(aggregate, "VM_SHARD_COUNT: ${{ needs.plan.outputs.vm-shard-count }}") ||
		!strings.Contains(aggregate, "CI_RACE_VM_SHARDS=") {
		t.Fatal("the VM aggregate must validate every planned shard")
	}
	raceJob := jobs["race"]
	if !strings.Contains(raceJob, "CI_RACE_BATCH_TIMEOUT_SECONDS: ${{ fromJSON(needs.plan.outputs.generic-batches)[matrix.package] || 3600 }}") {
		t.Fatal("the generic race job must read its batch deadline from the plan")
	}
	earlyJob := jobs["race-early"]
	if !strings.Contains(earlyJob, "timeout-minutes: ${{ matrix.timeoutMinutes }}") ||
		!strings.Contains(earlyJob, "CI_RACE_BATCH_TIMEOUT_SECONDS: ${{ matrix.batchTimeoutSeconds }}") {
		t.Fatal("the early lane must run behind its measured batch and job deadlines")
	}
	binaries := jobs["test-binaries"]
	if !strings.Contains(binaries, "has-binary-packages == 'true'") {
		t.Fatal("the binary producer must run only when a selected package consumes a shared binary")
	}
	// A job-level if cannot read the matrix context, so the plan emits the
	// browser matrices holding only the selected dedicated browser packages;
	// a narrowed run schedules no shard of an unselected package.
	lanesStep := workflowStepBlockText(jobs["plan"], "      - name: Plan bounded race lanes")
	if !strings.Contains(jobs["plan"], "browser-matrix: ${{ steps.lanes.outputs.browser-matrix }}") ||
		!strings.Contains(jobs["plan"], "browser-aggregate-matrix: ${{ steps.lanes.outputs.browser-aggregate-matrix }}") ||
		!strings.Contains(jobs["plan"], "has-browser-packages: ${{ steps.lanes.outputs.has-browser-packages }}") {
		t.Fatal("the plan job must publish the browser matrices and their gate")
	}
	if !strings.Contains(lanesStep, "scripts/ci-race-plan.py browser-lanes") ||
		!strings.Contains(lanesStep, "DEDICATED_PACKAGES: ${{ steps.classify.outputs.dedicated-packages }}") {
		t.Fatal("the plan job must derive the browser matrices from the selected dedicated packages")
	}
	browserJob := jobs["race-browser"]
	if !strings.Contains(browserJob, "has-browser-packages == 'true'") ||
		!strings.Contains(browserJob, "include: ${{ fromJSON(needs.plan.outputs.browser-matrix).include }}") {
		t.Fatal("the browser lane must gate on the plan and schedule only the selected packages' legs")
	}
	browserAggregate := jobs["race-browser-aggregate"]
	if !strings.Contains(browserAggregate, "has-browser-packages == 'true'") ||
		!strings.Contains(browserAggregate, "include: ${{ fromJSON(needs.plan.outputs.browser-aggregate-matrix).include }}") {
		t.Fatal("the browser aggregate must validate only the selected packages' shards")
	}
}

func raceWorkflowText(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", ".github", "workflows", "race.yml"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
