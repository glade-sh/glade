package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
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

	ci := readWorkflow("ci.yml")
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
				"cache-v1", "1.26.4", "runner.os", "runner.arch", sumExpression,
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
	if got := strings.Count(security, "1.26.4-ci-test-"); got != 12 {
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
		"go-version: \"1.26.4\"",
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
		"go-mod-v1-1.26.4-${{ runner.os }}-${{ runner.arch }}-${{ hashFiles('go.sum') }}-ci-apextest-${{ matrix.shard }}-${{ github.sha }}-${{ github.run_id }}-${{ github.run_attempt }}",
		"go-build-v1-1.26.4-${{ runner.os }}-${{ runner.arch }}-${{ hashFiles('go.sum') }}-ci-apextest-${{ matrix.shard }}-${{ github.sha }}-${{ github.run_id }}-${{ github.run_attempt }}",
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
