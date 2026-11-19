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
	"strconv"
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
	if len(got) != len(want) || !reflect.DeepEqual(got, want) {
		t.Fatalf("full packages = %d/%v, want %d exact manifest packages", len(got), got, len(want))
	}
}

func TestCIRaceClassifierPartitionsFullManifest(t *testing.T) {
	full, out, err := runRaceClassifier(t, nil, "full")
	if err != nil {
		t.Fatalf("full classification failed: %v\n%s", err, out)
	}
	fullJSON, err := json.Marshal(full)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", "ci-race-packages.sh", "partition", string(fullJSON))
	data, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("full partition failed: %v\n%s", err, data)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Fatalf("partition output has %d lines, want early, independent and generic JSON", len(lines))
	}
	var early, independent, generic []string
	if err := json.Unmarshal([]byte(lines[0]), &early); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(lines[1]), &independent); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(lines[2]), &generic); err != nil {
		t.Fatal(err)
	}
	if want := []string{"./internal/gladecli"}; !reflect.DeepEqual(early, want) {
		t.Fatalf("early packages = %v, want %v", early, want)
	}
	if want := []string{"./internal/repoguard"}; !reflect.DeepEqual(independent, want) {
		t.Fatalf("independent packages = %v, want %v", independent, want)
	}
	// The long binary-free package runs on its own early lane, and the VM
	// route is bounded by dedicated shards instead of the generic matrix.
	for _, excluded := range []string{"./internal/repoguard", "./internal/vm"} {
		if slices.Contains(generic, excluded) {
			t.Fatalf("generic packages still contain %s", excluded)
		}
	}
	dedicated := []string{"./internal/apextest", "./internal/visualforce", "./internal/lwc/compile", "./internal/vm"}
	combined := append(append(append(append([]string(nil), early...), independent...), generic...), dedicated...)
	sort.Strings(combined)
	if !reflect.DeepEqual(combined, full) || len(combined) != len(slices.Compact(combined)) {
		t.Fatalf("early, independent, generic and dedicated packages do not exactly partition the full manifest")
	}
}

func fullManifestPackageCount(t *testing.T) int {
	t.Helper()
	data, err := os.ReadFile("ci-package-lanes.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Lanes map[string][]string `json:"lanes"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, packages := range manifest.Lanes {
		count += len(packages)
	}
	return count
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
	if len(got) != fullManifestPackageCount(t) || !strings.Contains(diagnostic, "git diff failed") {
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
	if len(got) != fullManifestPackageCount(t) || !strings.Contains(diagnostic, "deleted Go file") {
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
	if len(got) != fullManifestPackageCount(t) {
		t.Fatalf("nested module change selected %d packages, want full manifest", len(got))
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
	if len(got) != fullManifestPackageCount(t) || !strings.Contains(diagnostic, "dependency graph failed") {
		t.Fatalf("graph failure selected %d packages without diagnostic: %s", len(got), diagnostic)
	}
}

func TestCIRaceWorkflowContract(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", ".github", "workflows", "race.yml"))
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(data)
	if !strings.Contains(workflow, "permissions:\n  contents: read\n\nenv:\n  GOMEMLIMIT: \"6GiB\"\n\nconcurrency:") {
		t.Fatal("race workflow must set the shared CI Go memory limit for all jobs")
	}
	for _, marker := range []string{
		"name: Race", "schedule:", "cron: '43 10 * * *'", "workflow_dispatch:",
		"fetch-depth: 0", "scripts/ci-race-packages.sh", "fromJSON(needs.plan.outputs.packages)",
		"fail-fast: false", "max-parallel: 6", "GOMAXPROCS: \"2\"", "go-version: \"1.26.11\"",
		`scripts/ci-race-test.sh "$PACKAGE" "$SLUG"`, "if: always()",
		"actions/upload-artifact@b7c566a772e6b6bfb58ed0dc250532a479d7789f # v6.0.0", "ci-artifacts/race/", "if-no-files-found: error",
		"npm ci --prefix third_party/lwc", "matrix.package == './internal/gladecli'",
		"early-packages", "independent-packages", "generic-packages", "has-early-packages", "has-independent-packages", "has-generic-packages", "race-early:", "race-independent:",
		"race-apextest:", "race-apextest-aggregate:", "race-vm:", "race-vm-aggregate:",
		"python3 scripts/ci-race-plan.py lanes",
		"include: ${{ fromJSON(needs.test-binaries.outputs.runner-matrix).include }}",
		"CI_RACE_APEXTEST_RUNNER: ${{ matrix.runner }}",
		"CI_RACE_PARTITION_DIR: /tmp/glade-race-apextest-partition",
		"CI_RACE_BATCH_TIMEOUT_SECONDS: ${{ matrix.batchTimeoutSeconds }}",
		"timeout-minutes: ${{ matrix.timeoutMinutes }}", "timeout-minutes: ${{ matrix.stepMinutes }}",
		"scripts/ci-race-test.sh partition ./internal/apextest internal-apextest",
		"name: race-apextest-partition-${{ github.run_id }}-${{ steps.bundle.outputs.attempt }}",
		"CI_RACE_HEAD_SHA: ${{ github.sha }}",
		"include: ${{ fromJSON(needs.plan.outputs.vm-matrix).include }}",
		"CI_RACE_TEST_TIMEOUT_SECONDS: ${{ matrix.testTimeoutSeconds }}",
		"scripts/ci-race-shard-aggregate.sh ./internal/vm",
		"if: always() && needs.plan.outputs.mode == 'full' && contains(fromJSON(needs.plan.outputs.packages), './internal/apextest')", "scripts/ci-race-apextest-aggregate.sh /tmp/glade-race-apextest-partition ci-artifacts/race/apextest-runners", "actions/download-artifact@d3f86a106a0bac45b974a628896c90dbdf5c8093 # v4.3.0",
	} {
		if !strings.Contains(workflow, marker) {
			t.Errorf("race workflow missing %q", marker)
		}
	}
	for _, forbidden := range []string{"workflow_run:", "pull_request:", "pull_request_target:", "runs-on: ubuntu-latest-", "go test -race ./..."} {
		if strings.Contains(workflow, forbidden) {
			t.Errorf("race workflow contains forbidden %q", forbidden)
		}
	}
	// The duplicated hard-coded Apex A/B/C assignments are gone: the
	// workflow, runner and aggregate all consume the one race partition.
	for _, forbidden := range []string{"CI_RACE_APEXTEST_SHARD_INDEXES", "race-apextest-a:", "race-apextest-b:", "race-apextest-c:"} {
		if strings.Contains(workflow, forbidden) {
			t.Errorf("race workflow contains obsolete %q", forbidden)
		}
	}
	if !strings.Contains(workflow, "schedule|workflow_dispatch)\n              mode=full\n              packages=\"$(scripts/ci-race-packages.sh full)\"") {
		t.Fatal("race workflow must retain the exhaustive manifest for scheduled and manual runs")
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
	planIndex, earlyIndex := strings.Index(workflow, "  plan:"), strings.Index(workflow, "  race-early:")
	raceIndex, apexIndex := strings.Index(workflow, "  race:"), strings.Index(workflow, "  race-apextest:")
	if planIndex < 0 || earlyIndex < 0 || raceIndex < 0 || apexIndex < 0 {
		t.Fatal("race workflow is missing a required job boundary")
	}
	jobs := workflowJobBlocksText(workflow)
	planJob := jobs["plan"]
	earlyJob := jobs["race-early"]
	raceJob := jobs["race"]
	apexJob := jobs["race-apextest"]
	vmJob := jobs["race-vm"]
	independentJob := jobs["race-independent"]
	if !strings.Contains(earlyJob, "- name: Run early race detector\n        env:\n          GOMAXPROCS: ${{ matrix.package == './internal/gladecli' && '1' || '2' }}\n          PACKAGE: ${{ matrix.package }}") {
		t.Fatal("race workflow must run the Glade CLI race shards with one Go scheduler thread")
	}
	if !strings.Contains(planJob, `scripts/ci-race-packages.sh partition "$packages"`) {
		t.Error("race plan must derive early, independent and generic packages from its one full manifest")
	}
	if !strings.Contains(planJob, `if [[ "${#partition[@]}" -ne 3 ]]; then`) {
		t.Error("race plan must fail closed when package partitioning does not return all three lists")
	}
	if !strings.Contains(planJob, "python3 scripts/ci-race-plan.py lanes") || !strings.Contains(planJob, "vm-matrix=") {
		t.Error("race plan must publish the shared lane policy for the VM matrix")
	}
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
		"if: ${{ steps.classify.outputs.mode == 'full' && contains(fromJSON(steps.classify.outputs.early-packages), './internal/gladecli') }}",
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
	if !strings.Contains(earlyJob, "matrix:\n        package: ${{ fromJSON(needs.plan.outputs.early-packages) }}") ||
		strings.Contains(earlyJob, "./internal/apextest") {
		t.Fatal("race early matrix must contain only the classified early packages")
	}
	if !strings.Contains(raceJob, "matrix:\n        package: ${{ fromJSON(needs.plan.outputs.generic-packages) }}") {
		t.Fatal("race generic matrix must use only the classified generic packages")
	}
	if got := strings.Count(raceJob, "actions/setup-node@249970729cb0ef3589644e2896645e5dc5ba9c38 # v6.5.0"); got != 1 {
		t.Fatalf("race Node setup count = %d, want one setup shared by all Node consumers", got)
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
	const nodeConsumers = `contains(fromJSON('["./internal/gladehome","./internal/lwc","./internal/lwcbrowser","./internal/lwcruntime","./internal/server"]'), matrix.package)`
	const lwcConsumers = `contains(fromJSON('["./internal/gladehome","./internal/lwc","./internal/lwcbrowser","./internal/lwcruntime","./internal/server"]'), matrix.package)`
	const browserConsumers = `contains(fromJSON('["./internal/lwcbrowser","./internal/lwcruntime","./internal/server"]'), matrix.package)`
	for selector, want := range map[string]int{lwcConsumers: 4, browserConsumers: 2} {
		if got := strings.Count(raceJob, selector); got != want {
			t.Errorf("race consumer selector %q count = %d, want %d", selector, got, want)
		}
	}
	previous := -1
	for _, requirement := range []struct{ marker, selector, command string }{
		{"      - uses: actions/setup-node@249970729cb0ef3589644e2896645e5dc5ba9c38 # v6.5.0", nodeConsumers, `node-version: "22"`},
		{"      - name: Resolve npm cache path", lwcConsumers, `run: echo "dir=$(npm config get cache)" >>"$GITHUB_OUTPUT"`},
		{"      - name: Restore LWC npm cache", lwcConsumers, "actions/cache/restore@0057852bfaa89a56745cba8c7296529d2fc39830 # v4.3.0"},
		{"      - name: Install LWC toolchain", lwcConsumers, "run: npm ci --prefix third_party/lwc"},
		{"      - name: Install browser conformance dependencies", browserConsumers, "run: npm ci --prefix lwcruntime"},
		{"      - name: Install conformance Chromium", browserConsumers, "run: scripts/ci-install-playwright.sh ./lwcruntime/node_modules/.bin/playwright install --with-deps chromium"},
	} {
		step := workflowStepBlockText(raceJob, requirement.marker)
		if !strings.Contains(step, "if: ${{ "+requirement.selector+" }}") || !strings.Contains(step, requirement.command) {
			t.Errorf("race prerequisite %q must use its exact consumer selector and command", requirement.marker)
		}
		index := strings.Index(raceJob, requirement.marker)
		if strings.Count(raceJob, requirement.marker) != 1 || index <= previous {
			t.Errorf("race prerequisite %q must occur exactly once after its prerequisites", requirement.marker)
		}
		previous = index
	}
	if runIndex := strings.Index(raceJob, "      - name: Run race detector"); runIndex <= previous {
		t.Error("race detector must run after browser prerequisites")
	}
	if strings.Contains(raceJob, "actions/cache/save") || strings.Contains(raceJob, "cache: npm") {
		t.Fatal("dynamic race matrix must be restore-only for npm cache")
	}
	// The long binary-free lanes start after the plan alone instead of
	// queueing behind the generic matrix or the binary producer.
	for _, job := range []struct{ name, body string }{{"race-vm", vmJob}, {"race-independent", independentJob}} {
		if !strings.Contains(job.body, "needs: [plan]") || strings.Contains(job.body, "test-binaries") {
			t.Errorf("%s must depend only on the plan job, not on binary producers it does not consume", job.name)
		}
	}
	if strings.Contains(raceJob, "./internal/vm") || strings.Contains(raceJob, "./internal/repoguard") {
		t.Fatal("the VM and repoguard routes must keep their own early lanes out of the generic matrix")
	}
	// Every planned job deadline is coherent with the partition: the job
	// budget comes from the plan matrix, the step budget covers the batch
	// deadline, and the runner verifies the batch against the partition.
	for _, marker := range []string{
		"needs: [plan, test-binaries]",
		"matrix:\n        include: ${{ fromJSON(needs.test-binaries.outputs.runner-matrix).include }}",
		"name: race-test-binaries-apextest-${{ github.run_id }}-${{ needs.test-binaries.outputs.artifact-attempt }}",
		"name: race-apextest-partition-${{ github.run_id }}-${{ needs.test-binaries.outputs.artifact-attempt }}",
		"- name: Run assigned Apex race shards\n        timeout-minutes: ${{ matrix.stepMinutes }}",
		"CI_RACE_APEXTEST_RUNNER: ${{ matrix.runner }}",
		"CI_RACE_BATCH_TIMEOUT_SECONDS: ${{ matrix.batchTimeoutSeconds }}",
	} {
		if !strings.Contains(apexJob, marker) {
			t.Errorf("race Apex runner lane missing %q", marker)
		}
	}
	for _, marker := range []string{
		"matrix:\n        include: ${{ fromJSON(needs.plan.outputs.vm-matrix).include }}",
		"CI_RACE_SHARD_INDEX: ${{ matrix.shard }}",
		"CI_RACE_TEST_TIMEOUT_SECONDS: ${{ matrix.testTimeoutSeconds }}",
		"CI_RACE_BATCH_TIMEOUT_SECONDS: ${{ matrix.batchTimeoutSeconds }}",
		"- name: Run assigned VM race shard\n        timeout-minutes: ${{ matrix.stepMinutes }}",
	} {
		if !strings.Contains(vmJob, marker) {
			t.Errorf("race VM lane missing %q", marker)
		}
	}
	if !strings.Contains(jobs["race-apextest-aggregate"], "needs: [plan, test-binaries, race-apextest]") ||
		!strings.Contains(jobs["race-apextest-aggregate"], "pattern: race-internal-apextest-*-${{ github.run_id }}-${{ github.run_attempt }}") {
		t.Fatal("the Apex aggregate must validate every planned runner against the shared partition")
	}
}

func TestCIRaceWorkflowSharesValidatedLWCHistory(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", ".github", "workflows", "race.yml"))
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(data)
	planEnd := strings.Index(workflow, "  race-browser:")
	browserEnd := strings.Index(workflow, "  race-browser-aggregate:")
	if planEnd < 0 || browserEnd <= planEnd {
		t.Fatal("missing Race history producer or browser consumer")
	}
	plan, browser := workflow[:planEnd], workflow[planEnd:browserEnd]
	for _, marker := range []string{
		"actions: read", `refs=("$GITHUB_SHA" "$BEFORE_SHA")`,
		`git merge-base HEAD "refs/remotes/origin/$DEFAULT_BRANCH"`,
		`-f head_sha="$ref" -f status=success`,
		"pattern: go-test-lwc-compile-shard-*",
		`/tmp/race-lwc-predecessor/go-test-lwc-compile-shard-{0..11}`,
		`scripts/ci-go-test.sh lwc-compile-history-refresh "$CI_LWC_COMPILE_HISTORY_PATH" "${shards[@]}"`,
		`rm -f "$CI_LWC_COMPILE_HISTORY_PATH"`,
		"using deterministic scheduling fallback",
		"name: race-lwc-history-input-${{ github.run_id }}-${{ github.run_attempt }}",
	} {
		if !strings.Contains(plan, marker) {
			t.Errorf("Race history input missing %q", marker)
		}
	}
	for _, marker := range []string{
		"CI_LWC_COMPILE_HISTORY_PATH: /tmp/glade-lwc-compile-duration-history/lwc-compile-duration-history.json",
		"name: Download shared LWC duration history\n        if: matrix.package == './internal/lwc/compile'",
		"name: race-lwc-history-input-${{ github.run_id }}-${{ github.run_attempt }}",
		"path: /tmp/glade-lwc-compile-duration-history",
	} {
		if !strings.Contains(browser, marker) {
			t.Errorf("Race browser history consumer missing %q", marker)
		}
	}
	if strings.Contains(browser, "gh api") || strings.Contains(browser, "cache/restore") {
		t.Fatal("Race workers must consume one immutable plan input")
	}
	downloadStart := strings.Index(plan, "      - name: Download LWC timing evidence")
	downloadEnd := strings.Index(plan, "      - name: Validate LWC duration history")
	if downloadStart < 0 || downloadEnd <= downloadStart ||
		!strings.Contains(plan[downloadStart:downloadEnd], "continue-on-error: true") ||
		strings.Count(workflow, "continue-on-error:") != 1 {
		t.Fatal("only optional timing artifact download may continue after an error")
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
	if pkg == "./internal/apextest" {
		if err := os.MkdirAll(filepath.Join(workdir, "internal", "apextest"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if pkg == "./internal/apextest" || pkg == "./internal/gladecli" || pkg == "./internal/playground" || pkg == "./internal/server" || pkg == "./internal/testdaemon" || pkg == "./internal/vm" || pkg == "./internal/visualforce" || pkg == "./internal/lwc/compile" {
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
    --shards) [[ "$2" == 16 ]]; shift 2 ;;
    --tests) tests="$2"; shift 2 ;;
    *) exit 91 ;;
  esac
done
names=()
while IFS= read -r name; do names+=("$name"); done <"$tests"
[[ "${#names[@]}" == 16 ]]
printf '{"version":1,"package":"%s","historyUsed":false,"shards":[' "$package"
for index in 0 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15; do
  [[ "$index" == 0 ]] || printf ','
  printf '{"index":%s,"tests":["%s"],"estimatedDurationMillis":0,"regex":"^(?:%s)$"}' "$index" "${names[$index]}" "${names[$index]}"
done
printf ']}\n'
`
}

// raceBatchTimeoutFixture accepts any planned batch deadline and logs it, so
// a test can assert the wrapper deadline the partition recorded.
func raceBatchTimeoutFixture(t *testing.T) string {
	t.Helper()
	return writeRaceFixture(t, "fake-timeout", `#!/usr/bin/env bash
set -euo pipefail
[[ "$1" == "--signal=TERM" && "$2" == "--kill-after=30s" ]] || exit 92
if [[ -n "${CI_RACE_CALL_LOG:-}" ]]; then printf 'deadline %s\n' "$3" >>"$CI_RACE_CALL_LOG"; fi
shift 3
exec "$@"
`)
}

// writeRaceApextestPartitionForFixture builds one partition artifact from the
// fixture binary: the same fake go command builds a probe binary so the
// partition records its real checksum, and the one race plan authority
// schedules the sixteen-shard partition and returns the runner the test
// should execute along with that runner's planned batch deadline.
func writeRaceApextestPartitionForFixture(t *testing.T, root, goCommand string, testNames []string) (partitionDir, runner, batch string) {
	t.Helper()
	probeBinary := filepath.Join(t.TempDir(), "probe.test")
	probe := exec.Command(goCommand, "test", "-trimpath", "-race", "-c", "-o", probeBinary, "./internal/apextest")
	probe.Env = append(os.Environ(), "CI_RACE_TEST_NAMES="+strings.Join(testNames, "\n")+"\n",
		"CI_RACE_CALL_LOG="+filepath.Join(t.TempDir(), "probe.log"))
	if out, err := probe.CombinedOutput(); err != nil {
		t.Fatalf("probe binary build failed: %v\n%s", err, out)
	}
	binaryData, err := os.ReadFile(probeBinary)
	if err != nil {
		t.Fatal(err)
	}
	binarySHA := fmt.Sprintf("%x", sha256.Sum256(binaryData))
	sorted := append([]string(nil), testNames...)
	sort.Strings(sorted)
	partitionDir = filepath.Join(root, "partition")
	if err := os.MkdirAll(partitionDir, 0o700); err != nil {
		t.Fatal(err)
	}
	discovery := filepath.Join(partitionDir, "discovery.txt")
	if err := os.WriteFile(discovery, []byte(strings.Join(sorted, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	shards := make([]map[string]any, len(sorted))
	for index, name := range sorted {
		shards[index] = map[string]any{"index": index, "tests": []string{name}, "estimatedDurationMillis": 0, "regex": "^(?:" + name + ")$"}
	}
	planData, err := json.Marshal(map[string]any{"version": 1, "package": "github.com/glade-sh/glade/internal/apextest", "historyUsed": false, "shards": shards})
	if err != nil {
		t.Fatal(err)
	}
	plan := filepath.Join(partitionDir, "plan.json")
	if err := os.WriteFile(plan, append(planData, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	partitionOutput, err := exec.Command("python3", "ci-race-plan.py", "schedule", discovery, plan, binarySHA, strconv.Itoa(len(binaryData))).CombinedOutput()
	if err != nil {
		t.Fatalf("fixture partition schedule failed: %v\n%s", err, partitionOutput)
	}
	if err := os.WriteFile(filepath.Join(partitionDir, "partition.json"), partitionOutput, 0o600); err != nil {
		t.Fatal(err)
	}
	var partition struct {
		Runners []struct {
			Runner              string `json:"runner"`
			BatchTimeoutSeconds int    `json:"batch_timeout_seconds"`
		} `json:"runners"`
	}
	if err := json.Unmarshal(partitionOutput, &partition); err != nil {
		t.Fatal(err)
	}
	// The second runner never owns shard zero in the balanced round-robin,
	// so the caller can also assert unassigned shards stay untouched.
	if len(partition.Runners) < 2 {
		t.Fatalf("fixture partition has %d runners", len(partition.Runners))
	}
	return partitionDir, partition.Runners[1].Runner, strconv.Itoa(partition.Runners[1].BatchTimeoutSeconds)
}

func raceApextestTestNames() []string {
	return []string{
		"TestAlpha", "TestBeta", "TestDelta", "TestEpsilon", "TestEta", "TestGamma", "TestTheta", "TestZeta",
		"TestIota", "TestKappa", "TestLambda", "TestMu", "TestNu", "TestOmicron", "TestPi", "TestRho",
	}
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
if [[ -n "${CI_RACE_EXPECT_PACKAGE_DIR:-}" ]]; then
  [[ "$(pwd -P)" == "$(cd "$CI_RACE_EXPECT_PACKAGE_DIR" && pwd -P)" ]] || exit 93
  [[ "$(cat testdata/cwd.txt)" == package-relative-fixture ]] || exit 94
fi
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
if [[ "${CI_RACE_DEFERRED_TEST:-}" == "$test" ]]; then
  name="${test}${CI_RACE_DEFERRED_SUFFIX:-}"
  printf '{"Action":"output","Package":"github.com/glade-sh/glade/internal/apextest","Test":"%s","Output":"    dataweave_typed_proof_test.go:15: %s\\n"}\n' "$name" "${CI_RACE_DEFERRED_REASON:-requires explicitly installed DataWeave toolchain}"
  printf '{"Action":"skip","Package":"github.com/glade-sh/glade/internal/apextest","Test":"%s"}\n' "$name"
else
case "${CI_RACE_DIRECT_EVENTS:-valid}" in
  malformed) printf '{bad-json}\n' ;;
  duplicate) printf '{"Action":"pass","Package":"github.com/glade-sh/glade/internal/apextest","Test":"%s"}\n' "$test"; printf '{"Action":"pass","Package":"github.com/glade-sh/glade/internal/apextest","Test":"%s"}\n' "$test" ;;
  missing) ;;
  skip|fail) printf '{"Action":"%s","Package":"github.com/glade-sh/glade/internal/apextest","Test":"%s"}\n' "$CI_RACE_DIRECT_EVENTS" "$test" ;;
  *) printf '{"Action":"run","Package":"github.com/glade-sh/glade/internal/apextest","Test":"%s"}\n' "$test"; printf '{"Action":"pass","Package":"github.com/glade-sh/glade/internal/apextest","Test":"%s"}\n' "$test" ;;
esac
fi
printf '{"Action":"pass","Package":"github.com/glade-sh/glade/internal/apextest"}\n'
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
			t.Parallel()
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
	// Discovery only needs names; playground has no race-specific build tags.
	// The real planner below still checks all five shards and their exact union.
	// -vet=off matches the CI discovery step (ci-go-test.sh) and skips a vet
	// pass over the whole playground dependency graph on a cold build cache.
	discoveryCommand := exec.Command("go", "test", "-vet=off", "-list", ".", "../internal/playground")
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
	for _, pkg := range []string{"./internal/gladecli", "./internal/server", "./internal/testdaemon"} {
		t.Run(filepath.Base(pkg), func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			logPath := filepath.Join(root, "calls.log")
			shardCount := 4
			if pkg == "./internal/gladecli" {
				shardCount = 16
			} else if pkg == "./internal/server" {
				shardCount = 8
			}
			testNames := make([]string, shardCount)
			for index := range testNames {
				testNames[index] = fmt.Sprintf("TestCase%02d", index)
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
printf '{"Action":"pass","Package":"github.com/glade-sh/glade/%s"}\n' "${CI_RACE_PACKAGE#./}"
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
			// The default go test deadline sits below the flat 60m batch so a
			// hung shard ends in its own timeout panic, not the outer kill.
			if strings.Count(log, "native "+testPrefix+" -json -race -count=1 -timeout=55m -run") != shardCount {
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
				base := filepath.Join(root, "ci-artifacts/race", slug, "shard-"+fmt.Sprint(shard))
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

func TestCIRacePackageRunnerSentinelMatchesTopology(t *testing.T) {
	for _, fixture := range []struct {
		pkg    string
		shards int
	}{
		{pkg: "./internal/apextest", shards: 16},
		{pkg: "./internal/gladecli", shards: 16},
		{pkg: "./internal/server", shards: 8},
	} {
		t.Run(filepath.Base(fixture.pkg), func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			goCommand := writeRaceFixture(t, "fake-go", "#!/usr/bin/env bash\nexit 23\n")
			resource := writeRaceFixture(t, "fake-resource", raceResourceFixture())
			slug := strings.ReplaceAll(strings.TrimPrefix(fixture.pkg, "./"), "/", "-")
			if out, err := runRacePackage(t, root, []string{
				"CI_RACE_GO_COMMAND=" + goCommand,
				"CI_RACE_RESOURCE_RUNNER=" + resource,
			}, fixture.pkg, slug); err == nil {
				t.Fatalf("discovery failure accepted:\n%s", out)
			}
			data, err := os.ReadFile(filepath.Join(root, "ci-artifacts/race", slug, "union-validation.json"))
			if err != nil {
				t.Fatal(err)
			}
			var sentinel struct {
				ShardCounts []int `json:"shard_counts"`
				Valid       bool  `json:"valid"`
			}
			if err := json.Unmarshal(data, &sentinel); err != nil || sentinel.Valid || !reflect.DeepEqual(sentinel.ShardCounts, make([]int, fixture.shards)) {
				t.Fatalf("%d-shard sentinel = %#v, err=%v", fixture.shards, sentinel, err)
			}
		})
	}
}

func TestCIRacePackageRunnerApextestBuildsOnceAndUsesSixteenDirectShards(t *testing.T) {
	root := t.TempDir()
	packageDir := filepath.Join(root, "internal", "apextest")
	if err := os.MkdirAll(filepath.Join(packageDir, "testdata"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packageDir, "testdata", "cwd.txt"), []byte("package-relative-fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(root, "calls.log")
	testNames := raceApextestTestNames()
	goCommand := writeRaceFixture(t, "fake-go", raceApextestGoFixture())
	planner := writeRaceFixture(t, "fake-planner", raceApextestPlannerFixture())
	resource := writeRaceFixture(t, "fake-resource", raceResourceFixture())
	out, err := runRacePackage(t, root, []string{
		"CI_RACE_GO_COMMAND=" + goCommand, "CI_RACE_SHARD_PLANNER=" + planner,
		"CI_RACE_RESOURCE_RUNNER=" + resource, "CI_RACE_CALL_LOG=" + logPath,
		"CI_RACE_TEST_NAMES=" + strings.Join(testNames, "\n") + "\n",
		"CI_RACE_BINARY_DISCOVERY_PASS=1",
		"CI_RACE_EXPECT_PACKAGE_DIR=" + packageDir,
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
	if strings.Count(log, "go test ") != 1 || strings.Count(log, "go test -trimpath -race -c -o") != 1 || strings.Count(log, "go tool test2json -t -p github.com/glade-sh/glade/internal/apextest") != 16 {
		t.Fatalf("apextest build/test2json calls:\n%s", log)
	}
	if strings.Count(log, "binary -test.list .") != 1 || strings.Count(log, "binary -test.v=test2json -test.count=1 -test.timeout=60m -test.run=") != 16 || strings.Contains(log, "go test -json -race") || strings.Contains(log, "go test -race -list") {
		t.Fatalf("apextest direct binary calls:\n%s", log)
	}
	for shard := 0; shard < 16; shard++ {
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
	if err != nil || json.Unmarshal(unionData, &union) != nil || union.DiscoveredCount != 16 || !reflect.DeepEqual(union.ShardCounts, []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}) || !union.Valid {
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
	testNames := raceApextestTestNames()
	goCommand := writeRaceFixture(t, "fake-go", raceApextestGoFixture())
	planner := writeRaceFixture(t, "fake-planner", raceApextestPlannerFixture())
	resource := writeRaceFixture(t, "fake-resource", raceResourceFixture())
	gitCommand := writeRaceFixture(t, "fake-git", "#!/usr/bin/env bash\nprintf '%s\\n' 0123456789abcdef0123456789abcdef01234567\n")
	partitionDir, runner, batch := writeRaceApextestPartitionForFixture(t, root, goCommand, testNames)
	out, err := runRacePackage(t, root, []string{
		"CI_RACE_GO_COMMAND=" + goCommand, "CI_RACE_SHARD_PLANNER=" + planner,
		"CI_RACE_RESOURCE_RUNNER=" + resource, "CI_RACE_GIT_COMMAND=" + gitCommand, "CI_RACE_CALL_LOG=" + logPath,
		"CI_RACE_TEST_NAMES=" + strings.Join(testNames, "\n") + "\n",
		"CI_RACE_TIMEOUT_COMMAND=" + raceBatchTimeoutFixture(t),
		"CI_RACE_BINARY_DISCOVERY_PASS=1", "CI_RACE_PARTITION_DIR=" + partitionDir,
		"CI_RACE_APEXTEST_RUNNER=" + runner, "CI_RACE_HEAD_SHA=0123456789abcdef0123456789abcdef01234567",
		"CI_RACE_BATCH_TIMEOUT_SECONDS=" + batch, "TMPDIR=" + root,
	}, "./internal/apextest", "internal-apextest")
	if err != nil {
		t.Fatalf("assigned apex shard failed: %v\n%s", err, out)
	}
	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	log := string(logData)
	var partition struct {
		Runners []struct {
			Runner       string `json:"runner"`
			ShardIndexes []int  `json:"shard_indexes"`
		} `json:"runners"`
		Shards []struct {
			Index              int `json:"index"`
			TestTimeoutSeconds int `json:"test_timeout_seconds"`
		} `json:"shards"`
	}
	partitionData, err := os.ReadFile(filepath.Join(partitionDir, "partition.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(partitionData, &partition); err != nil {
		t.Fatal(err)
	}
	assignment := map[int]bool{}
	for _, entry := range partition.Runners {
		if entry.Runner != runner {
			continue
		}
		for _, index := range entry.ShardIndexes {
			assignment[index] = true
		}
	}
	if len(assignment) == 0 {
		t.Fatalf("partition has no assignment for runner %q", runner)
	}
	if got := strings.Count(log, "start race-internal-apextest-shard-"); got != len(assignment) {
		t.Fatalf("assigned runner started %d shard lanes, want %d:\n%s", got, len(assignment), log)
	}
	for index := range assignment {
		if !strings.Contains(log, fmt.Sprintf("start race-internal-apextest-shard-%d", index)) {
			t.Fatalf("assigned runner omitted shard %d:\n%s", index, log)
		}
		if !strings.Contains(log, fmt.Sprintf("binary -test.v=test2json -test.count=1 -test.timeout=%ds -test.run=", partition.Shards[index].TestTimeoutSeconds)) {
			t.Fatalf("shard %d did not use its planned deadline:\n%s", index, log)
		}
	}
	if !strings.Contains(log, "deadline "+batch) {
		t.Fatalf("the batch wrapper did not use the planned batch deadline:\n%s", log)
	}
	artifactDir := filepath.Join(root, "ci-artifacts", "race", "internal-apextest")
	for index := range assignment {
		if _, err := os.Stat(filepath.Join(artifactDir, fmt.Sprintf("shard-%d", index), "events.json")); err != nil {
			t.Fatalf("assigned shard %d evidence: %v", index, err)
		}
	}
	if _, err := os.Stat(filepath.Join(artifactDir, "shard-0", "events.json")); !os.IsNotExist(err) {
		if assignment[0] {
			t.Fatalf("shard 0 was assigned; fix the fixture expectations")
		}
		t.Fatalf("unassigned shard evidence exists or failed unexpectedly: %v", err)
	}
	runnerData, err := os.ReadFile(filepath.Join(artifactDir, "runner-validation.json"))
	if err != nil {
		t.Fatalf("runner validation evidence: %v", err)
	}
	var runnerEvidence struct {
		Runner          string `json:"runner"`
		HeadSHA         string `json:"head_sha"`
		AssignedIndexes []int  `json:"assigned_indexes"`
		DiscoveredCount int    `json:"discovered_count"`
		PartitionSHA256 string `json:"partition_sha256"`
		BinaryRemoved   bool   `json:"binary_removed"`
		Outcome         string `json:"outcome"`
		Failure         any    `json:"failure"`
	}
	if err := json.Unmarshal(runnerData, &runnerEvidence); err != nil || runnerEvidence.Runner != runner ||
		runnerEvidence.HeadSHA != "0123456789abcdef0123456789abcdef01234567" || !reflect.DeepEqual(runnerEvidence.AssignedIndexes, raceAssignedShardIndexes(assignment)) ||
		runnerEvidence.DiscoveredCount != 16 || len(runnerEvidence.PartitionSHA256) != 64 || !runnerEvidence.BinaryRemoved {
		t.Fatalf("runner validation = %#v err=%v data=%s", runnerEvidence, err, runnerData)
	}
	if runnerEvidence.Outcome != "success" || runnerEvidence.Failure != nil {
		t.Fatalf("completed runner validation = %#v, want a success outcome with no failure detail", runnerEvidence)
	}
}

func raceAssignedShardIndexes(assignment map[int]bool) []int {
	keys := make([]int, 0, len(assignment))
	for key := range assignment {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	return keys
}

func TestCIRacePackageRunnerApextestRejectsUnexpectedCheckoutHead(t *testing.T) {
	root := t.TempDir()
	logPath := filepath.Join(root, "calls.log")
	testNames := raceApextestTestNames()
	goCommand := writeRaceFixture(t, "fake-go", raceApextestGoFixture())
	planner := writeRaceFixture(t, "fake-planner", raceApextestPlannerFixture())
	resource := writeRaceFixture(t, "fake-resource", raceResourceFixture())
	gitCommand := writeRaceFixture(t, "fake-git", "#!/usr/bin/env bash\nprintf '%s\\n' fedcba9876543210fedcba9876543210fedcba98\n")
	partitionDir, runner, batch := writeRaceApextestPartitionForFixture(t, root, goCommand, testNames)
	out, err := runRacePackage(t, root, []string{
		"CI_RACE_GO_COMMAND=" + goCommand, "CI_RACE_SHARD_PLANNER=" + planner,
		"CI_RACE_RESOURCE_RUNNER=" + resource, "CI_RACE_GIT_COMMAND=" + gitCommand,
		"CI_RACE_CALL_LOG=" + logPath, "CI_RACE_TEST_NAMES=" + strings.Join(testNames, "\n") + "\n",
		"CI_RACE_TIMEOUT_COMMAND=" + raceBatchTimeoutFixture(t),
		"CI_RACE_PARTITION_DIR=" + partitionDir, "CI_RACE_APEXTEST_RUNNER=" + runner,
		"CI_RACE_HEAD_SHA=0123456789abcdef0123456789abcdef01234567",
		"CI_RACE_BATCH_TIMEOUT_SECONDS=" + batch, "TMPDIR=" + root,
	}, "./internal/apextest", "internal-apextest")
	if err == nil || !strings.Contains(out, "checkout HEAD does not match expected head") {
		t.Fatalf("unexpected checkout head accepted: %v\n%s", err, out)
	}
	if data, readErr := os.ReadFile(logPath); readErr == nil && strings.Contains(string(data), "start race-") {
		t.Fatalf("unexpected checkout started a resource lane:\n%s", data)
	}
}

// An assigned Apex runner whose shard fails or times out must record the
// failing shard and test with a fail outcome, and the aggregate must report
// that runner's failure by name instead of dying on missing evidence.
func TestCIRacePackageRunnerApextestRecordsAssignedShardFailure(t *testing.T) {
	root := t.TempDir()
	testNames := raceApextestTestNames()
	goCommand := writeRaceFixture(t, "fake-go", raceApextestGoFixture())
	planner := writeRaceFixture(t, "fake-planner", raceApextestPlannerFixture())
	resource := writeRaceFixture(t, "fake-resource", raceResourceFixture())
	gitCommand := writeRaceFixture(t, "fake-git", "#!/usr/bin/env bash\nprintf '%s\\n' 0123456789abcdef0123456789abcdef01234567\n")
	partitionDir, _, _ := writeRaceApextestPartitionForFixture(t, root, goCommand, testNames)
	partitionData, err := os.ReadFile(filepath.Join(partitionDir, "partition.json"))
	if err != nil {
		t.Fatal(err)
	}
	var partition struct {
		Runners []struct {
			Runner              string `json:"runner"`
			ShardIndexes        []int  `json:"shard_indexes"`
			BatchTimeoutSeconds int    `json:"batch_timeout_seconds"`
		} `json:"runners"`
	}
	if err := json.Unmarshal(partitionData, &partition); err != nil {
		t.Fatal(err)
	}
	sortedNames := append([]string(nil), testNames...)
	sort.Strings(sortedNames)
	const failingShard = 1
	failingTest := sortedNames[failingShard]
	var failingRunner string
	for _, entry := range partition.Runners {
		if slices.Contains(entry.ShardIndexes, failingShard) {
			failingRunner = entry.Runner
		}
	}
	if failingRunner == "" {
		t.Fatalf("fixture partition assigns shard %d to no runner", failingShard)
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
		env := []string{
			"CI_RACE_GO_COMMAND=" + goCommand, "CI_RACE_SHARD_PLANNER=" + planner,
			"CI_RACE_RESOURCE_RUNNER=" + resource, "CI_RACE_CALL_LOG=" + filepath.Join(workdir, "calls.log"),
			"CI_RACE_GIT_COMMAND=" + gitCommand, "CI_RACE_HEAD_SHA=0123456789abcdef0123456789abcdef01234567",
			"CI_RACE_TIMEOUT_COMMAND=" + raceBatchTimeoutFixture(t),
			"CI_RACE_PARTITION_DIR=" + partitionDir, "CI_RACE_APEXTEST_RUNNER=" + entry.Runner,
			"CI_RACE_BATCH_TIMEOUT_SECONDS=" + strconv.Itoa(entry.BatchTimeoutSeconds),
			"CI_RACE_TEST_NAMES=" + strings.Join(testNames, "\n") + "\n", "TMPDIR=" + workdir,
		}
		if entry.Runner == failingRunner {
			env = append(env, "CI_RACE_DIRECT_EVENTS=fail", "CI_RACE_DIRECT_FAIL_TEST="+failingTest, "CI_RACE_DIRECT_STATUS=31")
		}
		out, err := runRacePackage(t, workdir, env, "./internal/apextest", "internal-apextest")
		if entry.Runner == failingRunner {
			if exit := raceExitCode(err); exit != 31 {
				t.Fatalf("failing runner exit = %d, want the go test status 31:\n%s", exit, out)
			}
			record, err := os.ReadFile(filepath.Join(workdir, "ci-artifacts/race/internal-apextest/runner-validation.json"))
			if err != nil {
				t.Fatalf("failing runner wrote no validation record: %v\n%s", err, out)
			}
			var validation struct {
				Outcome string `json:"outcome"`
				Failure struct {
					Shard        int      `json:"shard"`
					ExitStatus   int      `json:"exit_status"`
					FailingTests []string `json:"failing_tests"`
				} `json:"failure"`
			}
			if err := json.Unmarshal(record, &validation); err != nil || validation.Outcome != "fail" ||
				validation.Failure.Shard != failingShard || validation.Failure.ExitStatus != 31 ||
				fmt.Sprint(validation.Failure.FailingTests) != fmt.Sprint([]string{failingTest}) {
				t.Fatalf("failing runner validation = %s err=%v", record, err)
			}
		} else if err != nil {
			t.Fatalf("clean runner %s failed: %v\n%s", entry.Runner, err, out)
		}
		if err := os.CopyFS(filepath.Join(runnersRoot, fmt.Sprintf("race-internal-apextest-%s-44158211531-1", entry.Runner)), os.DirFS(filepath.Join(workdir, "ci-artifacts/race/internal-apextest"))); err != nil {
			t.Fatal(err)
		}
	}
	script, err := filepath.Abs("ci-race-apextest-aggregate.sh")
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(root, "aggregate.json")
	args := []string{script, partitionDir, runnersRoot, output, "0123456789abcdef0123456789abcdef01234567"}
	verdict, err := exec.Command("bash", args...).CombinedOutput()
	if err == nil {
		t.Fatalf("aggregate accepted a failed assigned shard:\n%s", verdict)
	}
	if !strings.Contains(string(verdict), failingRunner) || !strings.Contains(string(verdict), "shard 1") || !strings.Contains(string(verdict), failingTest) {
		t.Fatalf("aggregate must name the failing runner, shard and test:\n%s", verdict)
	}
	summary, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("aggregate must still write its verdict document: %v", err)
	}
	var rejected struct {
		Valid    bool     `json:"valid"`
		Verdicts []string `json:"verdicts"`
	}
	if json.Unmarshal(summary, &rejected) != nil || rejected.Valid {
		t.Fatalf("aggregate verdict = %s", summary)
	}
	joined := strings.Join(rejected.Verdicts, "\n")
	if !strings.Contains(joined, failingRunner) || !strings.Contains(joined, failingTest) {
		t.Fatalf("aggregate verdicts = %s", summary)
	}
}

// The partition producer discovers once from the verified shared race binary,
// scales the normal history, plans the sixteen shards, and freezes runner
// assignments and deadlines that both the runner and the aggregate verify.
func TestCIRacePackageRunnerApextestPartitionPlansOnceFromTheBinary(t *testing.T) {
	history := racePlanApexNormalHistory(t)
	names := make([]string, 0, 16)
	for name := range history {
		names = append(names, name)
		if len(names) == 16 {
			break
		}
	}
	sort.Strings(names)
	root := t.TempDir()
	logPath := filepath.Join(root, "calls.log")
	goCommand := writeRaceFixture(t, "fake-go", raceApextestGoFixture())
	planner := writeRaceFixture(t, "fake-planner", raceApextestHistoryPlannerFixture())
	resource := writeRaceFixture(t, "fake-resource", raceResourceFixture())
	gitCommand := writeRaceFixture(t, "fake-git", "#!/usr/bin/env bash\nprintf '%s\\n' 0123456789abcdef0123456789abcdef01234567\n")
	script, err := filepath.Abs("ci-race-test.sh")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", script, "partition", "./internal/apextest", "internal-apextest")
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"CI_RACE_GO_COMMAND="+goCommand, "CI_RACE_SHARD_PLANNER="+planner,
		"CI_RACE_RESOURCE_RUNNER="+resource, "CI_RACE_GIT_COMMAND="+gitCommand,
		"CI_RACE_CALL_LOG="+logPath, "CI_RACE_TEST_NAMES="+strings.Join(names, "\n")+"\n",
		"CI_RACE_TIMEOUT_COMMAND="+raceBatchTimeoutFixture(t),
		"CI_RACE_HEAD_SHA=0123456789abcdef0123456789abcdef01234567",
		"CI_RACE_BINARY_DISCOVERY_PASS=1", "TMPDIR="+root,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("partition production failed: %v\n%s", err, out)
	}
	artifactDir := filepath.Join(root, "ci-artifacts", "race", "internal-apextest")
	if _, err := os.Stat(filepath.Join(artifactDir, "runner-validation.json")); err == nil {
		t.Fatal("partition production must not claim a runner assignment")
	}
	if out, err := exec.Command("python3", "ci-race-plan.py", "verify", artifactDir).CombinedOutput(); err != nil {
		t.Fatalf("produced partition does not verify: %v\n%s", err, out)
	}
	partitionData, err := os.ReadFile(filepath.Join(artifactDir, "partition.json"))
	if err != nil {
		t.Fatal(err)
	}
	var partition struct {
		Runners []struct {
			Runner       string `json:"runner"`
			ShardIndexes []int  `json:"shard_indexes"`
		} `json:"runners"`
		Shards []struct {
			TestTimeoutSeconds int `json:"test_timeout_seconds"`
		} `json:"shards"`
		BinarySHA256 string `json:"binary_sha256"`
	}
	if err := json.Unmarshal(partitionData, &partition); err != nil {
		t.Fatal(err)
	}
	if len(partition.Shards) != 16 || len(partition.Runners) < 3 {
		t.Fatalf("partition topology = %d shards, %d runners", len(partition.Shards), len(partition.Runners))
	}
	assigned := map[int]bool{}
	for _, runner := range partition.Runners {
		for _, index := range runner.ShardIndexes {
			if assigned[index] {
				t.Fatalf("shard %d assigned to two runners", index)
			}
			assigned[index] = true
		}
	}
	for index := 0; index < 16; index++ {
		if !assigned[index] {
			t.Fatalf("shard %d unassigned", index)
		}
		if partition.Shards[index].TestTimeoutSeconds != 900 {
			t.Fatalf("shard %d deadline = %d, want the 900s floor", index, partition.Shards[index].TestTimeoutSeconds)
		}
	}
	if len(partition.BinarySHA256) != 64 {
		t.Fatalf("partition binary digest = %q", partition.BinarySHA256)
	}
	// The scaled normal history is the planner input the partition recorded.
	scaledData, err := os.ReadFile(filepath.Join(artifactDir, "race-history.json"))
	if err != nil {
		t.Fatalf("scaled history evidence: %v", err)
	}
	var scaled struct {
		Tests []struct {
			Name           string `json:"name"`
			DurationMillis int    `json:"durationMillis"`
		} `json:"tests"`
	}
	if err := json.Unmarshal(scaledData, &scaled); err != nil {
		t.Fatal(err)
	}
	if len(scaled.Tests) != len(names) {
		t.Fatalf("scaled history covers %d tests, want %d", len(scaled.Tests), len(names))
	}
	for _, row := range scaled.Tests {
		if want := history[row.Name] * 4; row.DurationMillis != want {
			t.Errorf("%s scaled to %d ms, want %d", row.Name, row.DurationMillis, want)
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), "apextest.test") {
			t.Fatalf("partition production left binary %s", entry.Name())
		}
	}
}

func raceApextestHistoryPlannerFixture() string {
	return `#!/usr/bin/env bash
set -euo pipefail
package=""
tests=""
history=""
while [[ "$#" -gt 0 ]]; do
  case "$1" in
    --package) package="$2"; shift 2 ;;
    --shards) [[ "$2" == 16 ]]; shift 2 ;;
    --tests) tests="$2"; shift 2 ;;
    --history) history="$2"; shift 2 ;;
    *) exit 91 ;;
  esac
done
[[ -n "$history" && -s "$history" ]] || exit 95
names=()
while IFS= read -r name; do names+=("$name"); done <"$tests"
[[ "${#names[@]}" == 16 ]]
printf '{"version":1,"package":"%s","historyUsed":true,"shards":[' "$package"
for index in 0 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15; do
  [[ "$index" == 0 ]] || printf ','
  printf '{"index":%s,"tests":["%s"],"estimatedDurationMillis":0,"regex":"^(?:%s)$"}' "$index" "${names[$index]}" "${names[$index]}"
done
printf ']}\n'
`
}

// A runner cannot execute an assignment the partition did not give it, and
// the workflow cannot pass it a batch deadline the partition did not plan.
func TestCIRacePackageRunnerApextestRejectsBrokenPartitions(t *testing.T) {
	flow := func(t *testing.T, mutate func(partitionDir string), runnerOverride string, extraEnv []string) (string, error) {
		t.Helper()
		root := t.TempDir()
		testNames := raceApextestTestNames()
		goCommand := writeRaceFixture(t, "fake-go", raceApextestGoFixture())
		resource := writeRaceFixture(t, "fake-resource", raceResourceFixture())
		gitCommand := writeRaceFixture(t, "fake-git", "#!/usr/bin/env bash\nprintf '%s\\n' 0123456789abcdef0123456789abcdef01234567\n")
		partitionDir, runner, batch := writeRaceApextestPartitionForFixture(t, root, goCommand, testNames)
		if runnerOverride != "" {
			runner = runnerOverride
		}
		if mutate != nil {
			mutate(partitionDir)
		}
		base := []string{
			"CI_RACE_GO_COMMAND=" + goCommand,
			"CI_RACE_RESOURCE_RUNNER=" + resource, "CI_RACE_GIT_COMMAND=" + gitCommand,
			"CI_RACE_CALL_LOG=" + filepath.Join(root, "calls.log"),
			"CI_RACE_TEST_NAMES=" + strings.Join(testNames, "\n") + "\n",
			"CI_RACE_TIMEOUT_COMMAND=" + raceBatchTimeoutFixture(t),
			"CI_RACE_PARTITION_DIR=" + partitionDir, "CI_RACE_APEXTEST_RUNNER=" + runner,
			"CI_RACE_HEAD_SHA=0123456789abcdef0123456789abcdef01234567",
			"CI_RACE_BATCH_TIMEOUT_SECONDS=" + batch, "TMPDIR=" + root,
		}
		return runRacePackage(t, root, append(base, extraEnv...), "./internal/apextest", "internal-apextest")
	}
	tests := []struct {
		name           string
		mutate         func(partitionDir string)
		runnerOverride string
		env            []string
	}{
		{name: "obsolete shard indexes", env: []string{"CI_RACE_APEXTEST_SHARD_INDEXES=0,3,5"}},
		{name: "unknown runner", runnerOverride: "z"},
		{name: "workflow batch mismatch", env: []string{"CI_RACE_BATCH_TIMEOUT_SECONDS=9999"}},
		{name: "tightened shard deadline", mutate: func(partitionDir string) {
			path := filepath.Join(partitionDir, "partition.json")
			data, err := os.ReadFile(path)
			if err != nil {
				panic(err)
			}
			if err := os.WriteFile(path, []byte(strings.Replace(string(data), `"test_timeout_seconds":900`, `"test_timeout_seconds":600`, 1)), 0o600); err != nil {
				panic(err)
			}
		}},
		{name: "tampered discovery", mutate: func(partitionDir string) {
			if err := os.WriteFile(filepath.Join(partitionDir, "discovery.txt"), []byte("TestIntruder\n"), 0o600); err != nil {
				panic(err)
			}
		}},
		{name: "wrong binary identity", mutate: func(partitionDir string) {
			path := filepath.Join(partitionDir, "partition.json")
			data, err := os.ReadFile(path)
			if err != nil {
				panic(err)
			}
			if err := os.WriteFile(path, []byte(strings.Replace(string(data), `"binary_sha256":"`, `"binary_sha256":"f`, 1)), 0o600); err != nil {
				panic(err)
			}
		}},
	}
	for _, fixture := range tests {
		t.Run(fixture.name, func(t *testing.T) {
			out, err := flow(t, fixture.mutate, fixture.runnerOverride, fixture.env)
			if err == nil {
				t.Fatalf("broken partition accepted:\n%s", out)
			}
		})
	}
}

// writeApextestPartitionEvidence builds one partition artifact plus a
// runners-root whose per-runner directories carry complete evidence for the
// exact assignment the partition planned. The plan authority (not this
// fixture) computes the partition, so the aggregate tests exercise the same
// manifest the workflow consumes.
func writeApextestPartitionEvidence(t *testing.T, root string, testCount int) (partitionDir, runnersRoot string, assignments map[string][]int) {
	t.Helper()
	headSHA := "0123456789abcdef0123456789abcdef01234567"
	names := make([]string, testCount)
	for index := range names {
		names[index] = fmt.Sprintf("TestCase%03d", index)
	}
	sort.Strings(names)
	partitionDir = filepath.Join(root, "partition")
	runnersRoot = filepath.Join(root, "runners")
	for _, dir := range []string{partitionDir, runnersRoot} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	discovery := strings.Join(names, "\n") + "\n"
	discoveryPath := filepath.Join(partitionDir, "discovery.txt")
	if err := os.WriteFile(discoveryPath, []byte(discovery), 0o600); err != nil {
		t.Fatal(err)
	}
	shards := make([]map[string]any, 16)
	shardTests := make([][]string, 16)
	for index := range shards {
		for nameIndex := index; nameIndex < len(names); nameIndex += 16 {
			shardTests[index] = append(shardTests[index], names[nameIndex])
		}
		shards[index] = map[string]any{"index": index, "tests": shardTests[index], "estimatedDurationMillis": 0, "regex": "^(?:" + strings.Join(shardTests[index], "|") + ")$"}
	}
	planData, err := json.Marshal(map[string]any{"version": 1, "package": "github.com/glade-sh/glade/internal/apextest", "historyUsed": false, "shards": shards})
	if err != nil {
		t.Fatal(err)
	}
	planData = append(planData, '\n')
	planPath := filepath.Join(partitionDir, "plan.json")
	if err := os.WriteFile(planPath, planData, 0o600); err != nil {
		t.Fatal(err)
	}
	binarySHA := strings.Repeat("a", 64)
	partitionOutput, err := exec.Command("python3", "ci-race-plan.py", "schedule", discoveryPath, planPath, binarySHA, "1048576").CombinedOutput()
	if err != nil {
		t.Fatalf("fixture partition schedule failed: %v\n%s", err, partitionOutput)
	}
	if err := os.WriteFile(filepath.Join(partitionDir, "partition.json"), partitionOutput, 0o600); err != nil {
		t.Fatal(err)
	}
	var partition struct {
		Runners []struct {
			Runner       string `json:"runner"`
			ShardIndexes []int  `json:"shard_indexes"`
		} `json:"runners"`
	}
	if err := json.Unmarshal(partitionOutput, &partition); err != nil {
		t.Fatal(err)
	}
	assignments = make(map[string][]int, len(partition.Runners))
	partitionSHA := fmt.Sprintf("%x", sha256.Sum256(partitionOutput))
	discoverySHA := fmt.Sprintf("%x", sha256.Sum256([]byte(discovery)))
	planSHA := fmt.Sprintf("%x", sha256.Sum256(planData))
	for _, entry := range partition.Runners {
		assignments[entry.Runner] = entry.ShardIndexes
		runnerDir := filepath.Join(runnersRoot, fmt.Sprintf("race-internal-apextest-%s-44158211531-1", entry.Runner))
		if err := os.MkdirAll(runnerDir, 0o700); err != nil {
			t.Fatal(err)
		}
		runnerData, err := json.Marshal(map[string]any{
			"schema_version": 1, "runner": entry.Runner, "package": "github.com/glade-sh/glade/internal/apextest", "head_sha": headSHA,
			"assigned_indexes": entry.ShardIndexes, "discovered_count": len(names), "discovery_sha256": discoverySHA, "plan_sha256": planSHA,
			"partition_sha256": partitionSHA, "binary_sha256": binarySHA, "binary_size_bytes": 1048576, "binary_removed": true,
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(runnerDir, "runner-validation.json"), append(runnerData, '\n'), 0o600); err != nil {
			t.Fatal(err)
		}
		binaryData := []byte(`{"schema_version":1,"package":"github.com/glade-sh/glade/internal/apextest","sha256":"` + binarySHA + `","size_bytes":1048576,"removed":true}` + "\n")
		if err := os.WriteFile(filepath.Join(runnerDir, "binary.json"), binaryData, 0o600); err != nil {
			t.Fatal(err)
		}
		resource := `{"schema_version":1,"lane":"race-internal-apextest-build","elapsed_seconds":0,"user_seconds":0,"system_seconds":0,"max_rss_kb":1,"exit_status":0}` + "\n"
		if err := os.WriteFile(filepath.Join(runnerDir, "build-resource.json"), []byte(resource), 0o600); err != nil {
			t.Fatal(err)
		}
		for _, index := range entry.ShardIndexes {
			shardDir := filepath.Join(runnerDir, fmt.Sprintf("shard-%d", index))
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
			events := make([]string, 0, len(shardTests[index]))
			for _, name := range shardTests[index] {
				events = append(events, fmt.Sprintf(`{"Action":"pass","Package":"github.com/glade-sh/glade/internal/apextest","Test":"%s"}`, name))
			}
			events = append(events, `{"Action":"pass","Package":"github.com/glade-sh/glade/internal/apextest"}`)
			if err := os.WriteFile(filepath.Join(shardDir, "events.json"), []byte(strings.Join(events, "\n")+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			resource := fmt.Sprintf(`{"schema_version":1,"lane":"race-internal-apextest-shard-%d","elapsed_seconds":0,"user_seconds":0,"system_seconds":0,"max_rss_kb":1,"exit_status":0}`+"\n", index)
			if err := os.WriteFile(filepath.Join(shardDir, "resource.json"), []byte(resource), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	return partitionDir, runnersRoot, assignments
}

func runApextestAggregate(t *testing.T, partitionDir, runnersRoot, output string) (string, error) {
	t.Helper()
	script, err := filepath.Abs("ci-race-apextest-aggregate.sh")
	if err != nil {
		t.Fatal(err)
	}
	result, err := exec.Command("bash", script, partitionDir, runnersRoot, output, "0123456789abcdef0123456789abcdef01234567").CombinedOutput()
	return string(result), err
}

func TestCIRaceApextestAggregateRejectsMismatchedEvidence(t *testing.T) {
	root := t.TempDir()
	partitionDir, runnersRoot, _ := writeApextestPartitionEvidence(t, root, 332)
	out := filepath.Join(root, "aggregate.json")
	if output, err := runApextestAggregate(t, partitionDir, runnersRoot, out); err != nil {
		t.Fatalf("valid aggregate evidence rejected: %v\n%s", err, output)
	}
	data, err := os.ReadFile(out)
	if err != nil || !strings.Contains(string(data), `"discovered_count":332`) || !strings.Contains(string(data), `"valid":true`) {
		t.Fatalf("aggregate evidence = %s err=%v", data, err)
	}
	entries, err := os.ReadDir(runnersRoot)
	if err != nil || len(entries) == 0 {
		t.Fatalf("runner evidence root = %v err=%v", entries, err)
	}
	if err := os.WriteFile(filepath.Join(runnersRoot, entries[0].Name(), "runner-validation.json"), []byte(`{"schema_version":1}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if output, err := runApextestAggregate(t, partitionDir, runnersRoot, out); err == nil || !strings.Contains(output, "invalid runner validation") {
		t.Fatalf("malformed runner evidence accepted: %v\n%s", err, output)
	}
}

func TestCIRaceApextestAggregateDerivesDiscoveryCount(t *testing.T) {
	root := t.TempDir()
	partitionDir, runnersRoot, _ := writeApextestPartitionEvidence(t, root, 333)
	out := filepath.Join(root, "aggregate.json")
	output, err := runApextestAggregate(t, partitionDir, runnersRoot, out)
	if err != nil {
		t.Fatalf("aggregate rejected matching 333-test discovery: %v\n%s", err, output)
	}
	data, err := os.ReadFile(out)
	if err != nil || !strings.Contains(string(data), `"discovered_count":333`) || !strings.Contains(string(data), `"valid":true`) {
		t.Fatalf("derived aggregate evidence = %s err=%v", data, err)
	}
}

func TestCIRaceApextestAggregateRejectsAdversarialEvidence(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(t *testing.T, root, partitionDir, runnersRoot string)
	}{
		{name: "missing runner c", mutate: func(t *testing.T, root, partitionDir, runnersRoot string) {
			entries, err := os.ReadDir(runnersRoot)
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), "race-internal-apextest-c-") {
					if err := os.RemoveAll(filepath.Join(runnersRoot, entry.Name())); err != nil {
						t.Fatal(err)
					}
					return
				}
			}
			t.Fatal("fixture has no runner c evidence")
		}},
		{name: "mismatched head", mutate: func(t *testing.T, root, partitionDir, runnersRoot string) {
			t.Helper()
			path := apextestRunnerEvidencePath(t, runnersRoot, "b", "runner-validation.json")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(strings.Replace(string(data), "0123456789abcdef0123456789abcdef01234567", "fedcba9876543210fedcba9876543210fedcba98", 1)), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "mismatched package", mutate: func(t *testing.T, root, partitionDir, runnersRoot string) {
			t.Helper()
			path := apextestRunnerEvidencePath(t, runnersRoot, "b", "runner-validation.json")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(strings.Replace(string(data), "github.com/glade-sh/glade/internal/apextest", "wrong/package", 1)), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "omitted assignment", mutate: func(t *testing.T, root, partitionDir, runnersRoot string) {
			t.Helper()
			path := apextestRunnerEvidencePath(t, runnersRoot, "c", "runner-validation.json")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var evidence struct {
				AssignedIndexes []int `json:"assigned_indexes"`
			}
			if err := json.Unmarshal(data, &evidence); err != nil {
				t.Fatal(err)
			}
			updated, err := json.Marshal(map[string]any{
				"schema_version": 1, "runner": "c", "package": "github.com/glade-sh/glade/internal/apextest", "head_sha": "0123456789abcdef0123456789abcdef01234567",
				"assigned_indexes": evidence.AssignedIndexes[:len(evidence.AssignedIndexes)-1], "discovered_count": 333,
				"discovery_sha256": apextestDigestField(t, path, "discovery_sha256"), "plan_sha256": apextestDigestField(t, path, "plan_sha256"),
				"partition_sha256": apextestDigestField(t, path, "partition_sha256"), "binary_sha256": apextestDigestField(t, path, "binary_sha256"),
				"binary_size_bytes": 1048576, "binary_removed": true,
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, append(updated, '\n'), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "duplicate assignment", mutate: func(t *testing.T, root, partitionDir, runnersRoot string) {
			t.Helper()
			path := apextestRunnerEvidencePath(t, runnersRoot, "c", "runner-validation.json")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var evidence struct {
				AssignedIndexes []int `json:"assigned_indexes"`
			}
			if err := json.Unmarshal(data, &evidence); err != nil {
				t.Fatal(err)
			}
			duplicated := append(append([]int(nil), evidence.AssignedIndexes...), evidence.AssignedIndexes[len(evidence.AssignedIndexes)-1])
			sort.Ints(duplicated)
			updated, err := json.Marshal(map[string]any{
				"schema_version": 1, "runner": "c", "package": "github.com/glade-sh/glade/internal/apextest", "head_sha": "0123456789abcdef0123456789abcdef01234567",
				"assigned_indexes": duplicated, "discovered_count": 333,
				"discovery_sha256": apextestDigestField(t, path, "discovery_sha256"), "plan_sha256": apextestDigestField(t, path, "plan_sha256"),
				"partition_sha256": apextestDigestField(t, path, "partition_sha256"), "binary_sha256": apextestDigestField(t, path, "binary_sha256"),
				"binary_size_bytes": 1048576, "binary_removed": true,
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, append(updated, '\n'), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "extra runner", mutate: func(t *testing.T, root, partitionDir, runnersRoot string) {
			if err := os.MkdirAll(filepath.Join(runnersRoot, "race-internal-apextest-z-44158211531-1"), 0o700); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "stale partition digest", mutate: func(t *testing.T, root, partitionDir, runnersRoot string) {
			t.Helper()
			path := apextestRunnerEvidencePath(t, runnersRoot, "b", "runner-validation.json")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(strings.Replace(string(data), `"partition_sha256":"`, `"partition_sha256":"0`, 1)), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "wrong binary", mutate: func(t *testing.T, root, partitionDir, runnersRoot string) {
			t.Helper()
			path := apextestRunnerEvidencePath(t, runnersRoot, "b", "binary.json")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(strings.Replace(string(data), strings.Repeat("a", 64), strings.Repeat("b", 64), 1)), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "tampered partition plan", mutate: func(t *testing.T, root, partitionDir, runnersRoot string) {
			path := filepath.Join(partitionDir, "plan.json")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(strings.Replace(string(data), `"TestCase004"`, `"TestCase999"`, 1)), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "unassigned shard executed", mutate: func(t *testing.T, root, partitionDir, runnersRoot string) {
			t.Helper()
			var partition struct {
				Runners []struct {
					Runner       string `json:"runner"`
					ShardIndexes []int  `json:"shard_indexes"`
				} `json:"runners"`
			}
			partitionData, err := os.ReadFile(filepath.Join(partitionDir, "partition.json"))
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(partitionData, &partition); err != nil {
				t.Fatal(err)
			}
			owned := map[string]map[int]bool{}
			for _, entry := range partition.Runners {
				owned[entry.Runner] = map[int]bool{}
				for _, index := range entry.ShardIndexes {
					owned[entry.Runner][index] = true
				}
			}
			for _, entry := range partition.Runners {
				for index := 0; index < 16; index++ {
					if !owned[entry.Runner][index] {
						shardDir := filepath.Join(apextestRunnerEvidenceDir(t, runnersRoot, entry.Runner), fmt.Sprintf("shard-%d", index))
						if err := os.MkdirAll(shardDir, 0o700); err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(filepath.Join(shardDir, "events.json"), []byte("{\"Action\":\"pass\"}\n"), 0o600); err != nil {
							t.Fatal(err)
						}
						return
					}
				}
			}
		}},
		{name: "malformed resource", mutate: func(t *testing.T, root, partitionDir, runnersRoot string) {
			t.Helper()
			var partition struct {
				Runners []struct {
					Runner       string `json:"runner"`
					ShardIndexes []int  `json:"shard_indexes"`
				} `json:"runners"`
			}
			partitionData, err := os.ReadFile(filepath.Join(partitionDir, "partition.json"))
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(partitionData, &partition); err != nil {
				t.Fatal(err)
			}
			for _, entry := range partition.Runners {
				if slices.Contains(entry.ShardIndexes, 7) {
					if err := os.WriteFile(filepath.Join(apextestRunnerEvidenceDir(t, runnersRoot, entry.Runner), "shard-7", "resource.json"), []byte("{}\n"), 0o600); err != nil {
						t.Fatal(err)
					}
					return
				}
			}
		}},
		{name: "skipped terminal", mutate: func(t *testing.T, root, partitionDir, runnersRoot string) {
			t.Helper()
			mutateApextestShardEvents(t, runnersRoot, func(body string) string {
				return strings.Replace(body, `"Action":"pass"`, `"Action":"skip"`, 1)
			})
		}},
		{name: "failed terminal", mutate: func(t *testing.T, root, partitionDir, runnersRoot string) {
			t.Helper()
			mutateApextestShardEvents(t, runnersRoot, func(body string) string {
				return strings.Replace(body, `"Action":"pass"`, `"Action":"fail"`, 1)
			})
		}},
		{name: "duplicate terminal", mutate: func(t *testing.T, root, partitionDir, runnersRoot string) {
			t.Helper()
			mutateApextestShardEvents(t, runnersRoot, func(body string) string {
				first, _, found := strings.Cut(body, "\n")
				if !found {
					t.Fatal("missing fixture event")
				}
				return body + first + "\n"
			})
		}},
	}
	for _, fixture := range tests {
		t.Run(fixture.name, func(t *testing.T) {
			// Each child owns its artifacts, mutations and validator output.
			t.Parallel()
			root := t.TempDir()
			partitionDir, runnersRoot, _ := writeApextestPartitionEvidence(t, root, 333)
			fixture.mutate(t, root, partitionDir, runnersRoot)
			if output, err := runApextestAggregate(t, partitionDir, runnersRoot, filepath.Join(root, "aggregate.json")); err == nil {
				t.Fatalf("adversarial evidence accepted:\n%s", output)
			}
		})
	}
}

func apextestRunnerEvidenceDir(t *testing.T, runnersRoot, runner string) string {
	t.Helper()
	entries, err := os.ReadDir(runnersRoot)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "race-internal-apextest-"+runner+"-") {
			return filepath.Join(runnersRoot, entry.Name())
		}
	}
	t.Fatalf("fixture has no runner %s evidence", runner)
	return ""
}

func apextestRunnerEvidencePath(t *testing.T, runnersRoot, runner, name string) string {
	t.Helper()
	return filepath.Join(apextestRunnerEvidenceDir(t, runnersRoot, runner), name)
}

func apextestDigestField(t *testing.T, path, field string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	digest, _ := value[field].(string)
	if len(digest) != 64 {
		t.Fatalf("fixture field %s = %q", field, digest)
	}
	return digest
}

func mutateApextestShardEvents(t *testing.T, runnersRoot string, mutate func(string) string) {
	t.Helper()
	var partition struct {
		Runners []struct {
			Runner       string `json:"runner"`
			ShardIndexes []int  `json:"shard_indexes"`
		} `json:"runners"`
	}
	partitionData, err := os.ReadFile(filepath.Join(filepath.Dir(runnersRoot), "partition", "partition.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(partitionData, &partition); err != nil {
		t.Fatal(err)
	}
	for _, entry := range partition.Runners {
		if slices.Contains(entry.ShardIndexes, 7) {
			path := filepath.Join(apextestRunnerEvidenceDir(t, runnersRoot, entry.Runner), "shard-7", "events.json")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(mutate(string(data))), 0o600); err != nil {
				t.Fatal(err)
			}
			return
		}
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
			t.Parallel()
			root := t.TempDir()
			logPath := filepath.Join(root, "calls.log")
			testNames := raceApextestTestNames()
			goCommand := writeRaceFixture(t, "fake-go", raceApextestGoFixture())
			plannerText := raceApextestPlannerFixture()
			switch fixture.planner {
			case "omission":
				plannerText = strings.Replace(plannerText, `[[ "${#names[@]}" == 16 ]]`, `names=("${names[@]:0:15}" "TestUnknown")`, 1)
			case "duplicate":
				plannerText = strings.Replace(plannerText, `[[ "${#names[@]}" == 16 ]]`, `names[15]="${names[0]}"`, 1)
			case "empty":
				plannerText = strings.Replace(plannerText, `for index in 0 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15; do`, `names[15]=""; for index in 0 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15; do`, 1)
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
			t.Parallel()
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, "internal", "apextest"), 0o700); err != nil {
				t.Fatal(err)
			}
			logPath := filepath.Join(root, "calls.log")
			runnerPIDPath := filepath.Join(root, "runner.pid")
			testNames := raceApextestTestNames()
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
printf '{"Action":"pass","Package":"github.com/glade-sh/glade/internal/playground"}\n'
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
			t.Parallel()
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
printf '{"Action":"pass","Package":"github.com/glade-sh/glade/internal/playground"}\n'
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
			t.Parallel()
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
			t.Parallel()
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
printf '{"Action":"pass","Package":"github.com/glade-sh/glade/internal/playground"}\n'
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

func TestCIL08RaceRoutesRequireBrowser(t *testing.T) {
	data, err := os.ReadFile("../.github/workflows/race.yml")
	if err != nil {
		t.Fatal(err)
	}
	jobs := workflowJobBlocksText(string(data))
	for _, name := range []string{"race", "race-push"} {
		t.Run(name, func(t *testing.T) {
			environment, _, ok := strings.Cut(jobs[name], "    steps:")
			if !ok || !strings.Contains(environment, "      GLADE_LWC_BROWSER: \"1\"\n") {
				t.Fatal("provisioned race server route must enable L08 browser conformance")
			}
		})
	}
}
