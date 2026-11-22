package scripts

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCIGuardMacDeferral(t *testing.T) {
	const pkg = "github.com/glade-sh/glade/scripts"
	const test = "TestCILocalReleaseLaneMetricsIncludeApexShards"
	for _, tc := range []struct {
		name, reason, identity, runner string
		ok                             bool
	}{
		{"captured", "local-release resource metrics use macOS /usr/bin/time -l", test, "Linux", true},
		{"different reason", "unrelated", test, "Linux", false},
		{"different test", "local-release resource metrics use macOS /usr/bin/time -l", "TestOther", "Linux", false},
		{"mac runner", "local-release resource metrics use macOS /usr/bin/time -l", test, "macOS", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			events := filepath.Join(dir, "events")
			meta := filepath.Join(dir, "metadata")
			summary := filepath.Join(dir, "summary")
			rows := []map[string]string{{"Action": "output", "Package": pkg, "Test": tc.identity, "Output": "    ci_local_release_lane_test.go:240: " + tc.reason + "\n"}, {"Action": "skip", "Package": pkg, "Test": tc.identity}, {"Action": "pass", "Package": pkg}}
			var text strings.Builder
			for _, row := range rows {
				b, _ := json.Marshal(row)
				text.Write(b)
				text.WriteByte('\n')
			}
			os.WriteFile(events, []byte(text.String()), 0600)
			os.WriteFile(meta, []byte(pkg+"\thas-tests\n"), 0600)
			cmd := exec.Command("bash", "ci-go-test.sh", "package-summary", "remaining-go", "fixture", events, summary, meta, pkg)
			cmd.Env = append(os.Environ(), "RUNNER_OS="+tc.runner)
			out, err := cmd.CombinedOutput()
			if (err == nil) != tc.ok {
				t.Fatalf("err=%v out=%s", err, out)
			}
			if tc.ok {
				b, err := os.ReadFile(summary)
				if err != nil || !strings.Contains(string(b), `"test": "`+test+`"`) {
					t.Fatalf("deferred evidence %s %v", b, err)
				}
			}
		})
	}
}

func TestCIGuardWorkflowUsesAuditedExecution(t *testing.T) {
	_, jobs := readCIWorkflow(t)
	for _, name := range []string{"early-guards", "pr-guards"} {
		if !strings.Contains(jobs[name], "ci-pr-packages.sh") {
			t.Errorf("%s does not audit selected guard terminals", name)
		}
	}
}

func TestCIGuardSelectedDiscovery(t *testing.T) {
	for _, empty := range []bool{false, true} {
		t.Run(map[bool]string{false: "selected", true: "empty"}[empty], func(t *testing.T) {
			dir := t.TempDir()
			program := `#!/bin/bash
printf '%s\n' "$*" >>"$CALLS"
if [[ $1 == list ]]; then printf 'github.com/glade-sh/glade/internal/example\thas-tests\n'; exit; fi
if [[ $* == *-list* ]]; then
  [[ "$EMPTY" == true ]] || printf 'TestExample\n'
  printf 'ok  \tgithub.com/glade-sh/glade/internal/example\t0.001s\n'; exit
fi
printf '%s\n' '{"Action":"pass","Package":"github.com/glade-sh/glade/internal/example","Test":"TestExample"}' '{"Action":"pass","Package":"github.com/glade-sh/glade/internal/example"}'
`
			if err := os.WriteFile(filepath.Join(dir, "go"), []byte(program), 0700); err != nil {
				t.Fatal(err)
			}
			calls := filepath.Join(dir, "calls")
			cmd := exec.Command("bash", "scripts/ci-pr-packages.sh", filepath.Join(dir, "evidence"), "github.com/glade-sh/glade/internal/example")
			cmd.Dir = ".."
			cmd.Env = append(os.Environ(), "PATH="+dir+":"+os.Getenv("PATH"), "CALLS="+calls, "CI_TEST_RUN_REGEX=^TestExample$", "EMPTY="+map[bool]string{false: "false", true: "true"}[empty])
			out, err := cmd.CombinedOutput()
			if (err == nil) == empty {
				t.Fatalf("selection err=%v output=%s", err, out)
			}
			data, err := os.ReadFile(calls)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(data), "-list ^TestExample$") || (!empty && !strings.Contains(string(data), "-run ^TestExample$")) {
				t.Fatalf("selection lost: %s", data)
			}
		})
	}
}

func TestCIGuardRelativePackageDiscovery(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command("bash", "scripts/ci-pr-packages.sh", dir, "./scripts")
	cmd.Dir = ".."
	cmd.Env = append(os.Environ(), "CI_TEST_RUN_REGEX=^TestCIWorkerClassification$")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("relative package wrapper: %v\n%s", err, out)
	}
	data, err := os.ReadFile(filepath.Join(dir, "discovery.json"))
	if err != nil {
		t.Fatal(err)
	}
	var discovery map[string][]string
	if err := json.Unmarshal(data, &discovery); err != nil {
		t.Fatal(err)
	}
	const pkg = "github.com/glade-sh/glade/scripts"
	if len(discovery) != 1 || len(discovery[pkg]) != 1 || discovery[pkg][0] != "TestCIWorkerClassification" {
		t.Fatalf("unexpected canonical discovery: %s", data)
	}
	if err := os.WriteFile(filepath.Join(dir, "events.json"), []byte(`{"Action":"pass","Package":"example.invalid/scripts","Test":"TestCIWorkerClassification"}`+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("python3", "scripts/ci-pr-package-discovery.py", "validate", dir, "./scripts")
	cmd.Dir = ".."
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("accepted terminal for another package: %s", out)
	}
}
