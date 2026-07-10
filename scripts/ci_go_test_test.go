package scripts

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

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
		"queries: +security-extended",
		"build-mode: manual",
		`run: go build -o "${RUNNER_TEMP}/glade-codeql" ./cmd/glade`,
		"github/codeql-action/init@v4",
		"github/codeql-action/analyze@v4",
	} {
		if !strings.Contains(codeql, want) {
			t.Errorf("codeql job missing %q", want)
		}
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
	if strings.Contains(workflowText, "actions/cache@") {
		t.Error("security.yml must not add actions/cache")
	}

	coverageMarkers := map[string][]string{
		"govulncheck":       {"go run golang.org/x/vuln/cmd/govulncheck@latest ./..."},
		"codeql":            {"actions: read", "contents: read", "security-events: write"},
		"gosec":             {"contents: read", "security-events: write", gosecPin, "args: -no-fail -fmt sarif -out gosec.sarif ./...", "github/codeql-action/upload-sarif@v4", "sarif_file: gosec.sarif"},
		"npm-audit":         {"actions/setup-node@v6", `node-version: "22"`, "npm audit --omit=dev --audit-level=high", "working-directory: third_party/lwc", "working-directory: contrib/vscode-glade"},
		"dependency-review": {"if: github.event_name == 'pull_request'", "contents: read", "actions/dependency-review-action@v5", "fail-on-severity: high"},
		"scorecard":         {"contents: read", "id-token: write", "security-events: write", scorecardPin, "results_file: scorecard.sarif", "results_format: sarif", "publish_results: true", "github/codeql-action/upload-sarif@v4", "sarif_file: scorecard.sarif"},
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
		"CI_APEXTEST_SHARD_SIZE",
		"run_with_heartbeat",
		"run_apextest_shards",
		"join_test_pattern",
		"heartbeat_pid",
		`kill -0 "${pid}"`,
		"./internal/apextest",
		"apextest.test",
		"go test \"${compile_args[@]}\"",
		"./internal/gladecli",
		"./internal/sema",
		"go test -v",
	} {
		if !strings.Contains(scriptText, want) {
			t.Fatalf("ci-go-test.sh missing %q", want)
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
		"go-version: \"1.26.4\"",
		"actions/checkout@v6",
		"client-id: ${{ vars.GLADE_APP_CLIENT_ID }}",
		"actions/setup-go@v6",
		"actions/setup-node@v6",
		"scripts/ci-go-test.sh test",
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
}
