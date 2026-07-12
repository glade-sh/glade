package scripts

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

const (
	playgroundGroupOne   = "TestExampleProjectsRunAnonymousGroupOne"
	playgroundGroupTwo   = "TestExampleProjectsRunAnonymousGroupTwo"
	playgroundGroupThree = "TestExampleProjectsRunAnonymousGroupThree"
	playgroundGroupFour  = "TestExampleProjectsRunAnonymousGroupFour"
)

func playgroundRaceDiscovery(extra ...string) string {
	names := []string{playgroundGroupOne, playgroundGroupTwo, playgroundGroupThree, playgroundGroupFour, "TestAlpha", "TestBeta", "TestDelta", "TestEpsilon", "TestGamma"}
	names = append(names, extra...)
	return strings.Join(names, "\n") + "\nok   github.com/glade-sh/glade/internal/playground 0.001s\n"
}

func racePlaygroundPlannerFixture() string {
	return `#!/usr/bin/env bash
set -euo pipefail
while [[ "$#" -gt 0 ]]; do case "$1" in --package) package="$2"; shift 2;; --shards) [[ "$2" == 5 ]]; shift 2;; --tests) tests="$2"; shift 2;; *) exit 91;; esac; done
names=(); while IFS= read -r name; do names+=("$name"); done <"$tests"
printf '{"version":1,"package":"%s","historyUsed":false,"shards":[' "$package"
for index in 0 1 2 3 4; do [[ "$index" == 0 ]] || printf ','; printf '{"index":%s,"tests":["%s"],"estimatedDurationMillis":0,"regex":"^(?:%s)$"}' "$index" "${names[$index]}" "${names[$index]}"; done
printf ']}\n'
`
}

func runRaceClassifier(t *testing.T, env []string, args ...string) ([]string, string, error) {
	t.Helper()
	cmd := exec.Command("bash", append([]string{"ci-race-packages.sh"}, args...)...)
	cmd.Env = append(os.Environ(), env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return nil, stderr.String(), err
	}
	var packages []string
	if err := json.Unmarshal(stdout.Bytes(), &packages); err != nil {
		t.Fatalf("classifier returned invalid JSON: %v\nstdout=%s\nstderr=%s", err, stdout.Bytes(), stderr.Bytes())
	}
	return packages, stderr.String(), nil
}

func TestCIRaceClassifierHighRiskSet(t *testing.T) {
	got, out, err := runRaceClassifier(t, nil, "high-risk")
	if err != nil {
		t.Fatalf("high-risk classification failed: %v\n%s", err, out)
	}
	want := []string{
		"./internal/apextest", "./internal/gladecli", "./internal/playground",
		"./internal/sema", "./internal/server", "./internal/startupcache", "./internal/storage",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("high-risk packages = %v, want %v", got, want)
	}
}

func TestCIRaceClassifierFullMatchesManifest(t *testing.T) {
	got, out, err := runRaceClassifier(t, nil, "full")
	if err != nil {
		t.Fatalf("full classification failed: %v\n%s", err, out)
	}
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
	var want []string
	for _, packages := range manifest.Lanes {
		for _, pkg := range packages {
			want = append(want, "."+strings.TrimPrefix(pkg, "github.com/glade-sh/glade"))
		}
	}
	sort.Strings(want)
	if len(got) != 61 || !reflect.DeepEqual(got, want) {
		t.Fatalf("full packages = %d/%v, want 61 exact manifest packages", len(got), got)
	}
}

func TestCIRaceClassifierChangedIncludesDependents(t *testing.T) {
	dir := t.TempDir()
	gitPath := filepath.Join(dir, "git")
	goPath := filepath.Join(dir, "go")
	gitScript := "#!/usr/bin/env bash\nprintf 'M\\t%s\\n' internal/storage/store.go\n"
	goScript := `#!/usr/bin/env bash
case "$*" in
  *"./internal/storage") printf '%s\n' github.com/glade-sh/glade/internal/storage ;;
  *"./...")
    if [[ "$*" == *TestImports* ]]; then
      printf '%s\t%s\t%s\t%s\n' github.com/glade-sh/glade/internal/storage '' '' ''
      printf '%s\t%s\t%s\t%s\n' github.com/glade-sh/glade/internal/vm 'github.com/glade-sh/glade/internal/storage' '' ''
      printf '%s\t%s\t%s\t%s\n' github.com/glade-sh/glade/internal/soql 'github.com/glade-sh/glade/internal/storage github.com/glade-sh/glade/internal/vm' '' ''
      printf '%s\t%s\t%s\t%s\n' github.com/glade-sh/glade/internal/apexlog '' 'github.com/glade-sh/glade/internal/storage' ''
      printf '%s\t%s\t%s\t%s\n' github.com/glade-sh/glade/internal/cliui '' '' ''
    else
      printf '%s\t%s\n' github.com/glade-sh/glade/internal/storage ''
      printf '%s\t%s\n' github.com/glade-sh/glade/internal/vm 'github.com/glade-sh/glade/internal/storage'
      printf '%s\t%s\n' github.com/glade-sh/glade/internal/soql 'github.com/glade-sh/glade/internal/storage github.com/glade-sh/glade/internal/vm'
      printf '%s\t%s\n' github.com/glade-sh/glade/internal/apexlog ''
      printf '%s\t%s\n' github.com/glade-sh/glade/internal/cliui ''
    fi
    ;;
  *) exit 97 ;;
esac
`
	for path, contents := range map[string]string{gitPath: gitScript, goPath: goScript} {
		if err := os.WriteFile(path, []byte(contents), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	got, out, err := runRaceClassifier(t, []string{"CI_RACE_GIT_COMMAND=" + gitPath, "CI_RACE_GO_COMMAND=" + goPath}, "changed", "base", "head")
	if err != nil {
		t.Fatalf("changed classification failed: %v\n%s", err, out)
	}
	want := []string{"./internal/apexlog", "./internal/soql", "./internal/storage", "./internal/vm"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("changed packages = %v, want %v", got, want)
	}
}

func TestCIRaceClassifierNonGoChangeIsEmpty(t *testing.T) {
	dir := t.TempDir()
	gitPath := filepath.Join(dir, "git")
	if err := os.WriteFile(gitPath, []byte("#!/usr/bin/env bash\nprintf 'M\\t%s\\n' docs/README.md\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	got, out, err := runRaceClassifier(t, []string{"CI_RACE_GIT_COMMAND=" + gitPath}, "changed", "base", "head")
	if err != nil {
		t.Fatalf("non-Go classification failed: %v\n%s", err, out)
	}
	if len(got) != 0 {
		t.Fatalf("non-Go change selected %v", got)
	}
}

func TestCIRaceClassifierFallsBackToFullOnDiffFailure(t *testing.T) {
	got, diagnostic, err := runRaceClassifier(t, []string{"CI_RACE_GIT_COMMAND=false"}, "changed", "base", "head")
	if err != nil {
		t.Fatalf("diff failure did not fall back: %v\n%s", err, diagnostic)
	}
	if len(got) != 61 || !strings.Contains(diagnostic, "git diff failed") {
		t.Fatalf("diff failure selected %d packages without diagnostic: %s", len(got), diagnostic)
	}
}

func TestCIRaceClassifierFallsBackToFullOnGoDeletion(t *testing.T) {
	dir := t.TempDir()
	gitPath := filepath.Join(dir, "git")
	if err := os.WriteFile(gitPath, []byte("#!/usr/bin/env bash\nprintf 'D\\t%s\\n' internal/storage/store.go\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	got, diagnostic, err := runRaceClassifier(t, []string{"CI_RACE_GIT_COMMAND=" + gitPath}, "changed", "base", "head")
	if err != nil {
		t.Fatalf("deletion did not fall back: %v\n%s", err, diagnostic)
	}
	if len(got) != 61 || !strings.Contains(diagnostic, "deleted Go file") {
		t.Fatalf("deletion selected %d packages without diagnostic: %s", len(got), diagnostic)
	}
}

func TestCIRaceClassifierFallsBackToFullOnNestedModuleChange(t *testing.T) {
	dir := t.TempDir()
	gitPath := filepath.Join(dir, "git")
	if err := os.WriteFile(gitPath, []byte("#!/usr/bin/env bash\nprintf 'M\\t%s\\n' third_party/glade-apex-parser/go.mod\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	got, diagnostic, err := runRaceClassifier(t, []string{"CI_RACE_GIT_COMMAND=" + gitPath}, "changed", "base", "head")
	if err != nil {
		t.Fatalf("nested module change did not fall back: %v\n%s", err, diagnostic)
	}
	if len(got) != 61 {
		t.Fatalf("nested module change selected %d packages, want full 61", len(got))
	}
}

func TestCIRaceClassifierFallsBackToFullOnDependencyGraphFailure(t *testing.T) {
	dir := t.TempDir()
	gitPath := filepath.Join(dir, "git")
	goPath := filepath.Join(dir, "go")
	if err := os.WriteFile(gitPath, []byte("#!/usr/bin/env bash\nprintf 'M\\t%s\\n' internal/storage/store.go\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	goScript := `#!/usr/bin/env bash
case "$*" in
  *"./internal/storage") printf '%s\n' github.com/glade-sh/glade/internal/storage ;;
  *"./...") exit 41 ;;
  *) exit 97 ;;
esac
`
	if err := os.WriteFile(goPath, []byte(goScript), 0o700); err != nil {
		t.Fatal(err)
	}
	got, diagnostic, err := runRaceClassifier(t, []string{"CI_RACE_GIT_COMMAND=" + gitPath, "CI_RACE_GO_COMMAND=" + goPath}, "changed", "base", "head")
	if err != nil {
		t.Fatalf("graph failure did not fall back: %v\n%s", err, diagnostic)
	}
	if len(got) != 61 || !strings.Contains(diagnostic, "dependency graph failed") {
		t.Fatalf("graph failure selected %d packages without diagnostic: %s", len(got), diagnostic)
	}
}

func TestCIRaceWorkflowContract(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", ".github", "workflows", "race.yml"))
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(data)
	for _, marker := range []string{
		"name: Race", "pull_request:", "branches: [main]", "schedule:", "workflow_dispatch:",
		"fetch-depth: 0", "scripts/ci-race-packages.sh", "fromJSON(needs.plan.outputs.packages)",
		"fail-fast: false", "max-parallel: 4", "GOMAXPROCS: \"2\"", "go-version: \"1.26.5\"",
		`scripts/ci-race-test.sh "$PACKAGE" "$SLUG"`, "if: always()",
		"actions/upload-artifact@v6", "ci-artifacts/race/", "if-no-files-found: error",
		"npm ci --prefix third_party/lwc", "contains(fromJSON('[\"./internal/gladecli\"",
	} {
		if !strings.Contains(workflow, marker) {
			t.Errorf("race workflow missing %q", marker)
		}
	}
	for _, forbidden := range []string{"pull_request_target:", "continue-on-error:", "runs-on: ubuntu-latest-", "go test -race ./..."} {
		if strings.Contains(workflow, forbidden) {
			t.Errorf("race workflow contains forbidden %q", forbidden)
		}
	}
	if strings.Contains(workflow, "Required CI") || strings.Contains(workflow, "required-ci") {
		t.Fatal("race workflow is coupled to normal Required CI")
	}
	if strings.Contains(workflow, "scripts/ci-resource-run.sh") || strings.Contains(workflow, "go test -race") {
		t.Fatal("race workflow bypasses the package race runner")
	}
}

func TestCIRacePackageRunnerPlaygroundGroupDiagnosticContract(t *testing.T) {
	data, err := os.ReadFile("ci-race-test.sh")
	if err != nil {
		t.Fatal(err)
	}
	script := string(data)
	if !strings.Contains(script, "exactly the four example execution groups") {
		t.Fatal("race runner is missing the exact four-group discovery diagnostic")
	}
	if strings.Contains(script, "exactly the two example execution groups") {
		t.Fatal("race runner still describes the obsolete two-group contract")
	}
}

func writeRaceFixture(t *testing.T, name, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(contents), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func runRacePackage(t *testing.T, workdir string, env []string, pkg, slug string) (string, error) {
	t.Helper()
	if pkg == "./internal/apextest" || pkg == "./internal/gladecli" || pkg == "./internal/playground" {
		hasTimeout := false
		for _, value := range env {
			if strings.HasPrefix(value, "CI_RACE_TIMEOUT_COMMAND=") {
				hasTimeout = true
				break
			}
		}
		if !hasTimeout {
			timeout := writeRaceFixture(t, "fake-timeout", `#!/usr/bin/env bash
set -euo pipefail
[[ "$1" == "--signal=TERM" && "$2" == "--kill-after=30s" && "$3" == "60m" ]] || exit 92
if [[ -n "${CI_RACE_CALL_LOG:-}" ]]; then printf 'deadline %s %s %s\n' "$1" "$2" "$3" >>"$CI_RACE_CALL_LOG"; fi
shift 3
exec "$@"
`)
			env = append(env, "CI_RACE_TIMEOUT_COMMAND="+timeout)
		}
	}
	script, err := filepath.Abs("ci-race-test.sh")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", script, pkg, slug)
	cmd.Dir = workdir
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func raceExitCode(err error) int {
	if err == nil {
		return 0
	}
	if exitError, ok := err.(*exec.ExitError); ok {
		return exitError.ExitCode()
	}
	return -1
}

func raceResourceFixture() string {
	return `#!/usr/bin/env bash
set -euo pipefail
output="$1"
lane="$2"
shift 3
printf 'start %s\n' "$lane" >>"$CI_RACE_CALL_LOG"
set +e
"$@"
rc="$?"
set -e
mkdir -p "$(dirname "$output")"
printf '{"schema_version":1,"lane":"%s","elapsed_seconds":0,"user_seconds":0,"system_seconds":0,"max_rss_kb":0,"exit_status":%s}\n' "$lane" "$rc" >"$output"
printf 'end %s %s\n' "$lane" "$rc" >>"$CI_RACE_CALL_LOG"
exit "$rc"
`
}

func racePlannerFixture() string {
	return `#!/usr/bin/env bash
set -euo pipefail
package=""
tests=""
while [[ "$#" -gt 0 ]]; do
  case "$1" in
    --package) package="$2"; shift 2 ;;
    --shards) [[ "$2" == 4 ]]; shift 2 ;;
    --tests) tests="$2"; shift 2 ;;
    *) exit 91 ;;
  esac
done
names=()
while IFS= read -r name; do names+=("$name"); done <"$tests"
printf '{"version":1,"package":"%s","historyUsed":false,"shards":[' "$package"
for index in 0 1 2 3; do
  if [[ "$index" -gt 0 ]]; then printf ','; fi
  printf '{"index":%s,"tests":["%s"],"estimatedDurationMillis":0,"regex":"^(?:%s)$"}' "$index" "${names[$index]}" "${names[$index]}"
done
printf ']}\n'
`
}

func racePlannerOmissionFixture() string {
	return `#!/usr/bin/env bash
printf '%s\n' '{"version":1,"package":"github.com/glade-sh/glade/internal/playground","historyUsed":false,"shards":[{"index":0,"tests":["TestAlpha"],"estimatedDurationMillis":0,"regex":"^(?:TestAlpha)$"},{"index":1,"tests":["TestBeta"],"estimatedDurationMillis":0,"regex":"^(?:TestBeta)$"},{"index":2,"tests":["TestDelta"],"estimatedDurationMillis":0,"regex":"^(?:TestDelta)$"},{"index":3,"tests":["TestEpsilon"],"estimatedDurationMillis":0,"regex":"^(?:TestEpsilon)$"},{"index":4,"tests":["TestGamma"],"estimatedDurationMillis":0,"regex":"^(?:TestGamma)$"}]}'
`
}

func racePlannerDuplicateFixture() string {
	return `#!/usr/bin/env bash
printf '%s\n' '{"version":1,"package":"github.com/glade-sh/glade/internal/playground","historyUsed":false,"shards":[{"index":0,"tests":["TestAlpha"],"estimatedDurationMillis":0,"regex":"^(?:TestAlpha)$"},{"index":1,"tests":["TestAlpha"],"estimatedDurationMillis":0,"regex":"^(?:TestAlpha)$"},{"index":2,"tests":["TestBeta"],"estimatedDurationMillis":0,"regex":"^(?:TestBeta)$"},{"index":3,"tests":["TestDelta"],"estimatedDurationMillis":0,"regex":"^(?:TestDelta)$"},{"index":4,"tests":["TestGamma"],"estimatedDurationMillis":0,"regex":"^(?:TestGamma)$"}]}'
`
}

func racePlannerEmptyShardFixture() string {
	return `#!/usr/bin/env bash
printf '%s\n' '{"version":1,"package":"github.com/glade-sh/glade/internal/playground","historyUsed":false,"shards":[{"index":0,"tests":[],"estimatedDurationMillis":0,"regex":"^(?:)$"},{"index":1,"tests":["TestAlpha","TestBeta"],"estimatedDurationMillis":0,"regex":"^(?:TestAlpha|TestBeta)$"},{"index":2,"tests":["TestDelta"],"estimatedDurationMillis":0,"regex":"^(?:TestDelta)$"},{"index":3,"tests":["TestEpsilon"],"estimatedDurationMillis":0,"regex":"^(?:TestEpsilon)$"},{"index":4,"tests":["TestGamma"],"estimatedDurationMillis":0,"regex":"^(?:TestGamma)$"}]}'
`
}

func TestCIRacePackageRunnerDirectPackageUsesOneNativeProcess(t *testing.T) {
	root := t.TempDir()
	logPath := filepath.Join(root, "calls.log")
	goCommand := writeRaceFixture(t, "fake-go", `#!/usr/bin/env bash
printf '%q ' "$@" >>"$CI_RACE_GO_LOG"
printf '\n' >>"$CI_RACE_GO_LOG"
exit "${CI_RACE_GO_STATUS:-0}"
`)
	resource := writeRaceFixture(t, "fake-resource", raceResourceFixture())
	env := []string{"CI_RACE_GO_COMMAND=" + goCommand, "CI_RACE_RESOURCE_RUNNER=" + resource, "CI_RACE_CALL_LOG=" + logPath, "CI_RACE_GO_LOG=" + logPath}
	out, err := runRacePackage(t, root, env, "./internal/storage", "internal-storage")
	if err != nil {
		t.Fatalf("direct package failed: %v\n%s", err, out)
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	log := string(data)
	if strings.Count(log, "test -race -count=1 -timeout=60m ./internal/storage") != 1 {
		t.Fatalf("direct native calls:\n%s", log)
	}
	if strings.Contains(log, "-list") || strings.Contains(log, "shard-") {
		t.Fatalf("direct package entered heavy path:\n%s", log)
	}
	if _, err := os.Stat(filepath.Join(root, "ci-artifacts/race/internal-storage/resource.json")); err != nil {
		t.Fatalf("direct resource evidence: %v", err)
	}
}

func TestCIRacePackageRunnerRejectsUnsafePackageSegmentsAndSlugs(t *testing.T) {
	tests := []struct {
		name string
		pkg  string
		slug string
	}{
		{name: "terminal dot", pkg: "./.", slug: "."},
		{name: "terminal dotdot", pkg: "./..", slug: ".."},
		{name: "nested terminal dot", pkg: "./internal/.", slug: "internal-."},
		{name: "nested terminal dotdot", pkg: "./internal/..", slug: "internal-.."},
		{name: "middle dot", pkg: "./internal/./storage", slug: "internal-.-storage"},
		{name: "middle dotdot", pkg: "./internal/../storage", slug: "internal-..-storage"},
		{name: "recursive root", pkg: "./...", slug: "..."},
		{name: "recursive segment", pkg: "./internal/...", slug: "internal-..."},
		{name: "embedded recursive pattern", pkg: "./internal/foo...bar", slug: "internal-foo...bar"},
		{name: "slug traversal", pkg: "./internal/storage", slug: "../storage"},
		{name: "slug mismatch", pkg: "./internal/storage", slug: "internal-other"},
	}
	for _, fixture := range tests {
		t.Run(fixture.name, func(t *testing.T) {
			root := t.TempDir()
			logPath := filepath.Join(root, "calls.log")
			goCommand := writeRaceFixture(t, "fake-go", "#!/usr/bin/env bash\nexit 0\n")
			resource := writeRaceFixture(t, "fake-resource", raceResourceFixture())
			out, err := runRacePackage(t, root, []string{
				"CI_RACE_GO_COMMAND=" + goCommand, "CI_RACE_RESOURCE_RUNNER=" + resource,
				"CI_RACE_CALL_LOG=" + logPath,
			}, fixture.pkg, fixture.slug)
			if got := raceExitCode(err); got != 2 {
				t.Fatalf("unsafe package/slug exit = %d, want 2:\n%s", got, out)
			}
			if data, readErr := os.ReadFile(logPath); readErr == nil && len(data) != 0 {
				t.Fatalf("unsafe package reached resource runner:\n%s", data)
			}
		})
	}
}

func TestCIRacePackageRunnerPreservesOuterDeadlineStatus(t *testing.T) {
	root := t.TempDir()
	goCommand := writeRaceFixture(t, "fake-go", "#!/usr/bin/env bash\nexit 97\n")
	resource := writeRaceFixture(t, "fake-resource", "#!/usr/bin/env bash\nexit 98\n")
	timeout := writeRaceFixture(t, "fake-timeout", `#!/usr/bin/env bash
[[ "$1" == "--signal=TERM" && "$2" == "--kill-after=30s" && "$3" == "60m" ]] || exit 92
exit 124
`)
	out, err := runRacePackage(t, root, []string{
		"CI_RACE_GO_COMMAND=" + goCommand, "CI_RACE_RESOURCE_RUNNER=" + resource,
		"CI_RACE_TIMEOUT_COMMAND=" + timeout,
	}, "./internal/playground", "internal-playground")
	if got := raceExitCode(err); got != 124 {
		t.Fatalf("outer deadline exit = %d, want 124\n%s", got, out)
	}
}

func TestCIRacePlaygroundRealPlanUsesFiveOrdinaryShards(t *testing.T) {
	discoveryCommand := exec.Command("go", "test", "-race", "-list", ".", "../internal/playground")
	discoveryOutput, err := discoveryCommand.Output()
	if err != nil {
		t.Fatalf("playground race discovery: %v", err)
	}
	testPattern := regexp.MustCompile(`^Test[A-Za-z0-9_]*$`)
	benchmarkPattern := regexp.MustCompile(`^Benchmark[A-Za-z0-9_]*$`)
	trailerPattern := regexp.MustCompile(`^ok\s+github\.com/glade-sh/glade/internal/playground(?:\s+\S+)?$`)
	groups := map[string]bool{playgroundGroupOne: true, playgroundGroupTwo: true, playgroundGroupThree: true, playgroundGroupFour: true}
	foundGroups := map[string]bool{}
	seenEntries := map[string]bool{}
	var ordinary []string
	trailers := 0
	for lineNumber, line := range strings.Split(strings.TrimSuffix(string(discoveryOutput), "\n"), "\n") {
		switch {
		case testPattern.MatchString(line):
			if seenEntries[line] {
				t.Fatalf("duplicate live playground discovery entry %q", line)
			}
			seenEntries[line] = true
			if groups[line] {
				foundGroups[line] = true
			} else {
				ordinary = append(ordinary, line)
			}
		case benchmarkPattern.MatchString(line):
			if seenEntries[line] {
				t.Fatalf("duplicate live playground discovery entry %q", line)
			}
			seenEntries[line] = true
		case trailerPattern.MatchString(line):
			trailers++
		default:
			t.Fatalf("invalid live playground discovery line %d: %q", lineNumber+1, line)
		}
	}
	if !reflect.DeepEqual(foundGroups, groups) {
		t.Fatalf("live playground groups = %#v, want %#v", foundGroups, groups)
	}
	if len(ordinary) == 0 || trailers != 1 {
		t.Fatalf("live ordinary tests/trailers = %d/%d, want nonempty/1", len(ordinary), trailers)
	}
	sort.Strings(ordinary)
	discovery := filepath.Join(t.TempDir(), "discovery.txt")
	if err := os.WriteFile(discovery, []byte(strings.Join(ordinary, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	planner := exec.Command("go", "run", "./internal/cishard", "--package", "github.com/glade-sh/glade/internal/playground", "--shards", "5", "--tests", discovery)
	var stdout, stderr bytes.Buffer
	planner.Stdout = &stdout
	planner.Stderr = &stderr
	if err := planner.Run(); err != nil {
		t.Fatalf("real cishard failed: %v\n%s", err, stderr.String())
	}
	var plan struct {
		Version     int               `json:"version"`
		Package     string            `json:"package"`
		HistoryUsed bool              `json:"historyUsed"`
		Shards      []json.RawMessage `json:"shards"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &plan); err != nil {
		t.Fatalf("real cishard plan JSON: %v\n%s", err, stdout.String())
	}
	var planObject map[string]json.RawMessage
	if err := json.Unmarshal(stdout.Bytes(), &planObject); err != nil || len(planObject) != 4 || planObject["version"] == nil || planObject["package"] == nil || planObject["historyUsed"] == nil || planObject["shards"] == nil {
		t.Fatalf("real cishard plan does not have the exact schema: %v", err)
	}
	if plan.Version != 1 || plan.Package != "github.com/glade-sh/glade/internal/playground" || len(plan.Shards) != 5 {
		t.Fatalf("real cishard plan header = version %d package %q shards %d", plan.Version, plan.Package, len(plan.Shards))
	}
	var union []string
	for expectedIndex, rawShard := range plan.Shards {
		var shard struct {
			Index                   int      `json:"index"`
			Tests                   []string `json:"tests"`
			EstimatedDurationMillis int64    `json:"estimatedDurationMillis"`
			Regex                   string   `json:"regex"`
		}
		var shardObject map[string]json.RawMessage
		if err := json.Unmarshal(rawShard, &shard); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(rawShard, &shardObject); err != nil || len(shardObject) != 4 || shardObject["index"] == nil || shardObject["tests"] == nil || shardObject["estimatedDurationMillis"] == nil || shardObject["regex"] == nil {
			t.Fatalf("real cishard shard %d does not have the exact schema", expectedIndex)
		}
		canonical := append([]string(nil), shard.Tests...)
		sort.Strings(canonical)
		if shard.Index != expectedIndex || len(shard.Tests) == 0 || !reflect.DeepEqual(shard.Tests, canonical) || shard.EstimatedDurationMillis < 0 {
			t.Fatalf("real cishard shard %d is invalid: %#v", expectedIndex, shard)
		}
		wantRegex := "^(?:" + strings.Join(func() []string {
			quoted := make([]string, len(shard.Tests))
			for i, name := range shard.Tests {
				quoted[i] = regexp.QuoteMeta(name)
			}
			return quoted
		}(), "|") + ")$"
		if shard.Regex != wantRegex {
			t.Fatalf("real cishard shard %d regex = %q, want %q", expectedIndex, shard.Regex, wantRegex)
		}
		for _, name := range shard.Tests {
			if groups[name] {
				t.Fatalf("example group %q leaked into ordinary plan", name)
			}
		}
		union = append(union, shard.Tests...)
	}
	sort.Strings(union)
	if len(union) != len(ordinary) || !reflect.DeepEqual(union, ordinary) {
		t.Fatalf("real cishard ordinary union differs: got %d tests, want %d", len(union), len(ordinary))
	}
}

func TestCIRacePackageRunnerGenericHeavyPackagesUseFourSequentialExactShards(t *testing.T) {
	for _, pkg := range []string{"./internal/apextest", "./internal/gladecli"} {
		t.Run(filepath.Base(pkg), func(t *testing.T) {
			root := t.TempDir()
			logPath := filepath.Join(root, "calls.log")
			testNames := []string{"TestAlpha", "TestBeta", "TestDelta", "TestGamma"}
			goCommand := writeRaceFixture(t, "fake-go", `#!/usr/bin/env bash
if [[ "$*" == *"-list ."* ]]; then
	printf 'discovery %s\n' "$*" >>"$CI_RACE_CALL_LOG"
	  printf '%b' "$CI_RACE_TEST_NAMES"
	  printf 'BenchmarkFixture\nok   github.com/glade-sh/glade/%s 0.001s\n' "${CI_RACE_PACKAGE#./}"
  exit 0
fi
printf 'native %s\n' "$*" >>"$CI_RACE_CALL_LOG"
while IFS= read -r candidate; do
  if [[ "$*" == *"$candidate"* ]]; then test="$candidate"; break; fi
done <<<"$CI_RACE_TEST_NAMES"
printf '{"Action":"run","Package":"github.com/glade-sh/glade/%s","Test":"%s"}\n' "${CI_RACE_PACKAGE#./}" "$test"
printf '{"Action":"pass","Package":"github.com/glade-sh/glade/%s","Test":"%s","Elapsed":0.01}\n' "${CI_RACE_PACKAGE#./}" "$test"
`)
			planner := writeRaceFixture(t, "fake-planner", racePlannerFixture())
			resource := writeRaceFixture(t, "fake-resource", raceResourceFixture())
			env := []string{
				"CI_RACE_GO_COMMAND=" + goCommand, "CI_RACE_SHARD_PLANNER=" + planner,
				"CI_RACE_RESOURCE_RUNNER=" + resource, "CI_RACE_CALL_LOG=" + logPath,
				"CI_RACE_PACKAGE=" + pkg,
				"CI_RACE_TEST_NAMES=" + strings.Join(testNames, "\n") + "\n",
			}
			slug := strings.ReplaceAll(strings.TrimPrefix(pkg, "./"), "/", "-")
			out, err := runRacePackage(t, root, env, pkg, slug)
			if err != nil {
				t.Fatalf("heavy package failed: %v\n%s", err, out)
			}
			data, err := os.ReadFile(logPath)
			if err != nil {
				t.Fatal(err)
			}
			log := string(data)
			var wantOrder []string
			for shard := 0; shard < 4; shard++ {
				wantOrder = append(wantOrder, "start race-"+slug+"-shard-"+fmt.Sprint(shard), "end race-"+slug+"-shard-"+fmt.Sprint(shard)+" 0")
			}
			position := 0
			for _, marker := range wantOrder {
				next := strings.Index(log[position:], marker)
				if next < 0 {
					t.Fatalf("missing sequential marker %q:\n%s", marker, log)
				}
				position += next + len(marker)
			}
			if strings.Count(log, "native test -json -race -count=1 -timeout=60m -run") != 4 {
				t.Fatalf("native heavy calls:\n%s", log)
			}
			if strings.Count(log, "discovery test -race -list . "+pkg) != 1 {
				t.Fatalf("heavy discovery did not use the exact race-tagged test set:\n%s", log)
			}
			if strings.Count(log, "deadline --signal=TERM --kill-after=30s 60m") != 1 {
				t.Fatalf("heavy package did not use exactly one aggregate deadline:\n%s", log)
			}
			benchmarks, err := os.ReadFile(filepath.Join(root, "ci-artifacts/race", slug, "discovery-benchmarks.txt"))
			if err != nil || string(benchmarks) != "BenchmarkFixture\n" {
				t.Fatalf("recorded benchmarks = %q, err=%v", benchmarks, err)
			}
			for shard := 0; shard < 4; shard++ {
				base := filepath.Join(root, "ci-artifacts/race", slug, "shard-"+string(rune('0'+shard)))
				for _, name := range []string{"events.json", "resource.json"} {
					if _, err := os.Stat(filepath.Join(base, name)); err != nil {
						t.Fatalf("shard %d %s: %v", shard, name, err)
					}
				}
			}
			unionData, err := os.ReadFile(filepath.Join(root, "ci-artifacts/race", slug, "union-validation.json"))
			if err != nil {
				t.Fatalf("union evidence: %v", err)
			}
			var union struct {
				SchemaVersion   int    `json:"schema_version"`
				Package         string `json:"package"`
				DiscoveredCount int    `json:"discovered_count"`
				ShardCounts     []int  `json:"shard_counts"`
				NamesSHA256     string `json:"names_sha256"`
				Valid           bool   `json:"valid"`
			}
			if err := json.Unmarshal(unionData, &union); err != nil {
				t.Fatalf("union evidence JSON: %v", err)
			}
			canonicalNames := append([]string(nil), testNames...)
			sort.Strings(canonicalNames)
			namesHash := fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(canonicalNames, "\n")+"\n")))
			if union.SchemaVersion != 1 || union.Package != "github.com/glade-sh/glade/"+strings.TrimPrefix(pkg, "./") ||
				union.DiscoveredCount != 4 || !reflect.DeepEqual(union.ShardCounts, []int{1, 1, 1, 1}) ||
				union.NamesSHA256 != namesHash || !union.Valid {
				t.Fatalf("union evidence = %#v", union)
			}
		})
	}
}

func TestCIRacePackageRunnerPlaygroundUsesNineSequentialExactLanes(t *testing.T) {
	root := t.TempDir()
	logPath := filepath.Join(root, "calls.log")
	ordinary := []string{"TestAlpha", "TestBeta", "TestDelta", "TestEpsilon", "TestGamma"}
	all := append([]string{playgroundGroupOne, playgroundGroupTwo, playgroundGroupThree, playgroundGroupFour}, ordinary...)
	goCommand := writeRaceFixture(t, "fake-go", `#!/usr/bin/env bash
if [[ "$*" == *"-list ."* ]]; then printf '%b' "$CI_RACE_TEST_NAMES"; printf 'ok   github.com/glade-sh/glade/internal/playground 0.001s\n'; exit 0; fi
printf 'native %s\n' "$*" >>"$CI_RACE_CALL_LOG"
while IFS= read -r candidate; do if [[ "$*" == *"$candidate"* ]]; then test="$candidate"; break; fi; done <<<"$CI_RACE_TEST_NAMES"
printf '{"Action":"pass","Package":"github.com/glade-sh/glade/internal/playground","Test":"%s"}\n' "$test"
`)
	planner := writeRaceFixture(t, "fake-planner", `#!/usr/bin/env bash
set -euo pipefail
while [[ "$#" -gt 0 ]]; do case "$1" in --package) package="$2"; shift 2;; --shards) [[ "$2" == 5 ]]; shift 2;; --tests) tests="$2"; shift 2;; *) exit 91;; esac; done
names=(); while IFS= read -r name; do names+=("$name"); done <"$tests"
[[ "${#names[@]}" == 5 ]]
printf '{"version":1,"package":"%s","historyUsed":false,"shards":[' "$package"
for index in 0 1 2 3 4; do [[ "$index" == 0 ]] || printf ','; printf '{"index":%s,"tests":["%s"],"estimatedDurationMillis":0,"regex":"^(?:%s)$"}' "$index" "${names[$index]}" "${names[$index]}"; done
printf ']}\n'
`)
	resource := writeRaceFixture(t, "fake-resource", raceResourceFixture())
	out, err := runRacePackage(t, root, []string{
		"CI_RACE_GO_COMMAND=" + goCommand, "CI_RACE_SHARD_PLANNER=" + planner,
		"CI_RACE_RESOURCE_RUNNER=" + resource, "CI_RACE_CALL_LOG=" + logPath,
		"CI_RACE_TEST_NAMES=" + strings.Join(all, "\n") + "\n",
	}, "./internal/playground", "internal-playground")
	if err != nil {
		t.Fatalf("playground nine lanes failed: %v\n%s", err, out)
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	log := string(data)
	wantLanes := []string{"group-0", "group-1", "group-2", "group-3", "ordinary-0", "ordinary-1", "ordinary-2", "ordinary-3", "ordinary-4"}
	position := 0
	for _, lane := range wantLanes {
		marker := "start race-internal-playground-" + lane
		next := strings.Index(log[position:], marker)
		if next < 0 {
			t.Fatalf("missing sequential lane %q:\n%s", lane, log)
		}
		position += next + len(marker)
	}
	if strings.Count(log, "native test -json -race -count=1 -timeout=60m -run") != 9 {
		t.Fatalf("playground native lane count:\n%s", log)
	}
	var union struct {
		DiscoveredCount int   `json:"discovered_count"`
		ShardCounts     []int `json:"shard_counts"`
		Valid           bool  `json:"valid"`
	}
	unionData, err := os.ReadFile(filepath.Join(root, "ci-artifacts/race/internal-playground/union-validation.json"))
	if err != nil || json.Unmarshal(unionData, &union) != nil || union.DiscoveredCount != 9 || !reflect.DeepEqual(union.ShardCounts, []int{1, 1, 1, 1, 1, 1, 1, 1, 1}) || !union.Valid {
		t.Fatalf("playground union = %#v err=%v data=%s", union, err, unionData)
	}
}

func TestCIRacePackageRunnerRejectsInvalidEvidence(t *testing.T) {
	tests := map[string]struct {
		discovery           string
		planner             string
		events              string
		goStatus            string
		plannerMustFailFast bool
	}{
		"duplicate discovery":         {discovery: playgroundRaceDiscovery(playgroundGroupOne)},
		"malformed discovery":         {discovery: playgroundGroupOne + "/sub\n" + strings.Join([]string{playgroundGroupTwo, playgroundGroupThree, playgroundGroupFour}, "\n") + "\nok   github.com/glade-sh/glade/internal/playground 0.001s\n"},
		"fuzz discovery":              {discovery: playgroundRaceDiscovery("FuzzBytes")},
		"example discovery":           {discovery: playgroundRaceDiscovery("ExampleThing")},
		"unknown discovery":           {discovery: playgroundRaceDiscovery("HelperThing")},
		"group discovery extra":       {discovery: playgroundRaceDiscovery("TestExampleProjectsRunAnonymousOld")},
		"planner omission":            {discovery: playgroundRaceDiscovery("TestZeta"), planner: racePlannerOmissionFixture(), plannerMustFailFast: true},
		"planner duplicate":           {planner: racePlannerDuplicateFixture(), plannerMustFailFast: true},
		"planner empty shard":         {planner: racePlannerEmptyShardFixture(), plannerMustFailFast: true},
		"wrong package":               {planner: strings.Replace(racePlaygroundPlannerFixture(), `"$package"`, `"wrong/package"`, 1)},
		"missing terminal":            {events: ""},
		"duplicate terminal":          {events: "duplicate"},
		"ordinary missing terminal":   {events: "missing-ordinary"},
		"ordinary duplicate terminal": {events: "duplicate-ordinary"},
		"malformed event":             {events: "malformed"},
		"skipped terminal":            {events: "skip"},
		"failed terminal":             {events: "fail"},
		"native status":               {goStatus: "17"},
	}
	for name, fixture := range tests {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			logPath := filepath.Join(root, "calls.log")
			discovery := fixture.discovery
			if discovery == "" {
				discovery = playgroundRaceDiscovery()
			}
			goCommand := writeRaceFixture(t, "fake-go", `#!/usr/bin/env bash
if [[ "$*" == *"-list ."* ]]; then printf '%b' "$CI_RACE_DISCOVERY"; exit 0; fi
while IFS= read -r candidate; do
  if [[ "$*" == *"$candidate"* ]]; then test="$candidate"; break; fi
done <<<"$CI_RACE_TEST_NAMES"
case "$CI_RACE_EVENTS" in
  missing) ;;
  duplicate) printf '{"Action":"pass","Package":"github.com/glade-sh/glade/internal/playground","Test":"%s"}\n' "$test"; printf '{"Action":"pass","Package":"github.com/glade-sh/glade/internal/playground","Test":"%s"}\n' "$test" ;;
  missing-ordinary) if [[ "$test" == TestExampleProjectsRunAnonymous* ]]; then printf '{"Action":"pass","Package":"github.com/glade-sh/glade/internal/playground","Test":"%s"}\n' "$test"; fi ;;
  duplicate-ordinary)
    printf '{"Action":"pass","Package":"github.com/glade-sh/glade/internal/playground","Test":"%s"}\n' "$test"
    if [[ "$test" != TestExampleProjectsRunAnonymous* ]]; then printf '{"Action":"pass","Package":"github.com/glade-sh/glade/internal/playground","Test":"%s"}\n' "$test"; fi ;;
  malformed) printf '{not-json}\n' ;;
  skip|fail) printf '{"Action":"%s","Package":"github.com/glade-sh/glade/internal/playground","Test":"%s"}\n' "$CI_RACE_EVENTS" "$test" ;;
  *) printf '{"Action":"pass","Package":"github.com/glade-sh/glade/internal/playground","Test":"%s"}\n' "$test" ;;
esac
exit "${CI_RACE_GO_STATUS:-0}"
`)
			plannerText := fixture.planner
			if plannerText == "" {
				plannerText = racePlaygroundPlannerFixture()
			}
			if fixture.plannerMustFailFast && plannerText == racePlaygroundPlannerFixture() {
				t.Fatal("invalid planner fixture is identical to the valid planner fixture")
			}
			planner := writeRaceFixture(t, "fake-planner", plannerText)
			resource := writeRaceFixture(t, "fake-resource", raceResourceFixture())
			events := fixture.events
			if events == "" {
				events = "missing"
			}
			env := []string{
				"CI_RACE_GO_COMMAND=" + goCommand, "CI_RACE_SHARD_PLANNER=" + planner,
				"CI_RACE_RESOURCE_RUNNER=" + resource, "CI_RACE_CALL_LOG=" + logPath,
				"CI_RACE_DISCOVERY=" + discovery, "CI_RACE_EVENTS=" + events,
				"CI_RACE_GO_STATUS=" + fixture.goStatus,
				"CI_RACE_TEST_NAMES=" + strings.Join([]string{playgroundGroupOne, playgroundGroupTwo, playgroundGroupThree, playgroundGroupFour, "TestAlpha", "TestBeta", "TestDelta", "TestEpsilon", "TestGamma"}, "\n"),
			}
			out, err := runRacePackage(t, root, env, "./internal/playground", "internal-playground")
			if err == nil {
				t.Fatalf("invalid evidence accepted:\n%s", out)
			}
			if fixture.plannerMustFailFast {
				if data, readErr := os.ReadFile(logPath); readErr == nil && strings.Contains(string(data), "start race-") {
					t.Fatalf("invalid planner reached resource runner:\n%s", data)
				}
			}
			unionPath := filepath.Join(root, "ci-artifacts/race/internal-playground/union-validation.json")
			if data, readErr := os.ReadFile(unionPath); readErr == nil {
				var union map[string]any
				if json.Unmarshal(data, &union) == nil && union["valid"] == true {
					t.Fatalf("invalid evidence left a valid union: %s", data)
				}
			}
		})
	}
}

func TestCIRacePackageRunnerRejectsInvalidResourceEvidence(t *testing.T) {
	valid := `{"schema_version":1,"lane":"race-internal-storage","elapsed_seconds":0,"user_seconds":0,"system_seconds":0,"max_rss_kb":0,"exit_status":0}`
	tests := map[string]string{
		"duplicate key":    strings.Replace(valid, `"schema_version":1`, `"schema_version":1,"schema_version":1`, 1),
		"malformed schema": `{}`,
		"wrong lane":       strings.Replace(valid, "race-internal-storage", "race-wrong", 1),
		"wrong status":     strings.Replace(valid, `"exit_status":0`, `"exit_status":1`, 1),
		"wrong version":    strings.Replace(valid, `"schema_version":1`, `"schema_version":2`, 1),
		"boolean version":  strings.Replace(valid, `"schema_version":1`, `"schema_version":true`, 1),
		"float version":    strings.Replace(valid, `"schema_version":1`, `"schema_version":1.0`, 1),
		"float status":     strings.Replace(valid, `"exit_status":0`, `"exit_status":0.0`, 1),
		"boolean number":   strings.Replace(valid, `"max_rss_kb":0`, `"max_rss_kb":true`, 1),
		"nonnumeric":       strings.Replace(valid, `"elapsed_seconds":0`, `"elapsed_seconds":"zero"`, 1),
	}
	for name, evidence := range tests {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			goCommand := writeRaceFixture(t, "fake-go", "#!/usr/bin/env bash\nexit 0\n")
			resource := writeRaceFixture(t, "fake-resource", `#!/usr/bin/env bash
set -euo pipefail
output="$1"
shift 3
"$@"
mkdir -p "$(dirname "$output")"
printf '%s\n' "$CI_RACE_RESOURCE_JSON" >"$output"
`)
			out, err := runRacePackage(t, root, []string{"CI_RACE_GO_COMMAND=" + goCommand, "CI_RACE_RESOURCE_RUNNER=" + resource, "CI_RACE_RESOURCE_JSON=" + evidence}, "./internal/storage", "internal-storage")
			if err == nil {
				t.Fatalf("invalid resource evidence accepted:\n%s", out)
			}
		})
	}
}

func TestCIRacePackageRunnerPreservesNativeStatusWhenResourceEvidenceIsInvalid(t *testing.T) {
	for _, fixture := range []struct {
		name     string
		pkg      string
		slug     string
		status   int
		resource string
	}{
		{name: "direct missing", pkg: "./internal/storage", slug: "internal-storage", status: 17},
		{name: "direct malformed", pkg: "./internal/storage", slug: "internal-storage", status: 18, resource: "{bad-json}"},
		{name: "shard missing", pkg: "./internal/playground", slug: "internal-playground", status: 19},
		{name: "shard malformed", pkg: "./internal/playground", slug: "internal-playground", status: 20, resource: "{bad-json}"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			root := t.TempDir()
			goCommand := writeRaceFixture(t, "fake-go", `#!/usr/bin/env bash
if [[ "$*" == *"-list ."* ]]; then
  printf 'TestExampleProjectsRunAnonymousGroupOne\nTestExampleProjectsRunAnonymousGroupTwo\nTestExampleProjectsRunAnonymousGroupThree\nTestExampleProjectsRunAnonymousGroupFour\nTestAlpha\nTestBeta\nTestDelta\nTestEpsilon\nTestGamma\nok   github.com/glade-sh/glade/internal/playground 0.001s\n'
  exit 0
fi
exit "$CI_RACE_NATIVE_STATUS"
`)
			planner := writeRaceFixture(t, "fake-planner", racePlaygroundPlannerFixture())
			resource := writeRaceFixture(t, "fake-resource", `#!/usr/bin/env bash
set -euo pipefail
output="$1"
shift 3
set +e
"$@"
rc="$?"
set -e
if [[ -n "${CI_RACE_RESOURCE_JSON:-}" ]]; then mkdir -p "$(dirname "$output")"; printf '%s\n' "$CI_RACE_RESOURCE_JSON" >"$output"; fi
exit "$rc"
`)
			env := []string{
				"CI_RACE_GO_COMMAND=" + goCommand, "CI_RACE_RESOURCE_RUNNER=" + resource,
				"CI_RACE_SHARD_PLANNER=" + planner, "CI_RACE_NATIVE_STATUS=" + fmt.Sprint(fixture.status),
				"CI_RACE_RESOURCE_JSON=" + fixture.resource,
			}
			out, err := runRacePackage(t, root, env, fixture.pkg, fixture.slug)
			if got := raceExitCode(err); got != fixture.status {
				t.Fatalf("exit code = %d, want native %d\n%s", got, fixture.status, out)
			}
		})
	}
}

func TestCIRacePackageRunnerRejectsWrapperFailure(t *testing.T) {
	root := t.TempDir()
	goCommand := writeRaceFixture(t, "fake-go", `#!/usr/bin/env bash
if [[ "$*" == *"-list ."* ]]; then printf 'TestExampleProjectsRunAnonymousGroupOne\nTestExampleProjectsRunAnonymousGroupTwo\nTestExampleProjectsRunAnonymousGroupThree\nTestExampleProjectsRunAnonymousGroupFour\nTestAlpha\nTestBeta\nTestDelta\nTestEpsilon\nTestGamma\nok   github.com/glade-sh/glade/internal/playground 0.001s\n'; fi
`)
	planner := writeRaceFixture(t, "fake-planner", racePlaygroundPlannerFixture())
	resource := writeRaceFixture(t, "fake-resource", "#!/usr/bin/env bash\nexit 23\n")
	out, err := runRacePackage(t, root, []string{"CI_RACE_GO_COMMAND=" + goCommand, "CI_RACE_SHARD_PLANNER=" + planner, "CI_RACE_RESOURCE_RUNNER=" + resource}, "./internal/playground", "internal-playground")
	if err == nil {
		t.Fatalf("wrapper failure accepted:\n%s", out)
	}
}

func TestCIRacePackageRunnerRejectsMissingResourceEvidence(t *testing.T) {
	root := t.TempDir()
	goCommand := writeRaceFixture(t, "fake-go", `#!/usr/bin/env bash
if [[ "$*" == *"-list ."* ]]; then
  printf 'TestExampleProjectsRunAnonymousGroupOne\nTestExampleProjectsRunAnonymousGroupTwo\nTestExampleProjectsRunAnonymousGroupThree\nTestExampleProjectsRunAnonymousGroupFour\nTestAlpha\nTestBeta\nTestDelta\nTestEpsilon\nTestGamma\nok   github.com/glade-sh/glade/internal/playground 0.001s\n'
else
  for test in TestExampleProjectsRunAnonymousGroupOne TestExampleProjectsRunAnonymousGroupTwo TestExampleProjectsRunAnonymousGroupThree TestExampleProjectsRunAnonymousGroupFour; do
    if [[ "$*" == *"$test"* ]]; then printf '{"Action":"pass","Package":"github.com/glade-sh/glade/internal/playground","Test":"%s"}\n' "$test"; break; fi
  done
fi
`)
	planner := writeRaceFixture(t, "fake-planner", racePlaygroundPlannerFixture())
	resource := writeRaceFixture(t, "fake-resource", "#!/usr/bin/env bash\nshift 3\n\"$@\"\n")
	out, err := runRacePackage(t, root, []string{"CI_RACE_GO_COMMAND=" + goCommand, "CI_RACE_SHARD_PLANNER=" + planner, "CI_RACE_RESOURCE_RUNNER=" + resource}, "./internal/playground", "internal-playground")
	if err == nil {
		t.Fatalf("missing resource evidence accepted:\n%s", out)
	}
}

func TestCIRacePackageRunnerRejectsMissingOrdinaryResourceEvidence(t *testing.T) {
	root := t.TempDir()
	logPath := filepath.Join(root, "calls.log")
	all := []string{playgroundGroupOne, playgroundGroupTwo, playgroundGroupThree, playgroundGroupFour, "TestAlpha", "TestBeta", "TestDelta", "TestEpsilon", "TestGamma"}
	goCommand := writeRaceFixture(t, "fake-go", `#!/usr/bin/env bash
if [[ "$*" == *"-list ."* ]]; then printf '%b' "$CI_RACE_TEST_NAMES"; printf 'ok   github.com/glade-sh/glade/internal/playground 0.001s\n'; exit 0; fi
while IFS= read -r test; do if [[ "$*" == *"$test"* ]]; then printf '{"Action":"pass","Package":"github.com/glade-sh/glade/internal/playground","Test":"%s"}\n' "$test"; break; fi; done <<<"$CI_RACE_TEST_NAMES"
`)
	planner := writeRaceFixture(t, "fake-planner", racePlaygroundPlannerFixture())
	resource := writeRaceFixture(t, "fake-resource", `#!/usr/bin/env bash
set -euo pipefail
output="$1"; lane="$2"; shift 3
printf 'start %s\n' "$lane" >>"$CI_RACE_CALL_LOG"
"$@"
if [[ "$lane" != *ordinary-0 ]]; then mkdir -p "$(dirname "$output")"; printf '{"schema_version":1,"lane":"%s","elapsed_seconds":0,"user_seconds":0,"system_seconds":0,"max_rss_kb":0,"exit_status":0}\n' "$lane" >"$output"; fi
`)
	out, err := runRacePackage(t, root, []string{
		"CI_RACE_GO_COMMAND=" + goCommand, "CI_RACE_SHARD_PLANNER=" + planner,
		"CI_RACE_RESOURCE_RUNNER=" + resource, "CI_RACE_CALL_LOG=" + logPath,
		"CI_RACE_TEST_NAMES=" + strings.Join(all, "\n") + "\n",
	}, "./internal/playground", "internal-playground")
	if err == nil {
		t.Fatalf("missing ordinary resource evidence accepted:\n%s", out)
	}
	data, readErr := os.ReadFile(logPath)
	if readErr != nil || !strings.Contains(string(data), "start race-internal-playground-group-3") || !strings.Contains(string(data), "start race-internal-playground-ordinary-0") || strings.Contains(string(data), "start race-internal-playground-ordinary-1") {
		t.Fatalf("ordinary resource failure lane order:\n%s err=%v", data, readErr)
	}
}
