// Package testshard splits one Go test package's top-level tests into n
// deterministic shards so a CI lane can run the shards as separate processes.
//
// A package opts in from its TestMain:
//
//	if err := testshard.Apply("GLADE_VM_SHARD", "testdata/shard_durations.json", "github.com/glade-sh/glade/internal/vm"); err != nil {
//		fmt.Fprintln(os.Stderr, err)
//		os.Exit(2)
//	}
//
// With the selector env set to i/n, Apply narrows -test.run to shard i of n
// before testing.M runs. Every top-level test belongs to exactly one shard, so
// the n shards together run exactly the unsharded test set; each package pins
// that with a partition test built on Assert. Shards are balanced with the
// measured durations in the package's duration table (longest first, each test
// onto the least-loaded shard); tests missing from the table weigh the table's
// median, so new tests spread without a table refresh.
//
// Refresh a table from one go test -json run of the package:
//
//	go test -count=1 -json ./internal/vm | jq -c 'select(.Action=="pass" or .Action=="fail") | select(.Test!=null and (.Test|test("/")|not)) | {name:.Test,durationMillis:((.Elapsed*1000)|floor)}' | jq -s '{version:1,package:"github.com/glade-sh/glade/internal/vm",source:"<host, commit, flags>",tests:.}'
//
// The selector env is read only by the package that names it, so a lane that
// runs several packages in one go test invocation shards just that package.
package testshard

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

// TestNamePattern matches the top-level test names a shard may select.
var TestNamePattern = regexp.MustCompile(`^Test[A-Za-z0-9_]*$`)

type durationEntry struct {
	Name           string `json:"name"`
	DurationMillis int64  `json:"durationMillis"`
}

type durationTable struct {
	Version int             `json:"version"`
	Package string          `json:"package"`
	Source  string          `json:"source"`
	Tests   []durationEntry `json:"tests"`
}

// Apply narrows -test.run to the shard selected by the env variable. It does
// nothing unless env is set. Malformed input fails closed so a shard never
// silently runs the wrong test set.
func Apply(env, tablePath, pkg string) error {
	spec := os.Getenv(env)
	if spec == "" {
		return nil
	}
	index, count, err := ParseSpec(env, spec)
	if err != nil {
		return err
	}
	names, err := DiscoverTestNames(env)
	if err != nil {
		return err
	}
	durations, err := LoadDurationTable(tablePath, pkg)
	if err != nil {
		return err
	}
	selected := Assignment(names, durations, count)[index]
	if !flag.Parsed() {
		flag.Parse()
	}
	runFlag := flag.Lookup("test.run")
	if runFlag == nil {
		return fmt.Errorf("%s: test.run flag is not registered", env)
	}
	pattern, err := RunPattern(selected, runFlag.Value.String())
	if err != nil {
		return fmt.Errorf("%s: %w", env, err)
	}
	if err := runFlag.Value.Set(pattern); err != nil {
		return err
	}
	// The shard is now baked into -test.run. Drop the selector so tests that
	// re-exec the test binary (often with another cwd) do not re-apply it and
	// fail to find the relative duration table.
	return os.Unsetenv(env)
}

// ParseSpec parses i/n with 0 <= i < n <= 4096.
func ParseSpec(env, spec string) (index, count int, err error) {
	indexText, countText, ok := strings.Cut(spec, "/")
	if ok {
		index, err = strconv.Atoi(indexText)
		if err == nil {
			count, err = strconv.Atoi(countText)
		}
	}
	if !ok || err != nil || index < 0 || count < 1 || index >= count || count > 4096 {
		return 0, 0, fmt.Errorf("%s=%q: want i/n with 0 <= i < n <= 4096", env, spec)
	}
	return index, count, nil
}

// DiscoverTestNames lists the running test binary's own top-level tests, so
// the assignment input is the testing framework's registry rather than a
// hand-maintained list. The child runs with the selector env removed and
// -test.list only lists: it runs no tests and touches no shared state.
func DiscoverTestNames(env string) ([]string, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("locate test binary: %w", err)
	}
	command := exec.Command(executable, "-test.list", "^Test")
	environment := make([]string, 0, len(os.Environ()))
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, env+"=") {
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
		if !TestNamePattern.MatchString(name) {
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

// LoadDurationTable reads a version-1 table for pkg into a name -> millisecond map.
func LoadDurationTable(path, pkg string) (map[string]int64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("shard duration table: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var table durationTable
	if err := decoder.Decode(&table); err != nil {
		return nil, fmt.Errorf("shard duration table: %w", err)
	}
	if table.Version != 1 || table.Package != pkg {
		return nil, fmt.Errorf("shard duration table: version %d package %q, want version 1 package %q", table.Version, table.Package, pkg)
	}
	durations := make(map[string]int64, len(table.Tests))
	for _, entry := range table.Tests {
		if !TestNamePattern.MatchString(entry.Name) || entry.DurationMillis < 0 {
			return nil, fmt.Errorf("shard duration table: invalid entry %q (%d ms)", entry.Name, entry.DurationMillis)
		}
		if _, exists := durations[entry.Name]; exists {
			return nil, fmt.Errorf("shard duration table: duplicate entry %q", entry.Name)
		}
		durations[entry.Name] = entry.DurationMillis
	}
	return durations, nil
}

// Assignment deterministically partitions names into shardCount bins: longest
// measured duration first, each test onto the least-loaded shard. Names
// without a measured duration weigh the table's median.
func Assignment(names []string, durations map[string]int64, shardCount int) [][]string {
	fallback := MedianDuration(durations)
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

// MedianDuration is the scheduling weight of an unmeasured test: the table's
// median, and at least one millisecond so a new test still spreads across bins.
func MedianDuration(durations map[string]int64) int64 {
	if len(durations) == 0 {
		return 1
	}
	values := make([]int64, 0, len(durations))
	for _, value := range durations {
		values = append(values, value)
	}
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	return max(int64(1), values[(len(values)-1)/2])
}

// RunPattern anchors the selection as an exact alternation. An existing
// -test.run narrows further: each alternative's top-level segment filters the
// shard's names, preserving its subtest segments and Go's regexp boundaries.
func RunPattern(selected []string, existing string) (string, error) {
	var patterns []string
	for _, branch := range regexpParts(existing, '|') {
		parts := regexpParts(branch, '/')
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

// regexpParts splits like testing's -run matcher: only outside bracket
// expressions and parentheses.
func regexpParts(pattern string, separator byte) []string {
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

// Assert proves, from inside the package's own test binary, that every shard
// split of the live registry runs exactly the unsharded test set: the same
// discovery TestMain uses, partitioned by the same assignment, with no test
// dropped, duplicated or invented, and that the checked-in duration table
// names only current tests. minTests guards against broken discovery.
func Assert(t *testing.T, env, tablePath, pkg string, minTests int) {
	t.Helper()
	names, err := DiscoverTestNames(env)
	if err != nil {
		t.Fatal(err)
	}
	all := make(map[string]bool, len(names))
	for _, name := range names {
		all[name] = true
	}
	if !all[t.Name()] {
		t.Fatalf("discovery does not see the running test %s", t.Name())
	}
	if len(names) < minTests {
		t.Fatalf("discovered only %d top-level tests; want at least %d, discovery is broken", len(names), minTests)
	}
	durations, err := LoadDurationTable(tablePath, pkg)
	if err != nil {
		t.Fatal(err)
	}
	for name := range durations {
		if !all[name] {
			t.Errorf("duration table entry %q is not a current top-level test", name)
		}
	}
	for _, shardCount := range []int{1, 2, 3, 5, 16} {
		assigned := make(map[string]int, len(names))
		bins := Assignment(names, durations, shardCount)
		if len(bins) != shardCount {
			t.Fatalf("Assignment returned %d bins, want %d", len(bins), shardCount)
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
