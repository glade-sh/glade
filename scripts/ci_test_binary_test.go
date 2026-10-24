package scripts

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// This owned module checks discovery, working directory, subtests and exit status
// through both Go's normal runner and the shared binary runner.
func TestCISharedBinaryPreservesExecution(t *testing.T) {
	helper, err := filepath.Abs("ci-test-binary.py")
	if err != nil {
		t.Fatal(err)
	}
	pythonOutput, err := exec.Command("python3", "-c", "import sys; print(sys.executable)").Output()
	if err != nil {
		t.Fatal(err)
	}
	python := strings.TrimSpace(string(pythonOutput))
	root := t.TempDir()
	files := map[string]string{
		"go.mod":   "module example.test/check\n\ngo 1.26\n",
		"rows.txt": "alpha\nbeta\ngamma\n",
		"rows_test.go": `package check
import("os";"strings";"testing")
func TestMain(m *testing.M) { os.Exit(m.Run()) }
func TestRows(t *testing.T) { b,e:=os.ReadFile("rows.txt"); if e!=nil {t.Fatal(e)}; for _,r:= range strings.Fields(string(b)) { t.Run(r,func(t *testing.T){if len(r)<4 {t.Fatal(r)}}) } }
func TestFailure(t *testing.T) { if os.Getenv("FIXTURE_FAIL")=="1" {t.Fatal("owned failure")} }
`,
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	run := func(env []string, name string, args ...string) ([]byte, error, time.Duration) {
		t.Helper()
		cmd := exec.Command(name, args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), env...)
		start := time.Now()
		out, err := cmd.CombinedOutput()
		return out, err, time.Since(start)
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "."}, {"-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit", "-qm", "Owned fixture"}} {
		if out, err, _ := run(nil, "git", args...); err != nil {
			t.Fatalf("git: %v %s", err, out)
		}
	}
	binaries := filepath.Join(root, "binaries")
	if out, err, _ := run(nil, python, helper, "build", binaries, "test", "."); err != nil {
		t.Fatalf("build shared binary: %v\n%s", err, out)
	}
	terminal := func(raw []byte) []string {
		t.Helper()
		var result []string
		for _, line := range strings.Split(string(raw), "\n") {
			var e struct{ Action, Test string }
			if json.Unmarshal([]byte(line), &e) == nil && (e.Action == "pass" || e.Action == "fail" || e.Action == "skip") {
				result = append(result, e.Action+":"+e.Test)
			}
		}
		return result
	}
	for _, failure := range []string{"0", "1"} {
		env := []string{"FIXTURE_FAIL=" + failure}
		before, beforeErr, beforeTime := run(env, realGoCommand(t), "test", "-json", "-vet=off", "-trimpath", "-count=1", "-timeout=30s", ".")
		after, afterErr, afterTime := run(append(env, "CI_TEST_BIN_DIR="+binaries), python, helper, "test", "-json", "-vet=off", "-trimpath", "-count=1", "-timeout=30s", ".")
		if (beforeErr == nil) != (afterErr == nil) || !reflect.DeepEqual(terminal(before), terminal(after)) {
			t.Fatalf("execution changed: before=%v after=%v\n%s\n%s", beforeErr, afterErr, before, after)
		}
		if len(terminal(after)) != 6 {
			t.Fatalf("want three rows, two parents and package terminal: %s", after)
		}
		t.Logf("failure=%s: same 6 terminal assertions; go test %.3fs, shared binary %.3fs", failure, beforeTime.Seconds(), afterTime.Seconds())
	}
	before, err, _ := run(nil, realGoCommand(t), "test", "-list", "^Test", ".")
	if err != nil {
		t.Fatal(err)
	}
	after, err, _ := run([]string{"CI_TEST_BIN_DIR=" + binaries}, python, helper, "test", "-list", "^Test", ".")
	if err != nil {
		t.Fatalf("discovery: %v %s", err, after)
	}
	names := func(b []byte) []string {
		var n []string
		for _, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(line, "Test") {
				n = append(n, line)
			}
		}
		return n
	}
	if !reflect.DeepEqual(names(before), names(after)) {
		t.Fatalf("discovery changed: %s / %s", before, after)
	}
	// Exercise the actual shell wrapper and its discovery/result validators too.
	planner := filepath.Join(root, "cishard")
	if out, err := exec.Command(realGoCommand(t), "build", "-o", planner, "./internal/cishard").CombinedOutput(); err != nil {
		t.Fatalf("planner build: %v %s", err, out)
	}
	wrapper, err := filepath.Abs("ci-go-test.sh")
	if err != nil {
		t.Fatal(err)
	}
	renderer := filepath.Join(root, "renderer")
	if err := os.WriteFile(renderer, []byte("#!/bin/sh\ncat >/dev/null\n"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, index := range []string{"0", "1"} {
		artifacts := filepath.Join(root, "shard-"+index)
		env := []string{"CI_TEST_BIN_DIR=" + binaries, "CI_SHARD_PLANNER=" + planner, "CI_TESTLOG_RENDERER=" + renderer, "FIXTURE_FAIL=0"}
		out, err, _ := run(env, "bash", "-c", `source "$1"; run_test_matrix_shard "$2" example.test/check . fixture fixture "$3" "" 1 test 2`, "fixture", wrapper, index, artifacts)
		if err != nil {
			t.Fatalf("shared matrix wrapper: %v %s", err, out)
		}
		raw, err := os.ReadFile(filepath.Join(artifacts, "validation-summary.json"))
		if err != nil {
			t.Fatal(err)
		}
		var result struct {
			Valid  bool
			Passed []string
		}
		if err := json.Unmarshal(raw, &result); err != nil || !result.Valid || len(result.Passed) != 1 {
			t.Fatalf("matrix validation: %v %s", err, raw)
		}
	}
	raceBinaries := filepath.Join(root, "race-binaries")
	if out, err, _ := run(nil, python, helper, "build", raceBinaries, "race", "."); err != nil {
		t.Fatalf("race build: %v %s", err, out)
	}
	raceEvents, err, _ := run([]string{"CI_TEST_BIN_DIR=" + raceBinaries}, python, helper, "test", "-json", "-race", "-count=1", ".")
	if err != nil || len(terminal(raceEvents)) != 6 {
		t.Fatalf("race coverage: %v %s", err, raceEvents)
	}
	copied := filepath.Join(root, "copied.test")
	if out, err, _ := run([]string{"CI_TEST_BIN_DIR=" + raceBinaries}, python, helper, "copy-race", ".", copied); err != nil {
		t.Fatalf("race copy: %v %s", err, out)
	}
	if out, err, _ := run(nil, copied, "-test.run=TestRows"); err != nil {
		t.Fatalf("copied race binary: %v %s", err, out)
	}
	for _, mutation := range []string{"binary", "revision", "race"} {
		t.Run(mutation, func(t *testing.T) {
			isolated := filepath.Join(root, mutation)
			if out, err, _ := run(nil, python, helper, "build", isolated, "test", "."); err != nil {
				t.Fatalf("build: %v %s", err, out)
			}
			switch mutation {
			case "binary":
				entries, _ := filepath.Glob(filepath.Join(isolated, "*.test"))
				if len(entries) != 1 {
					t.Fatal(entries)
				}
				if err := os.WriteFile(entries[0], []byte("corrupt"), 0700); err != nil {
					t.Fatal(err)
				}
			case "revision":
				if out, err, _ := run(nil, "git", "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit", "--allow-empty", "-qm", "Next fixture"); err != nil {
					t.Fatalf("commit: %v %s", err, out)
				}
			}
			args := []string{helper, "test", "-json", "-count=1"}
			if mutation == "race" {
				args = append(args, "-race")
			}
			args = append(args, ".")
			if out, err, _ := run([]string{"CI_TEST_BIN_DIR=" + isolated}, python, args...); err == nil {
				t.Fatalf("accepted %s mismatch: %s", mutation, out)
			}
		})
	}
}

func TestCISharedBinaryWorkflowWiring(t *testing.T) {
	_, jobs := readCIWorkflow(t)
	if !strings.Contains(jobs["test-binaries"], "ci-test-binary.py build") {
		t.Fatal("CI must build shared test binaries once")
	}
	helper, err := os.ReadFile("ci-test-binary.py")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(helper), `command = [GO, "test", "-trimpath", "-vet=off", "-c"`) {
		t.Fatal("shared test binaries must be built with -trimpath")
	}
	for _, name := range []string{"test", "sema", "apextest"} {
		if !strings.Contains(jobs[name], "test-binaries") || !strings.Contains(jobs[name], "CI_TEST_BIN_DIR") {
			t.Errorf("%s does not consume the shared binaries", name)
		}
	}
	if !strings.Contains(jobs["required-ci"], "      - test-binaries\n") {
		t.Fatal("shared build must remain required")
	}
}

func TestCISharedBinaryRemainingLaneKeepsOrdinaryGoExecution(t *testing.T) {
	_, jobs := readCIWorkflow(t)
	// This lane executes many packages and downloads no package binary artifact.
	if !strings.Contains(jobs["test"], `CI_TEST_BIN_DIR: ${{ matrix.lane != 'remaining-go' && '/tmp/glade-test-binaries' || '' }}`) {
		t.Fatal("remaining-go has no shared artifact and must keep the ordinary multi-package Go runner")
	}
}
