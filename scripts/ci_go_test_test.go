package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

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
