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

var packageLaneNames = []string{
	"apextest",
	"gladecli",
	"sema",
	"server-and-playground",
	"repoguard",
	"remaining-go",
}

var specializedPackageLanes = map[string][]string{
	"apextest":              {"github.com/glade-sh/glade/internal/apextest"},
	"gladecli":              {"github.com/glade-sh/glade/internal/gladecli"},
	"sema":                  {"github.com/glade-sh/glade/internal/sema"},
	"server-and-playground": {"github.com/glade-sh/glade/internal/playground", "github.com/glade-sh/glade/internal/server"},
	"repoguard":             {"github.com/glade-sh/glade/internal/repoguard"},
}

var testNamePattern = regexp.MustCompile(`^Test[A-Za-z0-9_]*$`)
var importPathPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._~/-]*$`)

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

type packageLanes struct {
	ApexTest            []string `json:"apextest"`
	GladeCLI            []string `json:"gladecli"`
	Sema                []string `json:"sema"`
	ServerAndPlayground []string `json:"server-and-playground"`
	RepoGuard           []string `json:"repoguard"`
	RemainingGo         []string `json:"remaining-go"`
}

type packageManifest struct {
	Version int          `json:"version"`
	Lanes   packageLanes `json:"lanes"`
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("cishard", flag.ContinueOnError)
	fs.SetOutput(stderr)
	shards := fs.Int("shards", 2, "number of shards")
	requestedPackage := fs.String("package", apexTestPackage, "exact Go import path for the test package")
	testsPath := fs.String("tests", "", "newline-delimited discovered tests (default: stdin)")
	historyPath := fs.String("history", "", "JSON test duration history")
	index := fs.Int("index", -1, "emit only the zero-based shard index")
	packageManifestPath := fs.String("package-manifest", "", "checked CI package ownership manifest")
	packagesPath := fs.String("packages", "", "newline-delimited current Go packages")
	lane := fs.String("lane", "", "emit only one package lane")
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
	if *packageManifestPath != "" {
		if *packagesPath == "" {
			fmt.Fprintln(stderr, "cishard: --packages is required with --package-manifest")
			return 2
		}
		forbidden := ""
		fs.Visit(func(f *flag.Flag) {
			switch f.Name {
			case "package-manifest", "packages", "lane":
			default:
				forbidden = f.Name
			}
		})
		if forbidden != "" {
			fmt.Fprintf(stderr, "cishard: --%s is not supported in package manifest mode\n", forbidden)
			return 2
		}
		if err := emitPackageManifest(*packageManifestPath, *packagesPath, *lane, stdout); err != nil {
			fmt.Fprintf(stderr, "cishard: package manifest rejected: %v\n", err)
			return 1
		}
		return 0
	}
	if *packagesPath != "" || *lane != "" {
		fmt.Fprintln(stderr, "cishard: --packages and --lane require --package-manifest")
		return 2
	}
	if err := validateImportPath(*requestedPackage); err != nil {
		fmt.Fprintf(stderr, "cishard: --package: %v\n", err)
		return 2
	}
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
	result, diagnostic, err := buildPlanForPackage(*requestedPackage, names, *shards, history)
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

func validateImportPath(packageName string) error {
	if packageName == "" || !importPathPattern.MatchString(packageName) ||
		strings.Contains(packageName, "//") || strings.Contains(packageName, "...") ||
		strings.HasSuffix(packageName, "/") {
		return fmt.Errorf("invalid exact import path %q", packageName)
	}
	for _, segment := range strings.Split(packageName, "/") {
		if segment == "." || segment == ".." || strings.HasSuffix(segment, ".") {
			return fmt.Errorf("invalid exact import path %q", packageName)
		}
	}
	return nil
}

func emitPackageManifest(manifestPath, packagesPath, selectedLane string, stdout io.Writer) error {
	manifestData, err := os.ReadFile(manifestPath) // #nosec G304 -- --package-manifest is an intentional trusted CLI input.
	if err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}
	currentFile, err := os.Open(packagesPath) // #nosec G304 -- --packages is an intentional trusted CLI input.
	if err != nil {
		return fmt.Errorf("read current packages: %w", err)
	}
	defer currentFile.Close()
	current, err := readPackageNames(currentFile)
	if err != nil {
		return fmt.Errorf("current packages: %w", err)
	}
	lanes, err := validatePackageManifest(manifestData, current)
	if err != nil {
		return err
	}
	if selectedLane != "" {
		if _, ok := lanes[selectedLane]; !ok {
			return fmt.Errorf("unknown lane %q", selectedLane)
		}
	}
	for _, lane := range packageLaneNames {
		if selectedLane != "" && lane != selectedLane {
			continue
		}
		for _, pkg := range lanes[lane] {
			argument := strings.TrimPrefix(pkg, "github.com/glade-sh/glade/")
			if argument != pkg {
				argument = "./" + argument
			}
			if _, err := fmt.Fprintf(stdout, "%s\t%s\n", lane, argument); err != nil {
				return fmt.Errorf("write package ownership: %w", err)
			}
		}
	}
	return nil
}

func readPackageNames(r io.Reader) ([]string, error) {
	scanner := bufio.NewScanner(r)
	var packages []string
	seen := make(map[string]bool)
	for scanner.Scan() {
		pkg := scanner.Text()
		if pkg == "" || strings.TrimSpace(pkg) != pkg || strings.ContainsAny(pkg, " \t") {
			return nil, fmt.Errorf("invalid package path %q", pkg)
		}
		if seen[pkg] {
			return nil, fmt.Errorf("duplicate package path %q", pkg)
		}
		seen[pkg] = true
		packages = append(packages, pkg)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(packages) == 0 {
		return nil, errors.New("no current packages")
	}
	return packages, nil
}

func validatePackageManifest(data []byte, current []string) (map[string][]string, error) {
	if err := rejectDuplicateJSONKeys(data); err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var manifest packageManifest
	if err := decoder.Decode(&manifest); err != nil {
		return nil, err
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return nil, err
	}
	if manifest.Version != 1 {
		return nil, fmt.Errorf("version = %d, want 1", manifest.Version)
	}
	lanes := map[string][]string{
		"apextest":              manifest.Lanes.ApexTest,
		"gladecli":              manifest.Lanes.GladeCLI,
		"sema":                  manifest.Lanes.Sema,
		"server-and-playground": manifest.Lanes.ServerAndPlayground,
		"repoguard":             manifest.Lanes.RepoGuard,
		"remaining-go":          manifest.Lanes.RemainingGo,
	}
	owned := make(map[string]string, len(current))
	for _, lane := range packageLaneNames {
		packages := lanes[lane]
		if len(packages) == 0 {
			return nil, fmt.Errorf("lane %q has empty ownership", lane)
		}
		if !sort.StringsAreSorted(packages) {
			return nil, fmt.Errorf("lane %q packages are not lexical", lane)
		}
		for _, pkg := range packages {
			if pkg == "" || strings.TrimSpace(pkg) != pkg || strings.ContainsAny(pkg, " \t*?") || strings.Contains(pkg, "...") {
				return nil, fmt.Errorf("lane %q has invalid explicit package %q", lane, pkg)
			}
			if previous, exists := owned[pkg]; exists {
				return nil, fmt.Errorf("package %q is owned by both %q and %q", pkg, previous, lane)
			}
			owned[pkg] = lane
		}
	}
	for lane, want := range specializedPackageLanes {
		if !equalStrings(lanes[lane], want) {
			return nil, fmt.Errorf("lane %q ownership does not match specialized CI intent", lane)
		}
	}
	want := make(map[string]bool, len(current))
	for _, pkg := range current {
		want[pkg] = true
		if _, exists := owned[pkg]; !exists {
			return nil, fmt.Errorf("current package %q is unowned", pkg)
		}
	}
	for pkg := range owned {
		if !want[pkg] {
			return nil, fmt.Errorf("owned package %q is not in the current package set", pkg)
		}
	}
	if len(owned) != len(want) {
		return nil, fmt.Errorf("owned package count %d does not match current package count %d", len(owned), len(want))
	}
	return lanes, nil
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func rejectDuplicateJSONKeys(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := consumeJSONValue(decoder); err != nil {
		return err
	}
	return ensureJSONEOF(decoder)
}

func consumeJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := make(map[string]bool)
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return errors.New("object key is not a string")
			}
			if seen[key] {
				return fmt.Errorf("duplicate JSON key %q", key)
			}
			seen[key] = true
			if err := consumeJSONValue(decoder); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
		return err
	case '[':
		for decoder.More() {
			if err := consumeJSONValue(decoder); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
		return err
	default:
		return fmt.Errorf("unexpected JSON delimiter %q", delim)
	}
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
	return buildPlanForPackage(apexTestPackage, names, shardCount, history)
}

func buildPlanForPackage(packageName string, names []string, shardCount int, history []byte) (plan, string, error) {
	if err := validateImportPath(packageName); err != nil {
		return plan{}, "", err
	}
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

	weights, diagnostic := historyWeightsForPackage(packageName, canonical, history)
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

	result := plan{Version: 1, Package: packageName, HistoryUsed: historyUsed, Shards: make([]shardPlan, shardCount)}
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
	return historyWeightsForPackage(apexTestPackage, names, data)
}

func historyWeightsForPackage(packageName string, names []string, data []byte) (map[string]int64, string) {
	fallback := make(map[string]int64, len(names))
	if len(data) == 0 {
		return fallback, "history unavailable; using deterministic fallback"
	}
	if err := rejectDuplicateJSONKeys(data); err != nil {
		return fallback, fmt.Sprintf("history rejected: %v; using deterministic fallback", err)
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
	if history.Version != 1 || history.Package != packageName || !history.Complete {
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
	for _, name := range names {
		if _, ok := weights[name]; !ok {
			// A newly discovered test has no measured duration yet. Keeping the
			// known history preserves the existing shard assignment and lets the
			// new test join the currently lighter shard.
			weights[name] = 0
		}
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
