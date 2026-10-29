package apextest

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// GLADE_APEXTEST_SHARD=i/n deterministically splits the top-level tests of
// this package into n shards and selects shard i (zero-based) by narrowing
// -test.run before testing.M runs. Every top-level test belongs to exactly
// one shard, so the n shards together run exactly the unsharded test set;
// TestShardPartitionMatchesUnshardedTestSet proves that against the test
// binary's own -test.list registry. Shards are balanced with the measured
// durations checked in at testdata/shard_durations.json; tests missing from
// that table get the table's median duration.
const shardSelectorEnv = "GLADE_APEXTEST_SHARD"

const shardDurationTablePath = "testdata/shard_durations.json"

const shardTestPackage = "github.com/glade-sh/glade/internal/apextest"

var shardTestNamePattern = regexp.MustCompile(`^Test[A-Za-z0-9_]*$`)

type shardDurationEntry struct {
	Name           string `json:"name"`
	DurationMillis int64  `json:"durationMillis"`
}

type shardDurationTable struct {
	Version int                  `json:"version"`
	Package string               `json:"package"`
	Source  string               `json:"source"`
	Tests   []shardDurationEntry `json:"tests"`
}

// applyShardSelection narrows -test.run to the selected shard. It is called
// by TestMain before m.Run and does nothing unless GLADE_APEXTEST_SHARD is
// set. Any malformed input fails closed so a shard never silently runs the
// wrong test set.
func applyShardSelection() error {
	spec := os.Getenv(shardSelectorEnv)
	if spec == "" {
		return nil
	}
	index, count, err := parseShardSpec(spec)
	if err != nil {
		return err
	}
	names, err := discoverShardTestNames()
	if err != nil {
		return err
	}
	durations, err := loadShardDurationTable(shardDurationTablePath)
	if err != nil {
		return err
	}
	selected := shardAssignment(names, durations, count)[index]
	if !flag.Parsed() {
		flag.Parse()
	}
	runFlag := flag.Lookup("test.run")
	if runFlag == nil {
		return fmt.Errorf("test.run flag is not registered")
	}
	pattern, err := shardRunPattern(selected, runFlag.Value.String())
	if err != nil {
		return err
	}
	return runFlag.Value.Set(pattern)
}

func parseShardSpec(spec string) (index, count int, err error) {
	indexText, countText, ok := strings.Cut(spec, "/")
	if ok {
		index, err = strconv.Atoi(indexText)
		if err == nil {
			count, err = strconv.Atoi(countText)
		}
	}
	if !ok || err != nil || index < 0 || count < 1 || index >= count || count > 4096 {
		return 0, 0, fmt.Errorf("%s=%q: want i/n with 0 <= i < n <= 4096", shardSelectorEnv, spec)
	}
	return index, count, nil
}

// discoverShardTestNames lists this binary's own top-level tests, so the
// assignment input is the testing framework's registry rather than a
// hand-maintained list. The child runs with the selector env removed and
// -test.list only lists: it runs no tests and touches no shared state.
func discoverShardTestNames() ([]string, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("locate test binary: %w", err)
	}
	command := exec.Command(executable, "-test.list", "^Test")
	environment := make([]string, 0, len(os.Environ()))
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, shardSelectorEnv+"=") {
			environment = append(environment, entry)
		}
	}
	command.Env = environment
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("list top-level tests: %w: %s", err, stderr.String())
	}
	var names []string
	seen := make(map[string]bool)
	for _, line := range strings.Split(stdout.String(), "\n") {
		name := strings.TrimSpace(line)
		if name == "" {
			continue
		}
		if !shardTestNamePattern.MatchString(name) {
			return nil, fmt.Errorf("unexpected -test.list line %q", line)
		}
		if seen[name] {
			return nil, fmt.Errorf("duplicate discovered test %q", name)
		}
		seen[name] = true
		names = append(names, name)
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("no top-level tests discovered")
	}
	sort.Strings(names)
	return names, nil
}

func loadShardDurationTable(path string) (map[string]int64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("shard duration table: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var table shardDurationTable
	if err := decoder.Decode(&table); err != nil {
		return nil, fmt.Errorf("shard duration table: %w", err)
	}
	if table.Version != 1 || table.Package != shardTestPackage {
		return nil, fmt.Errorf("shard duration table: version %d package %q, want version 1 package %q", table.Version, table.Package, shardTestPackage)
	}
	durations := make(map[string]int64, len(table.Tests))
	for _, entry := range table.Tests {
		if !shardTestNamePattern.MatchString(entry.Name) || entry.DurationMillis < 0 {
			return nil, fmt.Errorf("shard duration table: invalid entry %q (%d ms)", entry.Name, entry.DurationMillis)
		}
		if _, exists := durations[entry.Name]; exists {
			return nil, fmt.Errorf("shard duration table: duplicate entry %q", entry.Name)
		}
		durations[entry.Name] = entry.DurationMillis
	}
	return durations, nil
}

// shardAssignment deterministically partitions names into shardCount bins:
// longest measured duration first, each test onto the least-loaded shard.
// Names without a measured duration weigh the table's median so new tests
// land in balanced bins without a table refresh.
func shardAssignment(names []string, durations map[string]int64, shardCount int) [][]string {
	fallback := shardMedianDuration(durations)
	type weightedName struct {
		name   string
		weight int64
	}
	items := make([]weightedName, 0, len(names))
	for _, name := range names {
		weight, measured := durations[name]
		if !measured {
			weight = fallback
		}
		items = append(items, weightedName{name: name, weight: weight})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].weight != items[j].weight {
			return items[i].weight > items[j].weight
		}
		return items[i].name < items[j].name
	})
	bins := make([][]string, shardCount)
	loads := make([]int64, shardCount)
	for _, item := range items {
		target := 0
		for i := range bins {
			if len(bins[i]) == 0 {
				target = i
				break
			}
		}
		if len(bins[target]) != 0 {
			for i := 1; i < shardCount; i++ {
				if loads[i] < loads[target] {
					target = i
				}
			}
		}
		bins[target] = append(bins[target], item.name)
		loads[target] += item.weight
	}
	for i := range bins {
		sort.Strings(bins[i])
	}
	return bins
}

func shardMedianDuration(durations map[string]int64) int64 {
	if len(durations) == 0 {
		return 1
	}
	values := make([]int64, 0, len(durations))
	for _, value := range durations {
		values = append(values, value)
	}
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	// Sub-millisecond measurements legitimately round to zero. An unknown
	// family still needs positive scheduling weight to spread across bins.
	return max(int64(1), values[(len(values)-1)/2])
}

// shardRunPattern anchors the selection as an exact alternation. An existing
// -test.run narrows further: each alternative's top-level segment filters the
// shard's names, preserving its subtest segments and Go's regexp boundaries.
func shardRunPattern(selected []string, existing string) (string, error) {
	var patterns []string
	// Like testing's -run matcher, split only outside bracket expressions and
	// parentheses. Each top-level alternative has its own subtest filters.
	for _, branch := range shardRegexpParts(existing, '|') {
		parts := shardRegexpParts(branch, '/')
		filter, err := regexp.Compile(parts[0])
		if err != nil {
			return "", fmt.Errorf("invalid -run pattern %q: %w", existing, err)
		}
		var quoted []string
		for _, name := range selected {
			if filter.MatchString(name) {
				quoted = append(quoted, regexp.QuoteMeta(name))
			}
		}
		pattern := "^(?:" + strings.Join(quoted, "|") + ")$"
		if len(parts) > 1 {
			pattern += "/" + strings.Join(parts[1:], "/")
		}
		patterns = append(patterns, pattern)
	}
	return strings.Join(patterns, "|"), nil
}

func shardRegexpParts(pattern string, separator byte) []string {
	var parts []string
	brackets, parentheses, start := 0, 0, 0
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '\\':
			i++
		case '[':
			brackets++
		case ']':
			brackets = max(0, brackets-1)
		case '(':
			if brackets == 0 {
				parentheses++
			}
		case ')':
			if brackets == 0 {
				parentheses--
			}
		default:
			if pattern[i] == separator && brackets == 0 && parentheses == 0 {
				parts = append(parts, pattern[start:i])
				start = i + 1
			}
		}
	}
	return append(parts, pattern[start:])
}

// TestShardPartitionMatchesUnshardedTestSet proves every shard split runs
// exactly the unsharded test set: the same discovery TestMain uses (the test
// binary's own -test.list registry) partitioned by the same assignment, with
// no test dropped, duplicated, or invented. It also keeps the checked-in
// duration table honest against the live registry.
func TestShardPartitionMatchesUnshardedTestSet(t *testing.T) {
	names, err := discoverShardTestNames()
	if err != nil {
		t.Fatal(err)
	}
	all := make(map[string]bool, len(names))
	for _, name := range names {
		all[name] = true
	}
	if !all["TestShardPartitionMatchesUnshardedTestSet"] {
		t.Fatal("discovery does not see the running test itself")
	}
	if len(names) < 1000 {
		t.Fatalf("discovered only %d top-level tests; discovery is broken", len(names))
	}
	durations, err := loadShardDurationTable(shardDurationTablePath)
	if err != nil {
		t.Fatal(err)
	}
	for name := range durations {
		if !all[name] {
			t.Errorf("duration table entry %q is not a current top-level test", name)
		}
	}
	for _, shardCount := range []int{1, 2, 5, 16} {
		assigned := make(map[string]int, len(names))
		bins := shardAssignment(names, durations, shardCount)
		if len(bins) != shardCount {
			t.Fatalf("shardAssignment returned %d bins, want %d", len(bins), shardCount)
		}
		for index, bin := range bins {
			for _, name := range bin {
				if previous, duplicated := assigned[name]; duplicated {
					t.Fatalf("n=%d: %s is in shards %d and %d", shardCount, name, previous, index)
				}
				if !all[name] {
					t.Fatalf("n=%d: shard %d contains unknown test %q", shardCount, index, name)
				}
				assigned[name] = index
			}
		}
		if len(assigned) != len(names) {
			for _, name := range names {
				if _, ok := assigned[name]; !ok {
					t.Errorf("n=%d: %s is in no shard", shardCount, name)
				}
			}
			t.Fatalf("n=%d: shards cover %d of %d tests", shardCount, len(assigned), len(names))
		}
	}
}

func TestShardSelectionIsDeterministicAndBalanced(t *testing.T) {
	names := []string{"TestA", "TestB", "TestC", "TestD", "TestE", "TestF"}
	durations := map[string]int64{
		"TestA": 600, "TestB": 500, "TestC": 300,
		"TestD": 200, "TestE": 200, "TestF": 100,
	}
	first := shardAssignment(names, durations, 2)
	second := shardAssignment([]string{"TestF", "TestE", "TestD", "TestC", "TestB", "TestA"}, durations, 2)
	for i := range first {
		if strings.Join(first[i], ",") != strings.Join(second[i], ",") {
			t.Fatalf("assignment depends on input order: %v vs %v", first, second)
		}
	}
	// LPT on these weights: shard0 = A+D+E (1000), shard1 = B+C+F (900).
	if strings.Join(first[0], ",") != "TestA,TestD,TestE" || strings.Join(first[1], ",") != "TestB,TestC,TestF" {
		t.Fatalf("unexpected balanced assignment: %v", first)
	}
	// Unmeasured tests weigh the median so they spread rather than pile up.
	withNew := shardAssignment(append(names, "TestNew"), durations, 2)
	total := len(withNew[0]) + len(withNew[1])
	if total != 7 {
		t.Fatalf("new test lost: %v", withNew)
	}

	pattern, err := shardRunPattern([]string{"TestA", "TestB"}, "")
	if err != nil || pattern != "^(?:TestA|TestB)$" {
		t.Fatalf("pattern = %q, %v", pattern, err)
	}
	pattern, err = shardRunPattern([]string{"TestA", "TestB"}, "TestB/62\\.0")
	if err != nil || pattern != "^(?:TestB)$/62\\.0" {
		t.Fatalf("composed pattern = %q, %v", pattern, err)
	}
	if _, err := shardRunPattern([]string{"TestA"}, "("); err == nil {
		t.Fatal("invalid -run pattern was accepted")
	}

	if _, _, err := parseShardSpec("3/2"); err == nil {
		t.Fatal("out-of-range shard spec was accepted")
	}
	if _, _, err := parseShardSpec("1"); err == nil {
		t.Fatal("malformed shard spec was accepted")
	}
	index, count, err := parseShardSpec("0/1")
	if err != nil || index != 0 || count != 1 {
		t.Fatalf("parseShardSpec(0/1) = %d/%d, %v", index, count, err)
	}
}

func TestShardFilterFixtureA(t *testing.T) {
	for _, name := range []string{"a", "b"} {
		t.Run(name, func(t *testing.T) {})
	}
}

func TestShardFilterFixtureB(t *testing.T) {
	for _, name := range []string{"a", "b"} {
		t.Run(name, func(t *testing.T) {})
	}
}

func TestShardSelectionPreservesGoRunFilters(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		pattern string
		want    []string
	}{
		{"^TestShardFilterFixtureA$/a|^TestShardFilterFixtureB$/b", []string{"TestShardFilterFixtureA/a", "TestShardFilterFixtureB/b"}},
		{"^TestShardFilterFixture[AB/]$/a", []string{"TestShardFilterFixtureA/a", "TestShardFilterFixtureB/a"}},
		{"^(TestShardFilterFixtureA|TestShardFilterFixtureB)$/b", []string{"TestShardFilterFixtureA/b", "TestShardFilterFixtureB/b"}},
	} {
		t.Run(tc.pattern, func(t *testing.T) {
			command := exec.Command(executable, "-test.v", "-test.run", tc.pattern)
			for _, entry := range os.Environ() {
				if !strings.HasPrefix(entry, shardSelectorEnv+"=") {
					command.Env = append(command.Env, entry)
				}
			}
			command.Env = append(command.Env, shardSelectorEnv+"=0/1")
			output, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("sharded child: %v\n%s", err, output)
			}
			var got []string
			for _, line := range strings.Split(string(output), "\n") {
				if name, ok := strings.CutPrefix(line, "=== RUN   "); ok && strings.Contains(name, "/") {
					got = append(got, name)
				}
			}
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("sharded runs=%v; want=%v\n%s", got, tc.want, output)
			}
		})
	}
}

func TestShardRunPatternIntersectsEveryAlternative(t *testing.T) {
	pattern, err := shardRunPattern([]string{"TestShardFilterFixtureA"}, "^TestShardFilterFixtureA$/a|^TestShardFilterFixtureB$/b")
	if err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(executable, "-test.v", "-test.run", pattern)
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, shardSelectorEnv+"=") {
			command.Env = append(command.Env, entry)
		}
	}
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("filtered child: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "=== RUN   TestShardFilterFixtureA/a\n") || strings.Contains(string(output), "=== RUN   TestShardFilterFixtureB") {
		t.Fatalf("run filter escaped the selected shard:\n%s", output)
	}
}
