package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// The test job used to run the repository guards beside every remaining Go
// package in one row. It is now three matrix rows: the guards, and two parts
// of the remaining-go lane. Each row runs one lane command.
func TestCIRemainingGoRowsRunOneLaneEach(t *testing.T) {
	_, jobs := readCIWorkflow(t)
	for _, row := range []struct{ step, lane, command string }{
		{"Run repository guards", "repoguard", "scripts/ci-go-test.sh lane repoguard"},
		{"Run remaining Go packages (part 1)", "remaining-go", "scripts/ci-go-test.sh lane remaining-go-1"},
		{"Run remaining Go packages (part 2)", "remaining-go-2", "scripts/ci-go-test.sh lane remaining-go-2"},
	} {
		step := workflowStepBlockText(jobs["test"], "      - name: "+row.step)
		if !strings.Contains(step, "if: matrix.lane == '"+row.lane+"'") || strings.Count(step, row.command) != 1 {
			t.Errorf("%s must run %q only in the %s row", row.step, row.command, row.lane)
		}
		if strings.Count(jobs["test"], "          - lane: "+row.lane+"\n") != 1 {
			t.Errorf("test matrix must contain exactly one %s row", row.lane)
		}
	}
}

// Part 1 and part 2 of the remaining-go lane must together run every
// package the lane owns exactly once, and each part must write its own raw
// event artifact.
func TestCIRemainingGoPartsRunEveryPackageOnce(t *testing.T) {
	rows := "remaining-go\t./internal/soql\nremaining-go\t./scripts\nremaining-go\t./internal/testutil/toolchaininstall\nremaining-go\t./internal/visualforce\nremaining-go\t./internal/lwc/compile\nremaining-go\t./internal/vm\nremaining-go\t./internal/newpackage"
	run := func(part string) (string, string) {
		dir := t.TempDir()
		writeApexFixtureExecutable(t, filepath.Join(dir, "go"), "#!/usr/bin/env bash\nprintf '%s\\n' \"$*\" >> \"$FIXTURE_CALLS\"\nprintf '%s\\n' '{\"Action\":\"pass\",\"Package\":\"fixture\"}'\n")
		writeApexFixtureExecutable(t, filepath.Join(dir, "testlog"), "#!/usr/bin/env bash\ncat >/dev/null\n")
		cmd := exec.Command("bash", "-c", `source ./ci-go-test.sh; package_lane_rows=$'`+strings.ReplaceAll(rows, "\n", `\n`)+`'; run_package_lane remaining-go test 20m '' ci-remaining `+part)
		cmd.Env = append(os.Environ(), "PATH="+dir+":"+os.Getenv("PATH"),
			"CI_TESTLOG_RENDERER="+filepath.Join(dir, "testlog"), "CI_GO_TEST_ARTIFACT_DIR="+dir,
			"FIXTURE_CALLS="+filepath.Join(dir, "calls"))
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("part %q failed: %v\n%s", part, err, out)
		}
		calls, _ := os.ReadFile(filepath.Join(dir, "calls"))
		artifact := "test-remaining-go.json"
		if part == "2" {
			artifact = "test-remaining-go-2.json"
		}
		if data, err := os.ReadFile(filepath.Join(dir, artifact)); err != nil || len(data) == 0 {
			t.Fatalf("part %q raw events %s missing: %v", part, artifact, err)
		}
		return string(calls), dir
	}
	var seen []string
	for _, part := range []string{"1", "2"} {
		calls, _ := run(part)
		for _, field := range strings.Fields(calls) {
			if strings.HasPrefix(field, "./") {
				seen = append(seen, field)
			}
		}
	}
	sort.Strings(seen)
	want := []string{"./internal/newpackage", "./internal/soql", "./internal/testutil/toolchaininstall", "./scripts"}
	if strings.Join(seen, " ") != strings.Join(want, " ") {
		t.Fatalf("parts ran %v, want each routed package once: %v", seen, want)
	}
}
