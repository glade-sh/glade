package scripts

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestCIGoCacheReportsMissingDirectoriesAsZero(t *testing.T) {
	readWorkflow := func(name string) string {
		t.Helper()
		workflowPath := filepath.Join("..", ".github", "workflows", name)
		workflow, err := os.ReadFile(workflowPath)
		if err != nil {
			t.Fatalf("read %s: %v", workflowPath, err)
		}
		return string(workflow)
	}
	extractReportScripts := func(workflow string) []string {
		lines := strings.Split(workflow, "\n")
		var scripts []string
		for i, line := range lines {
			if !strings.Contains(line, "- name: Report Go ") {
				continue
			}
			for j := i + 1; j < len(lines); j++ {
				if strings.TrimSpace(lines[j]) != "run: |" {
					continue
				}
				runIndent := len(lines[j]) - len(strings.TrimLeft(lines[j], " "))
				var script strings.Builder
				for k := j + 1; k < len(lines); k++ {
					indent := len(lines[k]) - len(strings.TrimLeft(lines[k], " "))
					if strings.TrimSpace(lines[k]) != "" && indent <= runIndent {
						break
					}
					script.WriteString(strings.TrimPrefix(lines[k], strings.Repeat(" ", runIndent+2)))
					script.WriteByte('\n')
				}
				scripts = append(scripts, script.String())
				break
			}
		}
		return scripts
	}

	for _, workflow := range []struct {
		name string
		text string
		want int
	}{
		{name: "ci.yml", text: readWorkflow("ci.yml"), want: 2},
		{name: "security.yml", text: readWorkflow("security.yml"), want: 6},
	} {
		scripts := extractReportScripts(workflow.text)
		if len(scripts) != workflow.want {
			t.Fatalf("%s report scripts = %d, want %d", workflow.name, len(scripts), workflow.want)
		}
		for i, script := range scripts {
			cmd := exec.Command("bash", "--noprofile", "--norc", "-e", "-o", "pipefail", "-c", script)
			cmd.Env = []string{
				"HOME=" + t.TempDir(),
				"MATCHED_KEY=",
				"PATH=" + os.Getenv("PATH"),
			}
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("%s report script %d failed for missing cache directory: %v\n%s", workflow.name, i+1, err, out)
			}
			if !strings.Contains(string(out), "bytes=0") {
				t.Fatalf("%s report script %d output missing bytes=0:\n%s", workflow.name, i+1, out)
			}
		}
	}
}

func TestCIGoCacheOwnership(t *testing.T) {
	readWorkflow := func(name string) string {
		t.Helper()
		workflowPath := filepath.Join("..", ".github", "workflows", name)
		workflow, err := os.ReadFile(workflowPath)
		if err != nil {
			t.Fatalf("read %s: %v", workflowPath, err)
		}
		return string(workflow)
	}

	ciWorkflow := readWorkflow("ci.yml")
	ci := workflowJobBlocks(t, ciWorkflow)["test"]
	security := readWorkflow("security.yml")

	if got := strings.Count(ci, "actions/setup-go@v6"); got != 1 {
		t.Fatalf("ci.yml setup-go uses = %d, want 1", got)
	}
	if got := strings.Count(ci, "cache: false"); got != 1 {
		t.Fatalf("ci.yml setup-go cache opt-outs = %d, want 1", got)
	}
	if got := strings.Count(security, "actions/setup-go@v6"); got != 3 {
		t.Fatalf("security.yml setup-go uses = %d, want 3", got)
	}
	if got := strings.Count(security, "cache: false"); got != 3 {
		t.Fatalf("security.yml setup-go cache opt-outs = %d, want 3", got)
	}

	if got := strings.Count(ci, "actions/cache/restore@v4"); got != 2 {
		t.Fatalf("ci.yml cache restores = %d, want 2", got)
	}
	if got := strings.Count(ci, "actions/cache/save@v4"); got != 2 {
		t.Fatalf("ci.yml cache saves = %d, want 2", got)
	}
	if got := strings.Count(security, "actions/cache/restore@v4"); got != 6 {
		t.Fatalf("security.yml cache restores = %d, want 6", got)
	}
	if got := strings.Count(security, "actions/cache/save@v4"); got != 0 {
		t.Fatalf("security.yml cache saves = %d, want 0", got)
	}
	if got := strings.Count(ci, "continue-on-error: true"); got != 4 {
		t.Fatalf("ci.yml non-fatal cache actions = %d, want 4", got)
	}
	if got := strings.Count(security, "continue-on-error: true"); got != 6 {
		t.Fatalf("security.yml non-fatal cache actions = %d, want 6", got)
	}

	for _, workflow := range []struct {
		name string
		text string
	}{
		{name: "ci.yml", text: ci},
		{name: "security.yml", text: security},
	} {
		for _, want := range []string{"path: ~/go/pkg/mod", "path: ~/.cache/go-build", "go-mod-cache-v1", "go-build-cache-v1"} {
			if !strings.Contains(workflow.text, want) {
				t.Errorf("%s missing separate cache marker %q", workflow.name, want)
			}
		}
		for _, line := range strings.Split(workflow.text, "\n") {
			if strings.TrimSpace(line) == "path: |" {
				t.Errorf("%s contains a combined multi-path cache", workflow.name)
			}
		}
	}

	fullKeys := func(workflow string) []string {
		var keys []string
		for _, line := range strings.Split(workflow, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "key: ") && strings.Contains(line, "github.sha") {
				keys = append(keys, strings.TrimPrefix(line, "key: "))
			}
		}
		return keys
	}
	assertFullKeys := func(name, workflow, sumExpression string, want int) {
		t.Helper()
		keys := fullKeys(workflow)
		if len(keys) != want {
			t.Errorf("%s full cache keys = %d, want %d", name, len(keys), want)
		}
		for _, key := range keys {
			for _, dimension := range []string{
				"cache-v1", "1.26.5", "runner.os", "runner.arch", sumExpression,
				"github.sha", "github.run_id", "github.run_attempt",
			} {
				if !strings.Contains(key, dimension) {
					t.Errorf("%s full key %q missing dimension %q", name, key, dimension)
				}
			}
			if !strings.Contains(key, "go-mod-") && !strings.Contains(key, "go-build-") {
				t.Errorf("%s full key %q has no cache-class namespace", name, key)
			}
			if !strings.Contains(key, "ci-test") && !strings.Contains(key, "security-") {
				t.Errorf("%s full key %q has no lane namespace", name, key)
			}
		}
	}
	assertFullKeys("ci.yml", ci, "hashFiles('glade/go.sum')", 2)
	assertFullKeys("security.yml", security, "hashFiles('go.sum')", 6)

	for _, want := range []string{
		"key: ${{ steps.restore-go-mod-cache.outputs.cache-primary-key }}",
		"key: ${{ steps.restore-go-build-cache.outputs.cache-primary-key }}",
	} {
		if !strings.Contains(ci, want) {
			t.Errorf("ci.yml save missing matching restore primary key %q", want)
		}
	}
	for _, lane := range []string{"security-govulncheck", "security-codeql", "security-gosec"} {
		if got := strings.Count(security, lane); got != 2 {
			t.Errorf("security.yml lane %q occurrences = %d, want 2 full keys", lane, got)
		}
	}
	if got := strings.Count(security, "ci-test-${{ hashFiles('go.sum') }}-"); got != 6 {
		t.Errorf("security.yml digest-scoped ci-test restore prefixes = %d, want 6", got)
	}
	if got := strings.Count(security, "1.26.5-ci-test-"); got != 12 {
		t.Errorf("security.yml ci-test restore prefixes = %d, want 12", got)
	}

	for _, workflow := range []struct {
		name string
		text string
		want int
	}{
		{name: "ci.yml", text: ci, want: 2},
		{name: "security.yml", text: security, want: 6},
	} {
		if got := strings.Count(workflow.text, "cache-matched-key"); got != workflow.want {
			t.Errorf("%s restored-key reports = %d, want %d", workflow.name, got, workflow.want)
		}
		if got := strings.Count(workflow.text, " bytes=$bytes"); got != workflow.want {
			t.Errorf("%s byte-size reports = %d, want %d", workflow.name, got, workflow.want)
		}
	}

	if strings.Contains(ci, "cache-dependency-path: glade/go.sum") {
		t.Error("ci.yml retains setup-go cache-dependency-path")
	}
	if strings.Contains(security, "cache-dependency-path: go.sum") {
		t.Error("security.yml retains setup-go cache-dependency-path")
	}
	for _, want := range []string{
		"cache: npm",
		"glade/third_party/lwc/package-lock.json",
		"glade/site/package-lock.json",
	} {
		if !strings.Contains(ci, want) {
			t.Errorf("ci.yml npm cache block missing %q", want)
		}
	}
}

func TestSecurityWorkflowContract(t *testing.T) {
	workflowPath := filepath.Join("..", ".github", "workflows", "security.yml")
	workflow, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read %s: %v", workflowPath, err)
	}
	workflowText := string(workflow)
	if problem := securityWorkflowHeaderProblem(workflowText); problem != "" {
		t.Errorf("security.yml header contract: %s", problem)
	}
	jobs := workflowJobBlocks(t, workflowText)

	wantJobs := []string{"codeql", "dependency-review", "gosec", "govulncheck", "npm-audit", "scorecard"}
	gotJobs := make([]string, 0, len(jobs))
	for name := range jobs {
		gotJobs = append(gotJobs, name)
	}
	sort.Strings(gotJobs)
	if strings.Join(gotJobs, ",") != strings.Join(wantJobs, ",") {
		t.Fatalf("security.yml jobs = %v, want %v", gotJobs, wantJobs)
	}
	for _, jobName := range wantJobs {
		for _, marker := range []string{"runs-on: ubuntu-latest", "uses: actions/checkout@v6"} {
			if !strings.Contains(jobs[jobName], marker) {
				t.Errorf("%s job missing preserved marker %q", jobName, marker)
			}
		}
	}

	const gosecPin = "securego/gosec@9e6a9843d7a4a6e3e9a8539b02612c8a4aa3f889 # v2.27.1"
	if count := strings.Count(workflowText, gosecPin); count != 1 {
		t.Fatalf("security.yml exact gosec pin count = %d, want 1", count)
	}
	if strings.Contains(workflowText, "securego/gosec@v2") {
		t.Fatal("security.yml must not use the moving securego/gosec@v2 tag")
	}

	const scorecardPin = "ossf/scorecard-action@4eaacf0543bb3f2c246792bd56e8cdeffafb205a # v2.4.3"
	if count := strings.Count(workflowText, scorecardPin); count != 1 {
		t.Fatalf("security.yml exact Scorecard pin count = %d, want 1", count)
	}
	if strings.Contains(workflowText, "ossf/scorecard-action@v2") {
		t.Fatal("security.yml must not use the moving ossf/scorecard-action@v2 tag")
	}

	codeql := jobs["codeql"]
	for _, want := range []string{
		"timeout-minutes: 60",
		"languages: go",
		"config: |",
		"- uses: security-extended",
		"id: go/allocation-size-overflow",
		"build-mode: manual",
		`run: go build -o "${RUNNER_TEMP}/glade-codeql" ./cmd/glade`,
		"github/codeql-action/init@v4",
		"github/codeql-action/analyze@v4",
	} {
		if !strings.Contains(codeql, want) {
			t.Errorf("codeql job missing %q", want)
		}
	}
	if strings.Contains(codeql, "queries: +security-extended") {
		t.Error("codeql job must use inline config so the pathological allocation-size query can be filtered")
	}
	if strings.Contains(codeql, "github/codeql-action/autobuild@") {
		t.Error("codeql job must not use autobuild")
	}

	for _, jobName := range []string{"govulncheck", "codeql", "gosec"} {
		setupGo := workflowStepBlock(t, jobs[jobName], "uses: actions/setup-go@v6")
		if !strings.Contains(setupGo, "cache: false") {
			t.Errorf("%s setup-go step missing cache: false", jobName)
		}
		if strings.Contains(setupGo, "cache-dependency-path:") {
			t.Errorf("%s setup-go step retains cache-dependency-path", jobName)
		}
	}
	if count := strings.Count(workflowText, "uses: actions/setup-go@v6"); count != 3 {
		t.Errorf("security.yml setup-go step count = %d, want 3", count)
	}
	if strings.Contains(workflowText, "cache-dependency-path:") {
		t.Error("security.yml must not retain cache-dependency-path")
	}
	coverageMarkers := map[string][]string{
		"govulncheck":       {"go run golang.org/x/vuln/cmd/govulncheck@latest ./..."},
		"codeql":            {"actions: read", "contents: read", "security-events: write"},
		"gosec":             {"actions: read", "contents: read", "security-events: write", gosecPin, "args: -no-fail -fmt sarif -out gosec.sarif ./...", "github/codeql-action/upload-sarif@v4", "sarif_file: gosec.sarif"},
		"npm-audit":         {"actions/setup-node@v6", `node-version: "22"`, "npm audit --omit=dev --audit-level=high", "working-directory: third_party/lwc", "working-directory: contrib/vscode-glade"},
		"dependency-review": {"if: github.event_name == 'pull_request'", "contents: read", "actions/dependency-review-action@v5", "fail-on-severity: high"},
		"scorecard":         {"actions: read", "contents: read", "id-token: write", "security-events: write", scorecardPin, "results_file: scorecard.sarif", "results_format: sarif", "publish_results: true", "github/codeql-action/upload-sarif@v4", "sarif_file: scorecard.sarif"},
	}
	for jobName, markers := range coverageMarkers {
		for _, marker := range markers {
			if !strings.Contains(jobs[jobName], marker) {
				t.Errorf("%s job missing preserved marker %q", jobName, marker)
			}
		}
	}
}

func TestSecurityWorkflowHeaderRejectsNestedPermissionAndTriggerMutations(t *testing.T) {
	workflowPath := filepath.Join("..", ".github", "workflows", "security.yml")
	workflow, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read %s: %v", workflowPath, err)
	}
	workflowText := string(workflow)
	mutations := map[string]string{
		"job permission substitute": strings.Replace(workflowText, "\npermissions:\n  contents: read\n\njobs:\n", "\njobs:\n", 1),
		"changed trigger":           strings.Replace(workflowText, "  pull_request:\n", "  pull_request_target:\n", 1),
	}
	for name, mutated := range mutations {
		t.Run(name, func(t *testing.T) {
			if mutated == workflowText {
				t.Fatal("test fixture did not mutate workflow header")
			}
			if !strings.Contains(mutated, "contents: read") {
				t.Fatal("test mutation unexpectedly removed all job-level contents permissions")
			}
			if problem := securityWorkflowHeaderProblem(mutated); problem == "" {
				t.Fatal("header contract accepted invalid top-level workflow structure")
			}
		})
	}
}

func securityWorkflowHeaderProblem(workflow string) string {
	header, _, found := strings.Cut(workflow, "\njobs:\n")
	if !found {
		return "missing top-level jobs boundary"
	}
	blocks, problem := workflowTopLevelBlocks(header)
	if problem != "" {
		return problem
	}
	if len(blocks) != 3 {
		return fmt.Sprintf("top-level header block count = %d, want 3", len(blocks))
	}
	wantBlocks := map[string]string{
		"name": "name: Security",
		"on": `on:
  pull_request:
  push:
    branches:
      - main
  schedule:
    - cron: "17 9 * * 1"
  workflow_dispatch:`,
		"permissions": `permissions:
  contents: read`,
	}
	for name, want := range wantBlocks {
		got, ok := blocks[name]
		if !ok {
			return fmt.Sprintf("missing top-level %s block", name)
		}
		if got != want {
			return fmt.Sprintf("top-level %s block does not match required structure", name)
		}
	}
	return ""
}

func workflowTopLevelBlocks(header string) (map[string]string, string) {
	lines := strings.Split(header, "\n")
	blocks := make(map[string]string)
	for i := 0; i < len(lines); {
		if lines[i] == "" {
			i++
			continue
		}
		if strings.HasPrefix(lines[i], " ") {
			return nil, fmt.Sprintf("unexpected indented header line %q", lines[i])
		}
		name, _, ok := strings.Cut(lines[i], ":")
		if !ok || name == "" {
			return nil, fmt.Sprintf("invalid top-level header line %q", lines[i])
		}
		start := i
		i++
		for i < len(lines) && (lines[i] == "" || strings.HasPrefix(lines[i], " ")) {
			i++
		}
		if _, duplicate := blocks[name]; duplicate {
			return nil, fmt.Sprintf("duplicate top-level %s block", name)
		}
		blocks[name] = strings.TrimSpace(strings.Join(lines[start:i], "\n"))
	}
	return blocks, ""
}

func workflowJobBlocks(t *testing.T, workflow string) map[string]string {
	t.Helper()
	lines := strings.Split(workflow, "\n")
	jobsLine := -1
	for i, line := range lines {
		if line == "jobs:" {
			jobsLine = i
			break
		}
	}
	if jobsLine < 0 {
		t.Fatal("security.yml missing top-level jobs block")
	}

	blocks := make(map[string]string)
	for i := jobsLine + 1; i < len(lines); {
		line := lines[i]
		if line == "" || strings.HasPrefix(line, "  ") {
			if len(line) >= 3 && strings.HasPrefix(line, "  ") && line[2] != ' ' && strings.HasSuffix(line, ":") {
				name := strings.TrimSuffix(strings.TrimSpace(line), ":")
				start := i
				i++
				for i < len(lines) && (lines[i] == "" || strings.HasPrefix(lines[i], "    ")) {
					i++
				}
				blocks[name] = strings.Join(lines[start:i], "\n")
				continue
			}
			i++
			continue
		}
		break
	}
	return blocks
}

func workflowStepBlock(t *testing.T, jobBlock, marker string) string {
	t.Helper()
	lines := strings.Split(jobBlock, "\n")
	for i, line := range lines {
		if !strings.Contains(line, marker) {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		start := i
		for start > 0 {
			previous := lines[start-1]
			previousIndent := len(previous) - len(strings.TrimLeft(previous, " "))
			if strings.HasPrefix(strings.TrimSpace(previous), "- ") && previousIndent == indent {
				start--
				break
			}
			if previousIndent < indent {
				break
			}
			start--
		}
		end := i + 1
		for end < len(lines) {
			next := lines[end]
			nextIndent := len(next) - len(strings.TrimLeft(next, " "))
			if strings.HasPrefix(strings.TrimSpace(next), "- ") && nextIndent <= indent {
				break
			}
			end++
		}
		return strings.Join(lines[start:end], "\n")
	}
	t.Fatalf("job block missing step marker %q", marker)
	return ""
}

func TestCIGoTestWrapperIsWired(t *testing.T) {
	script, err := os.ReadFile("ci-go-test.sh")
	if err != nil {
		t.Fatalf("read ci-go-test.sh: %v", err)
	}
	scriptText := string(script)
	for _, want := range []string{
		`export GOMAXPROCS="${GOMAXPROCS:-2}"`,
		"run_with_heartbeat",
		"run_core_tests",
		"run_full_tests",
		"run_apextest_matrix_shard",
		"heartbeat_pid",
		`kill -0 "${pid}"`,
		"./internal/apextest",
		`go test -list '^Test' ./internal/apextest`,
		`go test -json -timeout=30m -run "${regex}" ./internal/apextest`,
		"CI_SHARD_PLANNER",
		"validation-summary.json",
		"./internal/gladecli",
		"./internal/sema",
		"go test -v",
	} {
		if !strings.Contains(scriptText, want) {
			t.Fatalf("ci-go-test.sh missing %q", want)
		}
	}
	if strings.Contains(scriptText, `grep '^Test' || true`) {
		t.Fatal("ci-go-test.sh must not suppress Apex test discovery failures")
	}
	for _, forbidden := range []string{"apextest.test", "go test -c", "-test.run", "-test.list"} {
		if strings.Contains(scriptText, forbidden) {
			t.Fatalf("ci-go-test.sh retains forbidden direct-binary path %q", forbidden)
		}
	}

	workflowPath := filepath.Join("..", ".github", "workflows", "ci.yml")
	workflow, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read %s: %v", workflowPath, err)
	}
	workflowText := string(workflow)
	for _, want := range []string{
		"timeout-minutes: 30",
		"GOMAXPROCS: \"2\"",
		"go-version: \"1.26.5\"",
		"actions/checkout@v6",
		"client-id: ${{ vars.GLADE_APP_CLIENT_ID }}",
		"actions/setup-go@v6",
		"actions/setup-node@v6",
		"scripts/ci-go-test.sh core",
		"apextest:",
		"matrix:",
		"shard: [0, 1]",
		"cache: false",
		"ci-apextest-${{ matrix.shard }}",
		"go-mod-v1-1.26.5-${{ runner.os }}-${{ runner.arch }}-${{ hashFiles('go.sum') }}-ci-apextest-${{ matrix.shard }}-${{ github.sha }}-${{ github.run_id }}-${{ github.run_attempt }}",
		"go-build-v1-1.26.5-${{ runner.os }}-${{ runner.arch }}-${{ hashFiles('go.sum') }}-ci-apextest-${{ matrix.shard }}-${{ github.sha }}-${{ github.run_id }}-${{ github.run_attempt }}",
		"scripts/ci-go-test.sh apex-shard \"${{ matrix.shard }}\"",
		"if: always()",
		"actions/upload-artifact@v6",
		"CI_APEXTEST_ARTIFACT_DIR: ci-artifacts/apextest-${{ matrix.shard }}",
		"ci-artifacts/apextest-${{ matrix.shard }}/discovery-stderr.txt",
		"ci-artifacts/apextest-${{ matrix.shard }}/discovery-command.txt",
	} {
		if !strings.Contains(workflowText, want) {
			t.Fatalf("ci.yml missing %q", want)
		}
	}
	if strings.Contains(workflowText, "app-id: ${{ vars.GLADE_APP_CLIENT_ID }}") {
		t.Fatalf("ci.yml should use create-github-app-token client-id, not deprecated app-id")
	}
	if strings.Contains(workflowText, "scripts/ci-go-test.sh race") {
		t.Fatalf("ci.yml should not run the full race suite on GitHub-hosted runners")
	}
	if strings.Count(workflowText, "go test -json") != 0 {
		t.Fatal("ci.yml must delegate the native Apex run to the tested wrapper")
	}
	if strings.Count(workflowText, "shard: [0, 1]") != 1 {
		t.Fatal("ci.yml must define exactly one two-cell Apex matrix")
	}
	if strings.Contains(workflowText, ".ci-artifacts") || strings.Contains(scriptText, ".ci-artifacts") {
		t.Fatal("Apex evidence must use a non-hidden path so upload-artifact includes it by default")
	}
}

func TestCIGoTestModesPreserveFullDefaultAndCoreExcludesApex(t *testing.T) {
	for _, tc := range []struct {
		name     string
		args     []string
		wantApex bool
	}{
		{name: "default", wantApex: true},
		{name: "test", args: []string{"test"}, wantApex: true},
		{name: "core", args: []string{"core"}, wantApex: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			calls := filepath.Join(dir, "calls")
			fakeGo := filepath.Join(dir, "go")
			script := `#!/usr/bin/env bash
printf '%s\n' "$*" >> "$FIXTURE_CALLS"
if [[ "$1" == "list" ]]; then
  printf '%s\n' github.com/glade-sh/glade/internal/apextest github.com/glade-sh/glade/internal/other
fi
`
			if err := os.WriteFile(fakeGo, []byte(script), 0o700); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("bash", append([]string{"ci-go-test.sh"}, tc.args...)...)
			cmd.Env = append(os.Environ(), "PATH="+dir+":"+os.Getenv("PATH"), "FIXTURE_CALLS="+calls, "CI_GO_TEST_HEARTBEAT_SECONDS=1")
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("mode failed: %v\n%s", err, out)
			}
			b, err := os.ReadFile(calls)
			if err != nil {
				t.Fatal(err)
			}
			gotApex := strings.Contains(string(b), "test -timeout=30m ./internal/apextest")
			if gotApex != tc.wantApex {
				t.Fatalf("Apex invocation = %v, want %v; calls:\n%s", gotApex, tc.wantApex, b)
			}
		})
	}
}
