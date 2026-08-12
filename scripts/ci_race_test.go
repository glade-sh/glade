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
	"slices"
	"sort"
	"strings"
	"syscall"
	"testing"
	"time"
)

const (
	playgroundGroupOne             = "TestExampleProjectsRunAnonymousGroupOne"
	playgroundGroupTwo             = "TestExampleProjectsRunAnonymousGroupTwo"
	playgroundGroupThree           = "TestExampleProjectsRunAnonymousGroupThree"
	playgroundGroupFour            = "TestExampleProjectsRunAnonymousGroupFour"
	ciRaceStartupFallback          = 30 * time.Second
	ciRacePostSignalCleanupReserve = 3 * time.Second
)

func ciRaceStartupDeadline(now, testDeadline time.Time, hasTestDeadline bool) time.Time {
	if !hasTestDeadline {
		return now.Add(ciRaceStartupFallback)
	}
	startupDeadline := testDeadline.Add(-ciRacePostSignalCleanupReserve)
	if startupDeadline.Before(now) {
		return now
	}
	return startupDeadline
}

func stopCIRaceCommand(cmd *exec.Cmd, waited <-chan error) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	<-waited
}

func TestCIRaceStartupDeadlineUsesBoundedFallbackAndReservesSignalCleanup(t *testing.T) {
	now := time.Date(2026, time.July, 27, 12, 0, 0, 0, time.UTC)
	if got, want := ciRaceStartupDeadline(now, time.Time{}, false), now.Add(ciRaceStartupFallback); !got.Equal(want) {
		t.Fatalf("no-deadline startup bound = %s, want %s", got, want)
	}
	testDeadline := now.Add(10 * time.Second)
	if got, want := ciRaceStartupDeadline(now, testDeadline, true), testDeadline.Add(-ciRacePostSignalCleanupReserve); !got.Equal(want) {
		t.Fatalf("test-deadline startup bound = %s, want %s", got, want)
	}
}

func TestStopCIRaceCommandKillsAndWaits(t *testing.T) {
	cmd := exec.Command("sleep", "300")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waited := make(chan error, 1)
	go func() { waited <- cmd.Wait() }()

	stopCIRaceCommand(cmd, waited)
	if cmd.ProcessState == nil {
		t.Fatal("race command was not reaped")
	}
	if err := cmd.Process.Signal(syscall.Signal(0)); err == nil {
		t.Fatal("race command still accepts signals after cleanup")
	}
}

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
		"./internal/sema", "./internal/semanticcache", "./internal/server", "./internal/startupcache", "./internal/storage",
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
	if len(got) != 64 || !reflect.DeepEqual(got, want) {
		t.Fatalf("full packages = %d/%v, want 64 exact manifest packages", len(got), got)
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
	if len(got) != 64 || !strings.Contains(diagnostic, "git diff failed") {
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
	if len(got) != 64 || !strings.Contains(diagnostic, "deleted Go file") {
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
	if len(got) != 64 {
		t.Fatalf("nested module change selected %d packages, want full 64", len(got))
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
	if len(got) != 64 || !strings.Contains(diagnostic, "dependency graph failed") {
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
		"name: Race", "branches: [main]", "schedule:", "workflow_dispatch:",
		"fetch-depth: 0", "scripts/ci-race-packages.sh", "fromJSON(needs.plan.outputs.packages)",
		"fail-fast: false", "max-parallel: 16", "GOMAXPROCS: \"2\"", "go-version: \"1.26.5\"",
		`scripts/ci-race-test.sh "$PACKAGE" "$SLUG"`, "if: always()",
		"actions/upload-artifact@b7c566a772e6b6bfb58ed0dc250532a479d7789f # v6.0.0", "ci-artifacts/race/", "if-no-files-found: error",
		"npm ci --prefix third_party/lwc", "contains(fromJSON('[\"./internal/gladecli\"",
		"non-apextest-packages", "has-non-apextest-packages", "race-apextest-a:", "race-apextest-b:", "race-apextest-aggregate:",
		"CI_RACE_APEXTEST_RUNNER: a", "CI_RACE_APEXTEST_SHARD_INDEXES: 0,2,4,5", "CI_RACE_APEXTEST_RUNNER: b", "CI_RACE_APEXTEST_SHARD_INDEXES: 1,3,6,7",
		"CI_RACE_HEAD_SHA: ${{ github.sha }}",
		"if: always() && contains(fromJSON(needs.plan.outputs.packages), './internal/apextest')", "scripts/ci-race-apextest-aggregate.sh ci-artifacts/race/apextest-a ci-artifacts/race/apextest-b", "actions/download-artifact@d3f86a106a0bac45b974a628896c90dbdf5c8093 # v4.3.0",
	} {
		if !strings.Contains(workflow, marker) {
			t.Errorf("race workflow missing %q", marker)
		}
	}
	for _, forbidden := range []string{"pull_request:", "pull_request_target:", "continue-on-error:", "runs-on: ubuntu-latest-", "go test -race ./..."} {
		if strings.Contains(workflow, forbidden) {
			t.Errorf("race workflow contains forbidden %q", forbidden)
		}
	}
	if !strings.Contains(workflow, "push)\n              mode=full\n              packages=\"$(scripts/ci-race-packages.sh full)\"") {
		t.Fatal("race workflow must run the exhaustive manifest after every main push")
	}
	if !strings.Contains(workflow, "group: race-${{ github.run_id }}") || strings.Contains(workflow, "group: race-${{ github.ref") {
		t.Fatal("race workflow must give every event a unique concurrency group")
	}
	if strings.Contains(workflow, "Required CI") || strings.Contains(workflow, "required-ci") {
		t.Fatal("race workflow is coupled to normal Required CI")
	}
	if strings.Contains(workflow, "scripts/ci-resource-run.sh") || strings.Contains(workflow, "go test -race") {
		t.Fatal("race workflow bypasses the package race runner")
	}
	planJob := workflow[strings.Index(workflow, "  plan:"):strings.Index(workflow, "  race:")]
	raceJob := workflow[strings.Index(workflow, "  race:"):strings.Index(workflow, "  race-apextest-a:")]
	for _, marker := range []string{
		"timeout-minutes: 15",
		"name: Resolve race npm cache path",
		"id: race-npm-cache",
		"name: Restore race npm cache",
		"id: race-npm-cache-restore",
		"actions/cache/restore@0057852bfaa89a56745cba8c7296529d2fc39830 # v4.3.0",
		"name: Install shared LWC toolchain",
		"npm ci --prefix third_party/lwc",
		"name: Save race npm cache",
		"actions/cache/save@0057852bfaa89a56745cba8c7296529d2fc39830 # v4.3.0",
		"path: ${{ steps.race-npm-cache.outputs.dir }}",
		"key: race-npm-v1-${{ runner.os }}-${{ runner.arch }}-22-${{ hashFiles('third_party/lwc/package-lock.json') }}",
		"key: ${{ steps.race-npm-cache-restore.outputs.cache-primary-key }}",
		"if: ${{ contains(fromJSON(steps.classify.outputs.non-apextest-packages), './internal/gladecli') }}",
	} {
		if !strings.Contains(planJob, marker) {
			t.Errorf("race plan cache owner missing %q", marker)
		}
	}
	if got := strings.Count(planJob, "actions/cache/save@0057852bfaa89a56745cba8c7296529d2fc39830 # v4.3.0"); got != 1 {
		t.Fatalf("race plan npm cache save count = %d, want one static owner", got)
	}
	if strings.Contains(planJob, "cache: npm") {
		t.Fatal("race plan setup-node must not own an implicit npm cache")
	}
	if got := strings.Count(raceJob, "actions/setup-node@249970729cb0ef3589644e2896645e5dc5ba9c38 # v6.5.0"); got != 1 {
		t.Fatalf("race Node setup count = %d, want one setup shared by all LWC consumers", got)
	}
	if strings.Contains(raceJob, "cache: npm") {
		t.Fatal("race setup-node must not own an implicit npm cache")
	}
	for _, marker := range []string{
		"name: Resolve npm cache path",
		"id: npm-cache",
		"name: Restore LWC npm cache",
		"id: npm-cache-restore",
		"actions/cache/restore@0057852bfaa89a56745cba8c7296529d2fc39830 # v4.3.0",
		"path: ${{ steps.npm-cache.outputs.dir }}",
		"key: race-npm-v1-${{ runner.os }}-${{ runner.arch }}-22-${{ hashFiles('third_party/lwc/package-lock.json') }}",
	} {
		if !strings.Contains(raceJob, marker) {
			t.Errorf("race explicit npm cache missing %q", marker)
		}
	}
	const lwcConsumers = `contains(fromJSON('["./internal/gladecli","./internal/gladehome","./internal/lwc/compile","./internal/lwcbrowser","./internal/server"]'), matrix.package)`
	if got := strings.Count(raceJob, lwcConsumers); got != 4 {
		t.Fatalf("race LWC consumer selector count = %d, want setup, path, restore, and install", got)
	}
	if strings.Contains(raceJob, "actions/cache/save") || strings.Contains(raceJob, "cache: npm") {
		t.Fatal("dynamic race matrix must be restore-only for npm cache")
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
	if pkg == "./internal/apextest" || pkg == "./internal/gladecli" || pkg == "./internal/playground" || pkg == "./internal/server" {
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
shards=""
while [[ "$#" -gt 0 ]]; do
  case "$1" in
    --package) package="$2"; shift 2 ;;
    --shards) shards="$2"; shift 2 ;;
    --tests) tests="$2"; shift 2 ;;
    *) exit 91 ;;
  esac
done
names=()
while IFS= read -r name; do names+=("$name"); done <"$tests"
printf '{"version":1,"package":"%s","historyUsed":false,"shards":[' "$package"
for ((index=0; index<shards; index++)); do
  if [[ "$index" -gt 0 ]]; then printf ','; fi
  printf '{"index":%s,"tests":["%s"],"estimatedDurationMillis":0,"regex":"^(?:%s)$"}' "$index" "${names[$index]}" "${names[$index]}"
done
printf ']}\n'
`
}

func raceApextestPlannerFixture() string {
	return `#!/usr/bin/env bash
set -euo pipefail
package=""
tests=""
while [[ "$#" -gt 0 ]]; do
  case "$1" in
    --package) package="$2"; shift 2 ;;
    --shards) [[ "$2" == 8 ]]; shift 2 ;;
    --tests) tests="$2"; shift 2 ;;
    *) exit 91 ;;
  esac
done
names=()
while IFS= read -r name; do names+=("$name"); done <"$tests"
[[ "${#names[@]}" == 8 ]]
printf '{"version":1,"package":"%s","historyUsed":false,"shards":[' "$package"
for index in 0 1 2 3 4 5 6 7; do
  [[ "$index" == 0 ]] || printf ','
  printf '{"index":%s,"tests":["%s"],"estimatedDurationMillis":0,"regex":"^(?:%s)$"}' "$index" "${names[$index]}" "${names[$index]}"
done
printf ']}\n'
`
}

func raceApextestGoFixture() string {
	return `#!/usr/bin/env bash
set -euo pipefail
line=go; for argument in "$@"; do printf -v quoted '%q' "$argument"; line+=" $quoted"; done; printf '%s\n' "$line" >>"$CI_RACE_CALL_LOG"
if [[ "$*" == *"-list ."* ]]; then
  exit 98
fi
if [[ "$1" == test && "$*" == *" -c "* ]]; then
  if [[ "${CI_RACE_BUILD_STATUS:-0}" != 0 ]]; then exit "$CI_RACE_BUILD_STATUS"; fi
  output=""
  while [[ "$#" -gt 0 ]]; do if [[ "$1" == -o ]]; then output="$2"; break; fi; shift; done
  [[ -n "$output" ]]
  if [[ "${CI_RACE_BINARY_MODE:-}" == absent ]]; then exit 0; fi
  printf '%s\n' '#!/usr/bin/env bash' 'set -euo pipefail' >"$output"
  cat >>"$output" <<'BIN'
line=binary; for argument in "$@"; do printf -v quoted '%q' "$argument"; line+=" $quoted"; done; printf '%s\n' "$line" >>"$CI_RACE_CALL_LOG"
if [[ "$*" == *"-test.list ."* ]]; then
  case "${CI_RACE_BINARY_DISCOVERY_MODE:-valid}" in
    failure) exit "${CI_RACE_BINARY_DISCOVERY_STATUS:-32}" ;;
    block) printf '%s\n' "$$" >"$CI_RACE_RUNNER_PID"; sleep 300 ;;
    malformed) printf 'HelperThing\n' ;;
    fuzz) printf 'FuzzBytes\n' ;;
    example) printf 'ExampleThing\n' ;;
    duplicate) printf '%b' "$CI_RACE_TEST_NAMES"; printf 'TestAlpha\nBenchmarkFixture\n' ;;
    pass-duplicate) printf '%b' "$CI_RACE_TEST_NAMES"; printf 'BenchmarkFixture\nPASS\nPASS\n' ;;
    go-trailer) printf '%b' "$CI_RACE_TEST_NAMES"; printf 'BenchmarkFixture\nok   github.com/glade-sh/glade/internal/apextest 0.001s\n' ;;
    tampered) printf '# tampered during discovery\n' >>"$0"; printf '%b' "$CI_RACE_TEST_NAMES"; printf 'BenchmarkFixture\n' ;;
    missing) rm -f "$0"; printf '%b' "$CI_RACE_TEST_NAMES"; printf 'BenchmarkFixture\n' ;;
    *) printf '%b' "$CI_RACE_TEST_NAMES"; printf 'BenchmarkFixture\n'; [[ "${CI_RACE_BINARY_DISCOVERY_PASS:-0}" == 1 ]] && printf 'PASS\n' ;;
  esac
  exit 0
fi
while IFS= read -r candidate; do if [[ "$*" == *"$candidate"* ]]; then test="$candidate"; break; fi; done <<<"$CI_RACE_TEST_NAMES"
case "${CI_RACE_DIRECT_EVENTS:-valid}" in
  malformed) printf '{bad-json}\n' ;;
  duplicate) printf '{"Action":"pass","Package":"github.com/glade-sh/glade/internal/apextest","Test":"%s"}\n' "$test"; printf '{"Action":"pass","Package":"github.com/glade-sh/glade/internal/apextest","Test":"%s"}\n' "$test" ;;
  missing) ;;
  skip|fail) printf '{"Action":"%s","Package":"github.com/glade-sh/glade/internal/apextest","Test":"%s"}\n' "$CI_RACE_DIRECT_EVENTS" "$test" ;;
  *) printf '{"Action":"run","Package":"github.com/glade-sh/glade/internal/apextest","Test":"%s"}\n' "$test"; printf '{"Action":"pass","Package":"github.com/glade-sh/glade/internal/apextest","Test":"%s"}\n' "$test" ;;
esac
if [[ "${CI_RACE_DIRECT_FAIL_TEST:-}" == "$test" ]]; then exit "${CI_RACE_DIRECT_STATUS:-31}"; fi
BIN
  chmod +x "$output"
  exit 0
fi
if [[ "$1" == tool && "$2" == test2json ]]; then
  cat
  exit "${CI_RACE_TEST2JSON_STATUS:-0}"
fi
exit 99
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

func TestCIRacePackageRunnerGenericHeavyPackagesUseBoundedSequentialExactShards(t *testing.T) {
	for _, pkg := range []string{"./internal/gladecli", "./internal/server"} {
		t.Run(filepath.Base(pkg), func(t *testing.T) {
			root := t.TempDir()
			logPath := filepath.Join(root, "calls.log")
			testNames := []string{"TestAlpha", "TestBeta", "TestDelta", "TestGamma"}
			if pkg == "./internal/server" {
				testNames = append(testNames, "TestEta", "TestIota", "TestKappa", "TestTheta")
			}
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
			shardCount := 4
			if pkg == "./internal/server" {
				shardCount = 8
			}
			var wantOrder []string
			for shard := 0; shard < shardCount; shard++ {
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
			testPrefix := "test"
			if pkg == "./internal/server" {
				testPrefix = "test -p=1"
			}
			if strings.Count(log, "native "+testPrefix+" -json -race -count=1 -timeout=60m -run") != shardCount {
				t.Fatalf("native heavy calls:\n%s", log)
			}
			if strings.Count(log, "discovery "+testPrefix+" -race -list . "+pkg) != 1 {
				t.Fatalf("heavy discovery did not use the exact race-tagged test set:\n%s", log)
			}
			if strings.Count(log, "deadline --signal=TERM --kill-after=30s 60m") != 1 {
				t.Fatalf("heavy package did not use exactly one aggregate deadline:\n%s", log)
			}
			benchmarks, err := os.ReadFile(filepath.Join(root, "ci-artifacts/race", slug, "discovery-benchmarks.txt"))
			if err != nil || string(benchmarks) != "BenchmarkFixture\n" {
				t.Fatalf("recorded benchmarks = %q, err=%v", benchmarks, err)
			}
			for shard := 0; shard < shardCount; shard++ {
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
			wantShardCounts := make([]int, shardCount)
			for index := range wantShardCounts {
				wantShardCounts[index] = 1
			}
			if union.SchemaVersion != 1 || union.Package != "github.com/glade-sh/glade/"+strings.TrimPrefix(pkg, "./") ||
				union.DiscoveredCount != shardCount || !reflect.DeepEqual(union.ShardCounts, wantShardCounts) ||
				union.NamesSHA256 != namesHash || !union.Valid {
				t.Fatalf("union evidence = %#v", union)
			}
		})
	}
}

func TestCIRacePackageRunnerApextestBuildsOnceAndUsesEightDirectShards(t *testing.T) {
	root := t.TempDir()
	logPath := filepath.Join(root, "calls.log")
	testNames := []string{"TestAlpha", "TestBeta", "TestDelta", "TestEpsilon", "TestEta", "TestGamma", "TestTheta", "TestZeta"}
	goCommand := writeRaceFixture(t, "fake-go", raceApextestGoFixture())
	planner := writeRaceFixture(t, "fake-planner", raceApextestPlannerFixture())
	resource := writeRaceFixture(t, "fake-resource", raceResourceFixture())
	out, err := runRacePackage(t, root, []string{
		"CI_RACE_GO_COMMAND=" + goCommand, "CI_RACE_SHARD_PLANNER=" + planner,
		"CI_RACE_RESOURCE_RUNNER=" + resource, "CI_RACE_CALL_LOG=" + logPath,
		"CI_RACE_TEST_NAMES=" + strings.Join(testNames, "\n") + "\n",
		"CI_RACE_BINARY_DISCOVERY_PASS=1",
		"TMPDIR=" + root,
	}, "./internal/apextest", "internal-apextest")
	if err != nil {
		t.Fatalf("apextest direct shards failed: %v\n%s", err, out)
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	log := string(data)
	if strings.Count(log, "go test ") != 1 || strings.Count(log, "go test -race -c -o") != 1 || strings.Count(log, "go tool test2json -p github.com/glade-sh/glade/internal/apextest") != 8 {
		t.Fatalf("apextest build/test2json calls:\n%s", log)
	}
	if strings.Count(log, "binary -test.list .") != 1 || strings.Count(log, "binary -test.v=test2json -test.count=1 -test.timeout=60m -test.run=") != 8 || strings.Contains(log, "go test -json -race") || strings.Contains(log, "go test -race -list") {
		t.Fatalf("apextest direct binary calls:\n%s", log)
	}
	for shard := 0; shard < 8; shard++ {
		if !strings.Contains(log, "start race-internal-apextest-shard-"+fmt.Sprint(shard)) {
			t.Fatalf("missing shard %d resource lane:\n%s", shard, log)
		}
	}
	metadataPath := filepath.Join(root, "ci-artifacts/race/internal-apextest/binary.json")
	metadataData, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatal(err)
	}
	var metadata struct {
		SchemaVersion int    `json:"schema_version"`
		Package       string `json:"package"`
		SHA256        string `json:"sha256"`
		SizeBytes     int64  `json:"size_bytes"`
		Removed       bool   `json:"removed"`
	}
	if err := json.Unmarshal(metadataData, &metadata); err != nil || metadata.SchemaVersion != 1 || metadata.Package != "github.com/glade-sh/glade/internal/apextest" || len(metadata.SHA256) != 64 || metadata.SizeBytes <= 0 || !metadata.Removed {
		t.Fatalf("binary metadata = %#v err=%v data=%s", metadata, err, metadataData)
	}
	unionData, err := os.ReadFile(filepath.Join(root, "ci-artifacts/race/internal-apextest/union-validation.json"))
	var union struct {
		DiscoveredCount int   `json:"discovered_count"`
		ShardCounts     []int `json:"shard_counts"`
		Valid           bool  `json:"valid"`
	}
	if err != nil || json.Unmarshal(unionData, &union) != nil || union.DiscoveredCount != 8 || !reflect.DeepEqual(union.ShardCounts, []int{1, 1, 1, 1, 1, 1, 1, 1}) || !union.Valid {
		t.Fatalf("apextest union = %#v err=%v data=%s", union, err, unionData)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), "apextest.test") {
			t.Fatalf("temporary race binary remained: %s", entry.Name())
		}
	}
}

func TestCIRacePackageRunnerApextestRunsOnlyAssignedShards(t *testing.T) {
	root := t.TempDir()
	logPath := filepath.Join(root, "calls.log")
	testNames := []string{"TestAlpha", "TestBeta", "TestDelta", "TestEpsilon", "TestEta", "TestGamma", "TestTheta", "TestZeta"}
	goCommand := writeRaceFixture(t, "fake-go", raceApextestGoFixture())
	planner := writeRaceFixture(t, "fake-planner", raceApextestPlannerFixture())
	resource := writeRaceFixture(t, "fake-resource", raceResourceFixture())
	gitCommand := writeRaceFixture(t, "fake-git", "#!/usr/bin/env bash\nprintf '%s\\n' 0123456789abcdef0123456789abcdef01234567\n")
	out, err := runRacePackage(t, root, []string{
		"CI_RACE_GO_COMMAND=" + goCommand, "CI_RACE_SHARD_PLANNER=" + planner,
		"CI_RACE_RESOURCE_RUNNER=" + resource, "CI_RACE_GIT_COMMAND=" + gitCommand, "CI_RACE_CALL_LOG=" + logPath,
		"CI_RACE_TEST_NAMES=" + strings.Join(testNames, "\n") + "\n",
		"CI_RACE_BINARY_DISCOVERY_PASS=1", "CI_RACE_APEXTEST_SHARD_INDEXES=1,3,6,7",
		"CI_RACE_APEXTEST_RUNNER=b", "CI_RACE_HEAD_SHA=0123456789abcdef0123456789abcdef01234567",
		"TMPDIR=" + root,
	}, "./internal/apextest", "internal-apextest")
	if err != nil {
		t.Fatalf("assigned apex shard failed: %v\n%s", err, out)
	}
	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	log := string(logData)
	if strings.Count(log, "start race-internal-apextest-shard-") != 4 {
		t.Fatalf("assigned runner started unexpected shard lanes:\n%s", log)
	}
	for _, index := range []int{1, 3, 6, 7} {
		if !strings.Contains(log, fmt.Sprintf("start race-internal-apextest-shard-%d", index)) {
			t.Fatalf("assigned runner omitted shard %d:\n%s", index, log)
		}
	}
	artifactDir := filepath.Join(root, "ci-artifacts", "race", "internal-apextest")
	for _, index := range []int{1, 3, 6, 7} {
		if _, err := os.Stat(filepath.Join(artifactDir, fmt.Sprintf("shard-%d", index), "events.json")); err != nil {
			t.Fatalf("assigned shard %d evidence: %v", index, err)
		}
	}
	if _, err := os.Stat(filepath.Join(artifactDir, "shard-0", "events.json")); !os.IsNotExist(err) {
		t.Fatalf("unassigned shard evidence exists or failed unexpectedly: %v", err)
	}
	runnerData, err := os.ReadFile(filepath.Join(artifactDir, "runner-validation.json"))
	if err != nil {
		t.Fatalf("runner validation evidence: %v", err)
	}
	var runner struct {
		Runner          string `json:"runner"`
		HeadSHA         string `json:"head_sha"`
		AssignedIndexes []int  `json:"assigned_indexes"`
		DiscoveredCount int    `json:"discovered_count"`
		BinaryRemoved   bool   `json:"binary_removed"`
	}
	if err := json.Unmarshal(runnerData, &runner); err != nil || runner.Runner != "b" || runner.HeadSHA != "0123456789abcdef0123456789abcdef01234567" || !reflect.DeepEqual(runner.AssignedIndexes, []int{1, 3, 6, 7}) || runner.DiscoveredCount != 8 || !runner.BinaryRemoved {
		t.Fatalf("runner validation = %#v err=%v data=%s", runner, err, runnerData)
	}
}

func TestCIRacePackageRunnerApextestRejectsUnexpectedCheckoutHead(t *testing.T) {
	root := t.TempDir()
	logPath := filepath.Join(root, "calls.log")
	testNames := []string{"TestAlpha", "TestBeta", "TestDelta", "TestEpsilon", "TestEta", "TestGamma", "TestTheta", "TestZeta"}
	goCommand := writeRaceFixture(t, "fake-go", raceApextestGoFixture())
	planner := writeRaceFixture(t, "fake-planner", raceApextestPlannerFixture())
	resource := writeRaceFixture(t, "fake-resource", raceResourceFixture())
	gitCommand := writeRaceFixture(t, "fake-git", "#!/usr/bin/env bash\nprintf '%s\\n' fedcba9876543210fedcba9876543210fedcba98\n")
	out, err := runRacePackage(t, root, []string{
		"CI_RACE_GO_COMMAND=" + goCommand, "CI_RACE_SHARD_PLANNER=" + planner,
		"CI_RACE_RESOURCE_RUNNER=" + resource, "CI_RACE_GIT_COMMAND=" + gitCommand,
		"CI_RACE_CALL_LOG=" + logPath, "CI_RACE_TEST_NAMES=" + strings.Join(testNames, "\n") + "\n",
		"CI_RACE_APEXTEST_SHARD_INDEXES=1,3,6,7", "CI_RACE_APEXTEST_RUNNER=b",
		"CI_RACE_HEAD_SHA=0123456789abcdef0123456789abcdef01234567", "TMPDIR=" + root,
	}, "./internal/apextest", "internal-apextest")
	if err == nil || !strings.Contains(out, "checkout HEAD does not match expected head") {
		t.Fatalf("unexpected checkout head accepted: %v\n%s", err, out)
	}
	if data, readErr := os.ReadFile(logPath); readErr == nil && strings.Contains(string(data), "start race-") {
		t.Fatalf("unexpected checkout started a resource lane:\n%s", data)
	}
}

func writeApextestAggregateArtifact(t *testing.T, root, runner string, indexes []int, headSHA string, testCount int) {
	t.Helper()
	artifactDir := filepath.Join(root, runner)
	if err := os.MkdirAll(artifactDir, 0o700); err != nil {
		t.Fatal(err)
	}
	names := make([]string, testCount)
	for index := range names {
		names[index] = fmt.Sprintf("TestCase%03d", index)
	}
	discovery := strings.Join(names, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(artifactDir, "discovery.txt"), []byte(discovery), 0o600); err != nil {
		t.Fatal(err)
	}
	shards := make([]map[string]any, 8)
	for index := range shards {
		shardTests := make([]string, 0, 42)
		for nameIndex := index; nameIndex < len(names); nameIndex += len(shards) {
			shardTests = append(shardTests, names[nameIndex])
		}
		shards[index] = map[string]any{"index": index, "tests": shardTests, "estimatedDurationMillis": 0, "regex": "^(?:" + strings.Join(shardTests, "|") + ")$"}
	}
	plan := map[string]any{"version": 1, "package": "github.com/glade-sh/glade/internal/apextest", "historyUsed": false, "shards": shards}
	planData, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	planData = append(planData, '\n')
	if err := os.WriteFile(filepath.Join(artifactDir, "plan.json"), planData, 0o600); err != nil {
		t.Fatal(err)
	}
	discoveryHash := fmt.Sprintf("%x", sha256.Sum256([]byte(discovery)))
	planHash := fmt.Sprintf("%x", sha256.Sum256(planData))
	runnerData, err := json.Marshal(map[string]any{
		"schema_version": 1, "runner": runner, "package": "github.com/glade-sh/glade/internal/apextest", "head_sha": headSHA,
		"assigned_indexes": indexes, "discovered_count": len(names), "discovery_sha256": discoveryHash, "plan_sha256": planHash,
		"binary_sha256": strings.Repeat("a", 64), "binary_size_bytes": 1, "binary_removed": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(artifactDir, "runner-validation.json"), append(runnerData, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	binaryData := []byte(`{"schema_version":1,"package":"github.com/glade-sh/glade/internal/apextest","sha256":"` + strings.Repeat("a", 64) + `","size_bytes":1,"removed":true}` + "\n")
	if err := os.WriteFile(filepath.Join(artifactDir, "binary.json"), binaryData, 0o600); err != nil {
		t.Fatal(err)
	}
	resource := `{"schema_version":1,"lane":"race-internal-apextest-build","elapsed_seconds":0,"user_seconds":0,"system_seconds":0,"max_rss_kb":1,"exit_status":0}` + "\n"
	if err := os.WriteFile(filepath.Join(artifactDir, "build-resource.json"), []byte(resource), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, index := range indexes {
		shardDir := filepath.Join(artifactDir, fmt.Sprintf("shard-%d", index))
		if err := os.MkdirAll(shardDir, 0o700); err != nil {
			t.Fatal(err)
		}
		selection, err := json.Marshal(shards[index])
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(shardDir, "selection.json"), append(selection, '\n'), 0o600); err != nil {
			t.Fatal(err)
		}
		shardTests := shards[index]["tests"].([]string)
		events := make([]string, 0, len(shardTests))
		for _, name := range shardTests {
			events = append(events, fmt.Sprintf(`{"Action":"pass","Package":"github.com/glade-sh/glade/internal/apextest","Test":"%s"}`, name))
		}
		if err := os.WriteFile(filepath.Join(shardDir, "events.json"), []byte(strings.Join(events, "\n")+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		resource := fmt.Sprintf(`{"schema_version":1,"lane":"race-internal-apextest-shard-%d","elapsed_seconds":0,"user_seconds":0,"system_seconds":0,"max_rss_kb":1,"exit_status":0}`+"\n", index)
		if err := os.WriteFile(filepath.Join(shardDir, "resource.json"), []byte(resource), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCIRaceApextestAggregateRejectsMismatchedEvidence(t *testing.T) {
	root := t.TempDir()
	headSHA := "0123456789abcdef0123456789abcdef01234567"
	writeApextestAggregateArtifact(t, root, "a", []int{0, 2, 4, 5}, headSHA, 332)
	writeApextestAggregateArtifact(t, root, "b", []int{1, 3, 6, 7}, headSHA, 332)
	script, err := filepath.Abs("ci-race-apextest-aggregate.sh")
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, "aggregate.json")
	command := func() (string, error) {
		result, runErr := exec.Command("bash", script, filepath.Join(root, "a"), filepath.Join(root, "b"), out, headSHA).CombinedOutput()
		return string(result), runErr
	}
	if output, runErr := command(); runErr != nil {
		t.Fatalf("valid aggregate evidence rejected: %v\n%s", runErr, output)
	}
	data, err := os.ReadFile(out)
	if err != nil || !strings.Contains(string(data), `"discovered_count":332`) || !strings.Contains(string(data), `"valid":true`) {
		t.Fatalf("aggregate evidence = %s err=%v", data, err)
	}
	if err := os.WriteFile(filepath.Join(root, "b", "runner-validation.json"), []byte(`{"schema_version":1}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if output, runErr := command(); runErr == nil || !strings.Contains(output, "invalid runner validation") {
		t.Fatalf("malformed runner evidence accepted: %v\n%s", runErr, output)
	}
}

func TestCIRaceApextestAggregateDerivesDiscoveryCount(t *testing.T) {
	root := t.TempDir()
	headSHA := "0123456789abcdef0123456789abcdef01234567"
	writeApextestAggregateArtifact(t, root, "a", []int{0, 2, 4, 5}, headSHA, 333)
	writeApextestAggregateArtifact(t, root, "b", []int{1, 3, 6, 7}, headSHA, 333)
	script, err := filepath.Abs("ci-race-apextest-aggregate.sh")
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, "aggregate.json")
	result, err := exec.Command("bash", script, filepath.Join(root, "a"), filepath.Join(root, "b"), out, headSHA).CombinedOutput()
	if err != nil {
		t.Fatalf("aggregate rejected matching 333-test discovery: %v\n%s", err, result)
	}
	data, err := os.ReadFile(out)
	if err != nil || !strings.Contains(string(data), `"discovered_count":333`) || !strings.Contains(string(data), `"valid":true`) {
		t.Fatalf("derived aggregate evidence = %s err=%v", data, err)
	}
}

func TestCIRaceApextestAggregateRejectsAdversarialEvidence(t *testing.T) {
	headSHA := "0123456789abcdef0123456789abcdef01234567"
	tests := []struct {
		name   string
		mutate func(t *testing.T, root string)
	}{
		{name: "mismatched head", mutate: func(t *testing.T, root string) {
			t.Helper()
			path := filepath.Join(root, "b", "runner-validation.json")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(strings.Replace(string(data), headSHA, "fedcba9876543210fedcba9876543210fedcba98", 1)), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "mismatched package", mutate: func(t *testing.T, root string) {
			t.Helper()
			path := filepath.Join(root, "b", "runner-validation.json")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(strings.Replace(string(data), "github.com/glade-sh/glade/internal/apextest", "wrong/package", 1)), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "mismatched plan", mutate: func(t *testing.T, root string) {
			path := filepath.Join(root, "b", "plan.json")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(strings.Replace(string(data), `"historyUsed":false`, `"historyUsed":true`, 1)), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "missing shard index", mutate: func(t *testing.T, root string) {
			t.Helper()
			path := filepath.Join(root, "b", "runner-validation.json")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(strings.Replace(string(data), `"assigned_indexes":[1,3,6,7]`, `"assigned_indexes":[]`, 1)), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "duplicate shard index", mutate: func(t *testing.T, root string) {
			t.Helper()
			path := filepath.Join(root, "b", "runner-validation.json")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(strings.Replace(string(data), `"assigned_indexes":[1,3,6,7]`, `"assigned_indexes":[0,1,3,6,7]`, 1)), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "malformed resource", mutate: func(t *testing.T, root string) {
			if err := os.WriteFile(filepath.Join(root, "b", "shard-7", "resource.json"), []byte("{}\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "skipped terminal", mutate: func(t *testing.T, root string) {
			path := filepath.Join(root, "b", "shard-7", "events.json")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(strings.Replace(string(data), `"Action":"pass"`, `"Action":"skip"`, 1)), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "failed terminal", mutate: func(t *testing.T, root string) {
			path := filepath.Join(root, "b", "shard-7", "events.json")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(strings.Replace(string(data), `"Action":"pass"`, `"Action":"fail"`, 1)), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "duplicate terminal", mutate: func(t *testing.T, root string) {
			path := filepath.Join(root, "b", "shard-7", "events.json")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			first, _, found := strings.Cut(string(data), "\n")
			if !found {
				t.Fatal("missing fixture event")
			}
			if err := os.WriteFile(path, append(data, []byte(first+"\n")...), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "plan union mismatch", mutate: func(t *testing.T, root string) {
			planPath := filepath.Join(root, "b", "plan.json")
			planData, err := os.ReadFile(planPath)
			if err != nil {
				t.Fatal(err)
			}
			var plan map[string]any
			if err := json.Unmarshal(planData, &plan); err != nil {
				t.Fatal(err)
			}
			shards := plan["shards"].([]any)
			shard := shards[4].(map[string]any)
			tests := shard["tests"].([]any)
			shard["tests"] = tests[:len(tests)-1]
			remaining := make([]string, len(tests)-1)
			for index, raw := range tests[:len(tests)-1] {
				remaining[index] = raw.(string)
			}
			shard["regex"] = "^(?:" + strings.Join(remaining, "|") + ")$"
			updatedPlan, err := json.Marshal(plan)
			if err != nil {
				t.Fatal(err)
			}
			updatedPlan = append(updatedPlan, '\n')
			if err := os.WriteFile(planPath, updatedPlan, 0o600); err != nil {
				t.Fatal(err)
			}
			runnerPath := filepath.Join(root, "b", "runner-validation.json")
			runnerData, err := os.ReadFile(runnerPath)
			if err != nil {
				t.Fatal(err)
			}
			var runner map[string]any
			if err := json.Unmarshal(runnerData, &runner); err != nil {
				t.Fatal(err)
			}
			runner["plan_sha256"] = fmt.Sprintf("%x", sha256.Sum256(updatedPlan))
			updatedRunner, err := json.Marshal(runner)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(runnerPath, append(updatedRunner, '\n'), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, fixture := range tests {
		t.Run(fixture.name, func(t *testing.T) {
			root := t.TempDir()
			writeApextestAggregateArtifact(t, root, "a", []int{0, 2, 4, 5}, headSHA, 333)
			writeApextestAggregateArtifact(t, root, "b", []int{1, 3, 6, 7}, headSHA, 333)
			fixture.mutate(t, root)
			script, err := filepath.Abs("ci-race-apextest-aggregate.sh")
			if err != nil {
				t.Fatal(err)
			}
			output, err := exec.Command("bash", script, filepath.Join(root, "a"), filepath.Join(root, "b"), filepath.Join(root, "aggregate.json"), headSHA).CombinedOutput()
			if err == nil {
				t.Fatalf("adversarial evidence accepted:\n%s", output)
			}
		})
	}
}

func TestCIRacePackageRunnerApextestRejectsBuildAndDirectCorruption(t *testing.T) {
	tests := []struct {
		name          string
		env           []string
		planner       string
		resource      string
		wantStatus    int
		wantMaxShards int
	}{
		{name: "build failure", env: []string{"CI_RACE_BUILD_STATUS=27"}, wantStatus: 27, wantMaxShards: 0},
		{name: "build resource corruption", resource: "malformed-build", wantMaxShards: 0},
		{name: "build native precedence", env: []string{"CI_RACE_BUILD_STATUS=28"}, resource: "malformed-build", wantStatus: 28, wantMaxShards: 0},
		{name: "binary absent", env: []string{"CI_RACE_BINARY_MODE=absent"}, wantMaxShards: 0},
		{name: "binary tampered", env: []string{"CI_RACE_BINARY_MODE=tampered"}, wantMaxShards: 1},
		{name: "binary discovery failure", env: []string{"CI_RACE_BINARY_DISCOVERY_MODE=failure", "CI_RACE_BINARY_DISCOVERY_STATUS=32"}, wantStatus: 32, wantMaxShards: 0},
		{name: "binary discovery tampered", env: []string{"CI_RACE_BINARY_DISCOVERY_MODE=tampered"}, wantMaxShards: 0},
		{name: "binary discovery missing", env: []string{"CI_RACE_BINARY_DISCOVERY_MODE=missing"}, wantMaxShards: 0},
		{name: "binary discovery malformed", env: []string{"CI_RACE_BINARY_DISCOVERY_MODE=malformed"}, wantMaxShards: 0},
		{name: "binary discovery fuzz", env: []string{"CI_RACE_BINARY_DISCOVERY_MODE=fuzz"}, wantMaxShards: 0},
		{name: "binary discovery example", env: []string{"CI_RACE_BINARY_DISCOVERY_MODE=example"}, wantMaxShards: 0},
		{name: "binary discovery duplicate", env: []string{"CI_RACE_BINARY_DISCOVERY_MODE=duplicate"}, wantMaxShards: 0},
		{name: "binary discovery duplicate pass", env: []string{"CI_RACE_BINARY_DISCOVERY_MODE=pass-duplicate"}, wantMaxShards: 0},
		{name: "binary discovery go trailer", env: []string{"CI_RACE_BINARY_DISCOVERY_MODE=go-trailer"}, wantMaxShards: 0},
		{name: "test2json failure", env: []string{"CI_RACE_TEST2JSON_STATUS=29"}, wantStatus: 29, wantMaxShards: 1},
		{name: "test2json missing", env: []string{"CI_RACE_TEST2JSON_STATUS=127"}, wantStatus: 127, wantMaxShards: 1},
		{name: "direct failure native precedence", env: []string{"CI_RACE_DIRECT_FAIL_TEST=TestAlpha", "CI_RACE_DIRECT_STATUS=31", "CI_RACE_TEST2JSON_STATUS=29"}, wantStatus: 31, wantMaxShards: 1},
		{name: "direct resource corruption", resource: "malformed-direct", wantMaxShards: 1},
		{name: "malformed direct event", env: []string{"CI_RACE_DIRECT_EVENTS=malformed"}, wantMaxShards: 1},
		{name: "duplicate direct terminal", env: []string{"CI_RACE_DIRECT_EVENTS=duplicate"}, wantMaxShards: 1},
		{name: "missing direct terminal", env: []string{"CI_RACE_DIRECT_EVENTS=missing"}, wantMaxShards: 1},
		{name: "skipped direct terminal", env: []string{"CI_RACE_DIRECT_EVENTS=skip"}, wantMaxShards: 1},
		{name: "failed direct terminal", env: []string{"CI_RACE_DIRECT_EVENTS=fail"}, wantMaxShards: 1},
		{name: "planner omission", planner: "omission", wantMaxShards: 0},
		{name: "planner duplicate", planner: "duplicate", wantMaxShards: 0},
		{name: "planner empty", planner: "empty", wantMaxShards: 0},
		{name: "planner schema", planner: "schema", wantMaxShards: 0},
	}
	for _, fixture := range tests {
		t.Run(fixture.name, func(t *testing.T) {
			root := t.TempDir()
			logPath := filepath.Join(root, "calls.log")
			testNames := []string{"TestAlpha", "TestBeta", "TestDelta", "TestEpsilon", "TestEta", "TestGamma", "TestTheta", "TestZeta"}
			goCommand := writeRaceFixture(t, "fake-go", raceApextestGoFixture())
			plannerText := raceApextestPlannerFixture()
			switch fixture.planner {
			case "omission":
				plannerText = strings.Replace(plannerText, `[[ "${#names[@]}" == 8 ]]`, `names=("${names[@]:0:7}" "TestUnknown")`, 1)
			case "duplicate":
				plannerText = strings.Replace(plannerText, `[[ "${#names[@]}" == 8 ]]`, `names[7]="${names[0]}"`, 1)
			case "empty":
				plannerText = strings.Replace(plannerText, `for index in 0 1 2 3 4 5 6 7; do`, `names[7]=""; for index in 0 1 2 3 4 5 6 7; do`, 1)
			case "schema":
				plannerText = strings.Replace(plannerText, `"historyUsed":false`, `"historyUsed":false,"extra":true`, 1)
			}
			planner := writeRaceFixture(t, "fake-planner", plannerText)
			resourceText := raceResourceFixture()
			if fixture.resource == "malformed-build" {
				resourceText = strings.Replace(resourceText, `printf '{"schema_version":1,"lane":"%s","elapsed_seconds":0,"user_seconds":0,"system_seconds":0,"max_rss_kb":0,"exit_status":%s}\n' "$lane" "$rc" >"$output"`, `if [[ "$lane" == race-internal-apextest-build ]]; then printf '{bad-json}\n' >"$output"; else printf '{"schema_version":1,"lane":"%s","elapsed_seconds":0,"user_seconds":0,"system_seconds":0,"max_rss_kb":0,"exit_status":%s}\n' "$lane" "$rc" >"$output"; fi`, 1)
			}
			if fixture.resource == "malformed-direct" {
				resourceText = strings.Replace(resourceText, `printf '{"schema_version":1,"lane":"%s","elapsed_seconds":0,"user_seconds":0,"system_seconds":0,"max_rss_kb":0,"exit_status":%s}\n' "$lane" "$rc" >"$output"`, `if [[ "$lane" == race-internal-apextest-shard-0 ]]; then printf '{bad-json}\n' >"$output"; else printf '{"schema_version":1,"lane":"%s","elapsed_seconds":0,"user_seconds":0,"system_seconds":0,"max_rss_kb":0,"exit_status":%s}\n' "$lane" "$rc" >"$output"; fi`, 1)
			}
			if slices.Contains(fixture.env, "CI_RACE_BINARY_MODE=tampered") {
				resourceText = strings.Replace(resourceText, `set +e
"$@"`, `if [[ "$lane" == race-internal-apextest-shard-0 ]]; then printf '# tampered\n' >>"$(find "$TMPDIR" -maxdepth 1 -name 'glade-race-apextest.test.*' -print -quit)"; fi
set +e
"$@"`, 1)
			}
			resource := writeRaceFixture(t, "fake-resource", resourceText)
			env := []string{
				"CI_RACE_GO_COMMAND=" + goCommand, "CI_RACE_SHARD_PLANNER=" + planner,
				"CI_RACE_RESOURCE_RUNNER=" + resource, "CI_RACE_CALL_LOG=" + logPath,
				"CI_RACE_TEST_NAMES=" + strings.Join(testNames, "\n") + "\n",
				"TMPDIR=" + root,
			}
			env = append(env, fixture.env...)
			out, err := runRacePackage(t, root, env, "./internal/apextest", "internal-apextest")
			if err == nil {
				t.Fatalf("invalid apex topology accepted:\n%s", out)
			}
			if fixture.wantStatus != 0 && raceExitCode(err) != fixture.wantStatus {
				t.Fatalf("status = %d, want %d\n%s", raceExitCode(err), fixture.wantStatus, out)
			}
			logData, _ := os.ReadFile(logPath)
			if got := strings.Count(string(logData), "start race-internal-apextest-shard-"); got > fixture.wantMaxShards {
				t.Fatalf("started %d shards, want at most %d:\n%s", got, fixture.wantMaxShards, logData)
			}
			unionData, readErr := os.ReadFile(filepath.Join(root, "ci-artifacts/race/internal-apextest/union-validation.json"))
			var union struct {
				Valid bool `json:"valid"`
			}
			if readErr != nil || json.Unmarshal(unionData, &union) != nil || union.Valid {
				t.Fatalf("failed apex topology union = %#v err=%v data=%s", union, readErr, unionData)
			}
			entries, readDirErr := os.ReadDir(root)
			if readDirErr != nil {
				t.Fatal(readDirErr)
			}
			for _, entry := range entries {
				if strings.Contains(entry.Name(), "apextest.test") {
					t.Fatalf("failed apex topology left binary %s", entry.Name())
				}
			}
		})
	}
}

func TestCIRacePackageRunnerApextestSignalStopsActiveLaneAndRemovesBinary(t *testing.T) {
	for _, phase := range []string{"discovery", "direct"} {
		t.Run(phase, func(t *testing.T) {
			root := t.TempDir()
			logPath := filepath.Join(root, "calls.log")
			runnerPIDPath := filepath.Join(root, "runner.pid")
			testNames := []string{"TestAlpha", "TestBeta", "TestDelta", "TestEpsilon", "TestEta", "TestGamma", "TestTheta", "TestZeta"}
			goCommand := writeRaceFixture(t, "fake-go", raceApextestGoFixture())
			planner := writeRaceFixture(t, "fake-planner", raceApextestPlannerFixture())
			resource := writeRaceFixture(t, "fake-resource", `#!/usr/bin/env bash
set -euo pipefail
output="$1"; lane="$2"; shift 3
if [[ "${CI_RACE_SIGNAL_PHASE:-direct}" == direct && "$lane" == race-internal-apextest-shard-0 ]]; then
  sleep 300 & child="$!"
  trap 'kill -TERM "$child" 2>/dev/null || true; wait "$child" 2>/dev/null || true; exit 143' TERM HUP INT
  printf '%s\n' "$$" >"$CI_RACE_RUNNER_PID"
  wait "$child"
fi
set +e
"$@"
rc="$?"
set -e
mkdir -p "$(dirname "$output")"
printf '{"schema_version":1,"lane":"%s","elapsed_seconds":0,"user_seconds":0,"system_seconds":0,"max_rss_kb":0,"exit_status":%s}\n' "$lane" "$rc" >"$output"
exit "$rc"
`)
			script, err := filepath.Abs("ci-race-test.sh")
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("bash", script, "./internal/apextest", "internal-apextest")
			cmd.Dir = root
			cmd.Env = append(os.Environ(),
				"CI_RACE_DEADLINE_ACTIVE=1", "CI_RACE_GO_COMMAND="+goCommand,
				"CI_RACE_SHARD_PLANNER="+planner, "CI_RACE_RESOURCE_RUNNER="+resource,
				"CI_RACE_CALL_LOG="+logPath, "CI_RACE_RUNNER_PID="+runnerPIDPath,
				"CI_RACE_TEST_NAMES="+strings.Join(testNames, "\n")+"\n", "TMPDIR="+root,
				"CI_RACE_SIGNAL_PHASE="+phase,
			)
			if phase == "discovery" {
				cmd.Env = append(cmd.Env, "CI_RACE_BINARY_DISCOVERY_MODE=block")
			}
			var output bytes.Buffer
			cmd.Stdout = &output
			cmd.Stderr = &output
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			waited := make(chan error, 1)
			go func() { waited <- cmd.Wait() }()
			testDeadline, hasTestDeadline := t.Deadline()
			deadline := ciRaceStartupDeadline(time.Now(), testDeadline, hasTestDeadline)
			for {
				if _, err := os.Stat(runnerPIDPath); err == nil {
					break
				}
				select {
				case err := <-waited:
					t.Fatalf("direct lane exited before startup: %v:\n%s", err, output.String())
				default:
				}
				if time.Now().After(deadline) {
					stopCIRaceCommand(cmd, waited)
					t.Fatalf("direct lane did not start:\n%s", output.String())
				}
				time.Sleep(10 * time.Millisecond)
			}
			if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-waited:
				if raceExitCode(err) != 143 {
					t.Fatalf("signal exit = %d, want 143:\n%s", raceExitCode(err), output.String())
				}
			case <-time.After(2 * time.Second):
				pidData, _ := os.ReadFile(runnerPIDPath)
				if runnerPID := strings.TrimSpace(string(pidData)); runnerPID != "" {
					_ = exec.Command("kill", "-TERM", runnerPID).Run()
				}
				stopCIRaceCommand(cmd, waited)
				t.Fatal("signal did not promptly stop the active Apex resource lane")
			}
			entries, err := os.ReadDir(root)
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if strings.Contains(entry.Name(), "apextest.test") {
					t.Fatalf("signal left temporary race binary %s", entry.Name())
				}
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
