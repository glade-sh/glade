package scripts

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestCIScaleApexWorkflowRoutes(t *testing.T) {
	_, jobs := readCIWorkflow(t)
	for _, marker := range []string{"shard: [0, 1, 2, 3, 4, 5]", "CI_APEXTEST_SHARDS: \"6\"", "timeout-minutes: 100", "fail-fast: false"} {
		if !strings.Contains(jobs["apextest"], marker) {
			t.Errorf("Apex matrix missing %q", marker)
		}
	}
	history := jobs["apextest-history"]
	for index := range 6 {
		if !strings.Contains(history, fmt.Sprintf("/tmp/apextest-history-input/apex-shard-%d", index)) {
			t.Errorf("Apex history omits shard %d", index)
		}
	}
	for _, name := range []string{"apextest", "apextest-history"} {
		if !strings.Contains(jobs["required-ci"], "      - "+name+"\n") {
			t.Errorf("Required CI omits %s", name)
		}
	}
}

func TestCIScaleApexSixShardDispatcherRoute(t *testing.T) {
	t.Setenv("CI_APEXTEST_SHARDS", "6")
	var shards []map[string]any
	var discovery strings.Builder
	for index := range 6 {
		name := fmt.Sprintf("TestScale%d", index)
		fmt.Fprintln(&discovery, name)
		shards = append(shards, map[string]any{"index": index, "tests": []string{name}, "estimatedDurationMillis": 0, "regex": "^(?:" + name + ")$"})
	}
	plan, err := json.Marshal(map[string]any{"version": 1, "package": fixturePackage, "historyUsed": false, "shards": shards})
	if err != nil {
		t.Fatal(err)
	}
	for index := range 6 {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			out, err, artifacts := runApexShardFixture(t, fmt.Sprint(index), discovery.String(), 0, string(plan), 0, passEvent(fmt.Sprintf("TestScale%d", index)), 0)
			if err != nil {
				t.Fatalf("six-shard dispatcher failed: %v\n%s", err, out)
			}
			calls, err := os.ReadFile(filepath.Join(filepath.Dir(artifacts), "calls"))
			if err != nil || !strings.Contains(string(calls), "-count=1 -timeout=90m") {
				t.Fatalf("CI route must run uncached with the larger budget: %v %s", err, calls)
			}
		})
	}
}

func TestCIScaleApexLocalReleaseRouteIgnoresCIShardCount(t *testing.T) {
	for index, name := range []string{"TestAlpha", "TestBeta"} {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			out, err, calls, artifacts := runLocalReleaseLaneFixture(t, []string{"local-release-lane", fmt.Sprintf("apex-%d", index)}, localReleaseLaneFixture{
				events: passEvent(name), env: []string{"CI_APEXTEST_SHARDS=6"},
			})
			if err != nil || !strings.Contains(calls, "-count=1 -timeout=90m") {
				t.Fatalf("local-release route must retain its two-shard plan and budget: %v %s\n%s", err, calls, out)
			}
			data, err := os.ReadFile(filepath.Join(artifacts, fmt.Sprintf("apex-%d", index), "selected-shard.json"))
			var selected struct{ Tests []string }
			if err != nil || json.Unmarshal(data, &selected) != nil || len(selected.Tests) != 1 || selected.Tests[0] != name {
				t.Fatalf("local release selected the wrong partition: %v %s", err, data)
			}
		})
	}
}

// Exercise the real dispatcher, discovery, planner, test execution, validators
// and history collector on an owned project through both execution routes.
func TestCIScaleApexSourceAndSharedBinaryProjectRoutes(t *testing.T) {
	t.Setenv("GITHUB_EVENT_NAME", "pull_request")
	root := t.TempDir()
	pkgDir := filepath.Join(root, "internal", "apextest")
	if err := os.MkdirAll(pkgDir, 0700); err != nil {
		t.Fatal(err)
	}
	write := func(path, body string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(root, "go.mod"), "module github.com/glade-sh/glade\n\ngo 1.26\n")
	write(filepath.Join(pkgDir, "rows.txt"), "owned project row\n")
	var source strings.Builder
	source.WriteString(`package apextest
import("flag"; "os"; "testing")
func check(t *testing.T) {
 if flag.Lookup("test.timeout").Value.String() != "1h30m0s" { t.Fatal("Apex route must receive a 90m timeout") }
 if b,e:=os.ReadFile("rows.txt"); e!=nil || string(b)!="owned project row\n" {t.Fatalf("project working directory: %s %v",b,e)}
}
`)
	for index := range 12 {
		fmt.Fprintf(&source, "func TestScale%02d(t *testing.T) { check(t) }\n", index)
	}
	write(filepath.Join(pkgDir, "rows_test.go"), source.String())
	goCommand := realGoCommand(t)
	run := func(name string, args ...string) {
		t.Helper()
		cmd := exec.Command(name, args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s: %v\n%s", name, err, out)
		}
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "."}, {"-c", "user.name=Matt Simonis", "-c", "user.email=720686+mattsimonis@users.noreply.github.com", "commit", "-qm", "Add owned CI fixture\n\nCo-Authored-By: Codex <noreply@openai.com>"}} {
		run("git", args...)
	}
	planner := filepath.Join(root, "cishard")
	if out, err := exec.Command(goCommand, "build", "-o", planner, "./internal/cishard").CombinedOutput(); err != nil {
		t.Fatalf("build planner: %v\n%s", err, out)
	}
	adapter, _ := filepath.Abs("ci-test-binary.py")
	wrapper, _ := filepath.Abs("ci-go-test.sh")
	binaries := filepath.Join(root, "binaries")
	run("python3", adapter, "build", binaries, "test", "./internal/apextest")
	renderer := filepath.Join(root, "renderer")
	writeApexFixtureExecutable(t, renderer, "#!/bin/sh\ncat >/dev/null\n")
	for _, route := range []string{"source", "shared-binary"} {
		t.Run(route, func(t *testing.T) {
			var dirs []string
			for index := range 6 {
				dir := filepath.Join(root, route, fmt.Sprint(index))
				dirs = append(dirs, dir)
				cmd := exec.Command("bash", wrapper, "apex-shard", fmt.Sprint(index))
				cmd.Dir = root
				binaryDir := ""
				if route == "shared-binary" {
					binaryDir = binaries
				}
				cmd.Env = append(os.Environ(), "CI_TEST_BIN_DIR="+binaryDir, "CI_SHARD_PLANNER="+planner,
					"CI_TESTLOG_RENDERER="+renderer, "CI_APEXTEST_ARTIFACT_DIR="+dir, "CI_APEXTEST_HISTORY_PATH=", "CI_APEXTEST_SHARDS=6")
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("Apex %s shard %d: %v\n%s", route, index, err, out)
				}
			}
			output := filepath.Join(root, route, "history.json")
			cmd := exec.Command("bash", append([]string{wrapper, "apex-history-refresh", output}, dirs...)...)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("six-shard history: %v\n%s", err, out)
			}
			var history struct {
				Complete bool
				Tests    []struct{ Name string }
			}
			data, err := os.ReadFile(output)
			if err != nil || json.Unmarshal(data, &history) != nil || !history.Complete || len(history.Tests) != 12 {
				t.Fatalf("history must cover the exact project union: %v %s", err, data)
			}
			plan, err := exec.Command(planner, "--package", fixturePackage, "--shards", "6",
				"--tests", filepath.Join(dirs[0], "discovery.txt"), "--history", output).Output()
			var replanned struct{ HistoryUsed bool }
			if err != nil || json.Unmarshal(plan, &replanned) != nil || !replanned.HistoryUsed {
				t.Fatalf("six-shard planner must consume refreshed history: %v %s", err, plan)
			}
			assertApexProjectHistoryRebalances(t, planner, output, dirs)
			if err := os.Remove(filepath.Join(dirs[5], "events.json")); err != nil {
				t.Fatal(err)
			}
			cmd = exec.Command("bash", append([]string{wrapper, "apex-history-refresh", output}, dirs...)...)
			if out, err := cmd.CombinedOutput(); err == nil || !strings.Contains(string(out), "duration history refresh rejected") {
				t.Fatalf("history must reject missing evidence from the last shard: %v %s", err, out)
			}
			if _, err := os.Stat(output); !os.IsNotExist(err) {
				t.Fatalf("failed six-shard refresh left stale output: %v", err)
			}
		})
	}
}

func TestCIScaleApexLocalHelperRetainsTwoShardRoute(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0700); err != nil {
		t.Fatal(err)
	}
	wrapper, err := filepath.Abs("ci-go-test.sh")
	if err != nil {
		t.Fatal(err)
	}
	helper, err := os.ReadFile("apextest-shards.sh")
	if err != nil {
		t.Fatal(err)
	}
	writeApexFixtureExecutable(t, filepath.Join(root, "scripts/apextest-shards.sh"), string(helper))
	writeApexFixtureExecutable(t, filepath.Join(root, "scripts/ci-go-test.sh"), fmt.Sprintf("#!/bin/sh\nexec %q \"$@\"\n", wrapper))
	planner := filepath.Join(root, "planner")
	if out, err := exec.Command(realGoCommand(t), "build", "-o", planner, "./internal/cishard").CombinedOutput(); err != nil {
		t.Fatalf("planner build: %v\n%s", err, out)
	}
	// The local helper launches concurrent workers. Stub their Go commands so
	// this contract checks the complete route without running parallel suites.
	writeApexFixtureExecutable(t, filepath.Join(root, "go"), `#!/usr/bin/env python3
import json, sys
names = ["TestLocal%02d" % i for i in range(12)]
package = "github.com/glade-sh/glade/internal/apextest"
if "-list" in sys.argv:
    print("\n".join(names)); sys.exit(0)
assert "-timeout=30m" in sys.argv, sys.argv
import re
pattern = sys.argv[sys.argv.index("-run")+1].replace("(?:", "(")
for name in names:
    if re.fullmatch(pattern, name):
        print(json.dumps(dict(Action="pass", Package=package, Test=name, Elapsed=1)))
print(json.dumps(dict(Action="pass", Package=package, Elapsed=1)))
`)
	writeApexFixtureExecutable(t, filepath.Join(root, "renderer"), "#!/bin/sh\ncat >/dev/null\n")
	artifacts := filepath.Join(root, "artifacts")
	cmd := exec.Command("bash", filepath.Join(root, "scripts/apextest-shards.sh"), artifacts)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "PATH="+root+":"+os.Getenv("PATH"), "CI_SHARD_PLANNER="+planner,
		"CI_TESTLOG_RENDERER="+filepath.Join(root, "renderer"), "CI_APEXTEST_SHARDS=", "CI_TEST_BIN_DIR=", "CI_APEXTEST_HISTORY_PATH=")
	if out, err := cmd.CombinedOutput(); err != nil {
		logs := string(out)
		for index := range 2 {
			data, _ := os.ReadFile(filepath.Join(artifacts, fmt.Sprintf("shard-%d.log", index)))
			logs += string(data)
		}
		t.Fatalf("local two-worker route failed: %v\n%s", err, logs)
	}
	data, err := os.ReadFile(filepath.Join(artifacts, "duration-history.json"))
	var history struct{ Tests []struct{ Name string } }
	if err != nil || json.Unmarshal(data, &history) != nil || len(history.Tests) != 12 {
		t.Fatalf("local route did not retain the full two-shard union: %v %s", err, data)
	}
}

func TestCIScaleCodeQLJobAndAnalyzeRoutes(t *testing.T) {
	jobs := workflowJobBlocks(t, readWorkflowFile(t, "codeql.yml"))
	job := jobs["codeql"]
	if !strings.Contains(job, "    timeout-minutes: 15\n") {
		t.Error("CodeQL job needs setup and upload time around analysis")
	}
	if !strings.Contains(workflowStepBlock(t, job, "name: Analyze CodeQL"), "timeout-minutes: 6") {
		t.Error("CodeQL analysis needs a five-minute process budget plus cleanup")
	}
}

func scaleSemaHistoryFixture(t *testing.T) apexHistoryFixture {
	t.Helper()
	fixture := semaPairFixture(t)
	for index, dir := range fixture.shards {
		data, err := os.ReadFile(filepath.Join(dir, "events.json"))
		if err != nil {
			t.Fatal(err)
		}
		// Preserve the full synthetic discovery and terminal union, with a
		// first-run timing imbalance of the same shape as a larger test tree.
		var events strings.Builder
		for line := range strings.Lines(string(data)) {
			var event map[string]any
			if err := json.Unmarshal([]byte(line), &event); err != nil {
				t.Fatal(err)
			}
			if index == 0 {
				event["Elapsed"] = 4.0
			}
			encoded, _ := json.Marshal(event)
			events.Write(encoded)
			events.WriteByte('\n')
		}
		if err := os.WriteFile(filepath.Join(dir, "events.json"), []byte(events.String()), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return fixture
}

func TestCIScaleSemaPullRequestHistoryRoute(t *testing.T) {
	fixture := scaleSemaHistoryFixture(t)
	cmd := exec.Command("bash", "ci-go-test.sh", "sema-history-refresh", fixture.shards[0], fixture.shards[1], fixture.output)
	cmd.Env = append(os.Environ(), "GITHUB_EVENT_NAME=pull_request")
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "::warning::") {
		t.Fatalf("validated PR imbalance must warn and refresh: %v\n%s", err, out)
	}
	if data, err := os.ReadFile(fixture.output); err != nil || !json.Valid(data) {
		t.Fatalf("PR refresh must publish valid timing data: %v %s", err, data)
	}
}

func TestCIScaleSemaStrictAndCorruptHistoryRouteTwins(t *testing.T) {
	for _, tc := range []struct{ event, corrupt string }{
		{"push", ""}, {"schedule", ""}, {"workflow_dispatch", ""}, {"", ""},
		{"pull_request", "missing shard"}, {"pull_request", "malformed JSON"},
		{"pull_request", "failed test"}, {"pull_request", "invalid summary"},
	} {
		t.Run(tc.event+"/"+tc.corrupt, func(t *testing.T) {
			fixture := scaleSemaHistoryFixture(t)
			switch tc.corrupt {
			case "missing shard":
				fixture.shards[1] = filepath.Join(fixture.root, "missing")
			case "malformed JSON":
				if err := os.WriteFile(filepath.Join(fixture.shards[1], "plan.json"), []byte("{"), 0600); err != nil {
					t.Fatal(err)
				}
			case "failed test", "invalid summary":
				file, old, replacement := "events.json", `"Action":"pass"`, `"Action":"fail"`
				if tc.corrupt == "invalid summary" {
					file, old, replacement = "validation-summary.json", `"valid": true`, `"valid": false`
				}
				path := filepath.Join(fixture.shards[1], file)
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(string(data), old) {
					t.Fatal("corruption did not match fixture")
				}
				if err := os.WriteFile(path, []byte(strings.Replace(string(data), old, replacement, 1)), 0600); err != nil {
					t.Fatal(err)
				}
			}
			cmd := exec.Command("bash", "ci-go-test.sh", "sema-history-refresh", fixture.shards[0], fixture.shards[1], fixture.output)
			cmd.Env = append(os.Environ(), "GITHUB_EVENT_NAME="+tc.event)
			if out, err := cmd.CombinedOutput(); err == nil {
				t.Fatalf("non-PR imbalance or corrupt evidence must fail: %s", out)
			}
			if _, err := os.Stat(fixture.output); !os.IsNotExist(err) {
				t.Fatalf("rejected history was published: %v", err)
			}
		})
	}
}

func TestCIScalePlaywrightWorkflowRouteTwins(t *testing.T) {
	for _, filename := range []string{"ci.yml", "browser.yml", "race.yml"} {
		var workflow struct {
			Jobs map[string]struct {
				Steps []struct {
					Run              string
					WorkingDirectory string `yaml:"working-directory"`
				}
			}
		}
		if err := yaml.Unmarshal([]byte(readWorkflowFile(t, filename)), &workflow); err != nil {
			t.Fatal(err)
		}
		count := 0
		for job, body := range workflow.Jobs {
			for _, step := range body.Steps {
				if !strings.Contains(step.Run, "playwright install") {
					continue
				}
				count++
				t.Run(filename+"/"+job, func(t *testing.T) {
					if !strings.Contains(step.Run, "scripts/ci-install-playwright.sh ") {
						t.Fatalf("browser install route lacks resilient helper: %s", step.Run)
					}
					root := t.TempDir()
					project := root
					if step.WorkingDirectory == "source" {
						project = filepath.Join(root, "source")
					}
					for _, dir := range []string{"scripts", "site", "lwcruntime/node_modules/.bin"} {
						if err := os.MkdirAll(filepath.Join(project, dir), 0700); err != nil {
							t.Fatal(err)
						}
					}
					if err := os.MkdirAll(filepath.Join(root, "ci-artifacts/browser"), 0700); err != nil {
						t.Fatal(err)
					}
					helper, err := os.ReadFile("ci-install-playwright.sh")
					if err != nil {
						t.Fatal(err)
					}
					writeApexFixtureExecutable(t, filepath.Join(project, "scripts/ci-install-playwright.sh"), string(helper))
					browser := `#!/bin/bash
echo "$PLAYWRIGHT_DOWNLOAD_HOST $*" >>"$FIXTURE_ROOT/calls"
count=$(wc -l <"$FIXTURE_ROOT/calls")
if (( count < 3 )); then echo 'getaddrinfo ENOTFOUND'; exit 17; fi
`
					writeApexFixtureExecutable(t, filepath.Join(project, "lwcruntime/node_modules/.bin/playwright"), browser)
					writeApexFixtureExecutable(t, filepath.Join(root, "playwright"), browser)
					writeApexFixtureExecutable(t, filepath.Join(root, "npx"), "#!/bin/sh\nshift\nexec \"$FIXTURE_ROOT/playwright\" \"$@\"\n")
					for _, command := range []string{"npm", "sleep"} {
						writeApexFixtureExecutable(t, filepath.Join(root, command), "#!/bin/sh\nexit 0\n")
					}
					cmd := exec.Command("bash", "-eo", "pipefail", "-c", step.Run)
					cmd.Dir = filepath.Join(root, step.WorkingDirectory)
					cmd.Env = append(os.Environ(), "PATH="+root+":"+os.Getenv("PATH"), "FIXTURE_ROOT="+root,
						"PLAYWRIGHT_DOWNLOAD_HOST=", "GITHUB_WORKSPACE="+root)
					if out, err := cmd.CombinedOutput(); err != nil {
						t.Fatalf("workflow install must recover: %v\n%s", err, out)
					}
					calls, err := os.ReadFile(filepath.Join(root, "calls"))
					args := "install chromium"
					if strings.Contains(step.Run, "--with-deps") {
						args = "install --with-deps chromium"
					}
					if err != nil || string(calls) != strings.Repeat("https://cdn.playwright.dev "+args+"\n", 3) {
						t.Fatalf("workflow retry changed browser args: %v %s", err, calls)
					}
				})
			}
		}
		want := map[string]int{"ci.yml": 3, "browser.yml": 1, "race.yml": 3}[filename]
		if count != want {
			t.Errorf("%s browser install routes = %d, want %d", filename, count, want)
		}
	}
}

func TestCIScalePlaywrightPermanentFailureAndHostOverrideRoutes(t *testing.T) {
	root := t.TempDir()
	writeApexFixtureExecutable(t, filepath.Join(root, "playwright"), "#!/bin/sh\nprintf '%s|%s|%s\\n' \"$PLAYWRIGHT_DOWNLOAD_HOST\" \"$1\" \"$2\" >>\"$FIXTURE_ROOT/calls\"\nexit 17\n")
	writeApexFixtureExecutable(t, filepath.Join(root, "sleep"), "#!/bin/sh\nexit 0\n")
	cmd := exec.Command("bash", "ci-install-playwright.sh", filepath.Join(root, "playwright"), "argument with spaces", "chromium")
	cmd.Env = append(os.Environ(), "PATH="+root+":"+os.Getenv("PATH"), "FIXTURE_ROOT="+root,
		"PLAYWRIGHT_DOWNLOAD_HOST=https://download.example.test")
	out, err := cmd.CombinedOutput()
	if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 17 {
		t.Fatalf("permanent install failure must retain status 17: %v\n%s", err, out)
	}
	calls, _ := os.ReadFile(filepath.Join(root, "calls"))
	if string(calls) != strings.Repeat("https://download.example.test|argument with spaces|chromium\n", 3) {
		t.Fatalf("permanent failure must be bounded and preserve arguments/host: %s", calls)
	}
}

func TestCIScalePlaywrightRetryRoute(t *testing.T) {
	root := t.TempDir()
	writeApexFixtureExecutable(t, filepath.Join(root, "playwright"), `#!/bin/bash
echo "$PLAYWRIGHT_DOWNLOAD_HOST $*" >>"$FIXTURE_ROOT/calls"
count=$(wc -l <"$FIXTURE_ROOT/calls")
if (( count < 3 )); then echo 'getaddrinfo ENOTFOUND'; exit 17; fi
`)
	writeApexFixtureExecutable(t, filepath.Join(root, "sleep"), "#!/bin/sh\nprintf '%s\\n' \"$*\" >>\"$FIXTURE_ROOT/sleeps\"\n")
	cmd := exec.Command("bash", "ci-install-playwright.sh", filepath.Join(root, "playwright"), "install", "--with-deps", "chromium")
	cmd.Env = append(os.Environ(), "PATH="+root+":"+os.Getenv("PATH"), "FIXTURE_ROOT="+root, "PLAYWRIGHT_DOWNLOAD_HOST=")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("transient browser download must recover: %v\n%s", err, out)
	}
	calls, _ := os.ReadFile(filepath.Join(root, "calls"))
	if string(calls) != strings.Repeat("https://cdn.playwright.dev install --with-deps chromium\n", 3) {
		t.Fatalf("browser host/arguments/attempts = %s", calls)
	}
	sleeps, _ := os.ReadFile(filepath.Join(root, "sleeps"))
	if string(sleeps) != "5\n10\n" {
		t.Fatalf("browser backoff = %q, want 5s then 10s", sleeps)
	}
}
