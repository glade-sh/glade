// Command cishard creates deterministic, fail-closed Apex test shard plans.
package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"sort"
	"strings"
)

const apexTestPackage = "github.com/glade-sh/glade/internal/apextest"

var testNamePattern = regexp.MustCompile(`^Test[A-Za-z0-9_]*$`)

type historyTest struct {
	Name           string `json:"name"`
	DurationMillis *int64 `json:"durationMillis"`
}

type historyFile struct {
	Version  int           `json:"version"`
	Package  string        `json:"package"`
	Complete bool          `json:"complete"`
	Tests    []historyTest `json:"tests"`
}

type shardPlan struct {
	Index                   int      `json:"index"`
	Tests                   []string `json:"tests"`
	EstimatedDurationMillis int64    `json:"estimatedDurationMillis"`
	Regex                   string   `json:"regex"`
}

type plan struct {
	Version     int         `json:"version"`
	Package     string      `json:"package"`
	HistoryUsed bool        `json:"historyUsed"`
	Shards      []shardPlan `json:"shards"`
}

type weightedTest struct {
	name     string
	duration int64
	hash     [sha256.Size]byte
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("cishard", flag.ContinueOnError)
	fs.SetOutput(stderr)
	shards := fs.Int("shards", 2, "number of shards")
	testsPath := fs.String("tests", "", "newline-delimited discovered tests (default: stdin)")
	historyPath := fs.String("history", "", "JSON test duration history")
	index := fs.Int("index", -1, "emit only the zero-based shard index")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "cishard: positional arguments are not supported")
		return 2
	}
	indexSet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "index" {
			indexSet = true
		}
	})
	if indexSet && (*index < 0 || *index >= *shards) {
		fmt.Fprintf(stderr, "cishard: --index must be between 0 and %d\n", *shards-1)
		return 2
	}

	testReader := stdin
	var testFile *os.File
	if *testsPath != "" {
		var err error
		testFile, err = os.Open(*testsPath)
		if err != nil {
			fmt.Fprintf(stderr, "cishard: read tests: %v\n", err)
			return 1
		}
		defer testFile.Close()
		testReader = testFile
	}
	names, err := readNames(testReader)
	if err != nil {
		fmt.Fprintf(stderr, "cishard: discovery: %v\n", err)
		return 1
	}

	var history []byte
	if *historyPath != "" {
		history, err = os.ReadFile(*historyPath)
		if err != nil {
			fmt.Fprintf(stderr, "cishard: history rejected: %v; using deterministic fallback\n", err)
		}
	}
	result, diagnostic, err := buildPlan(names, *shards, history)
	if err != nil {
		fmt.Fprintf(stderr, "cishard: %v\n", err)
		return 1
	}
	if diagnostic != "" {
		fmt.Fprintf(stderr, "cishard: %s\n", diagnostic)
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetEscapeHTML(false)
	if indexSet {
		err = encoder.Encode(result.Shards[*index])
	} else {
		err = encoder.Encode(result)
	}
	if err != nil {
		fmt.Fprintf(stderr, "cishard: write plan: %v\n", err)
		return 1
	}
	return 0
}

func readNames(r io.Reader) ([]string, error) {
	scanner := bufio.NewScanner(r)
	var names []string
	for scanner.Scan() {
		name := scanner.Text()
		if strings.TrimSpace(name) == "" {
			return nil, errors.New("empty test name")
		}
		names = append(names, name)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return nil, errors.New("no tests discovered")
	}
	return names, nil
}

func buildPlan(names []string, shardCount int, history []byte) (plan, string, error) {
	canonical, err := validateNames(names)
	if err != nil {
		return plan{}, "", err
	}
	if shardCount < 1 {
		return plan{}, "", errors.New("shard count must be positive")
	}
	if shardCount > len(canonical) {
		return plan{}, "", fmt.Errorf("shard count %d exceeds test count %d", shardCount, len(canonical))
	}

	weights, diagnostic := historyWeights(canonical, history)
	historyUsed := diagnostic == ""
	items := make([]weightedTest, 0, len(canonical))
	for _, name := range canonical {
		items = append(items, weightedTest{name: name, duration: weights[name], hash: sha256.Sum256([]byte(name))})
	}
	if historyUsed {
		sort.Slice(items, func(i, j int) bool {
			if items[i].duration != items[j].duration {
				return items[i].duration > items[j].duration
			}
			return items[i].name < items[j].name
		})
	} else {
		sort.Slice(items, func(i, j int) bool {
			if cmp := bytes.Compare(items[i].hash[:], items[j].hash[:]); cmp != 0 {
				return cmp < 0
			}
			return items[i].name < items[j].name
		})
	}

	result := plan{Version: 1, Package: apexTestPackage, HistoryUsed: historyUsed, Shards: make([]shardPlan, shardCount)}
	counts := make([]int, shardCount)
	for i := range result.Shards {
		result.Shards[i].Index = i
		result.Shards[i].Tests = []string{}
	}
	for _, item := range items {
		target := 0
		for i := 0; i < shardCount; i++ {
			if counts[i] == 0 {
				target = i
				break
			}
		}
		for i := 1; counts[target] != 0 && i < shardCount; i++ {
			if historyUsed {
				if result.Shards[i].EstimatedDurationMillis < result.Shards[target].EstimatedDurationMillis {
					target = i
				}
			} else if counts[i] < counts[target] {
				target = i
			}
		}
		result.Shards[target].Tests = append(result.Shards[target].Tests, item.name)
		result.Shards[target].EstimatedDurationMillis += item.duration
		counts[target]++
	}
	for i := range result.Shards {
		sort.Strings(result.Shards[i].Tests)
		result.Shards[i].Regex = makeRegex(result.Shards[i].Tests)
	}
	if err := validatePlan(result, canonical); err != nil {
		return plan{}, "", fmt.Errorf("internal plan validation: %w", err)
	}
	return result, diagnostic, nil
}

func validateNames(names []string) ([]string, error) {
	if len(names) == 0 {
		return nil, errors.New("no tests discovered")
	}
	canonical := append([]string(nil), names...)
	sort.Strings(canonical)
	for i, name := range canonical {
		if !testNamePattern.MatchString(name) {
			return nil, fmt.Errorf("invalid top-level test name %q", name)
		}
		if i > 0 && canonical[i-1] == name {
			return nil, fmt.Errorf("duplicate top-level test name %q", name)
		}
	}
	return canonical, nil
}

func historyWeights(names []string, data []byte) (map[string]int64, string) {
	fallback := make(map[string]int64, len(names))
	if len(data) == 0 {
		return fallback, "history unavailable; using deterministic fallback"
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var history historyFile
	if err := decoder.Decode(&history); err != nil {
		return fallback, fmt.Sprintf("history rejected: %v; using deterministic fallback", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return fallback, fmt.Sprintf("history rejected: %v; using deterministic fallback", err)
	}
	if history.Version != 1 || history.Package != apexTestPackage || !history.Complete {
		return fallback, "history rejected: wrong schema, package, or completeness; using deterministic fallback"
	}
	want := make(map[string]bool, len(names))
	for _, name := range names {
		want[name] = true
	}
	weights := make(map[string]int64, len(names))
	var totalDuration int64
	for _, item := range history.Tests {
		if !testNamePattern.MatchString(item.Name) || item.DurationMillis == nil || *item.DurationMillis < 0 || !want[item.Name] {
			return fallback, "history rejected: invalid, negative, or stale test entry; using deterministic fallback"
		}
		if _, exists := weights[item.Name]; exists {
			return fallback, "history rejected: duplicate test entry; using deterministic fallback"
		}
		if *item.DurationMillis > math.MaxInt64-totalDuration {
			return fallback, "history rejected: duration total overflows int64; using deterministic fallback"
		}
		totalDuration += *item.DurationMillis
		weights[item.Name] = *item.DurationMillis
	}
	if len(weights) != len(want) {
		return fallback, "history rejected: discovered test set mismatch; using deterministic fallback"
	}
	return weights, ""
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func makeRegex(names []string) string {
	quoted := make([]string, len(names))
	for i, name := range names {
		quoted[i] = regexp.QuoteMeta(name)
	}
	return "^(?:" + strings.Join(quoted, "|") + ")$"
}

func validatePlan(result plan, names []string) error {
	if len(result.Shards) == 0 {
		return errors.New("no shards")
	}
	want := make(map[string]bool, len(names))
	for _, name := range names {
		want[name] = true
	}
	seen := make(map[string]bool, len(names))
	for i, shard := range result.Shards {
		if shard.Index != i || len(shard.Tests) == 0 {
			return fmt.Errorf("invalid or empty shard %d", i)
		}
		if !sort.StringsAreSorted(shard.Tests) {
			return fmt.Errorf("tests in shard %d are not lexical", i)
		}
		for _, name := range shard.Tests {
			if !want[name] || seen[name] {
				return fmt.Errorf("invalid or duplicate test %q", name)
			}
			seen[name] = true
		}
		if shard.Regex != makeRegex(shard.Tests) {
			return fmt.Errorf("invalid regex for shard %d", i)
		}
	}
	if len(seen) != len(want) {
		return fmt.Errorf("union has %d tests, want %d", len(seen), len(want))
	}
	return nil
}
