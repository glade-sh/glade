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
	for _, marker := range []string{"shard: [0, 1, 2, 3, 4, 5]", "timeout-minutes: 100", "fail-fast: false"} {
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

// Exercise the real dispatcher, discovery, planner, test execution, validators
// and history collector on an owned project through both execution routes.
func TestCIScaleApexSourceAndSharedBinaryProjectRoutes(t *testing.T) {
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
					"CI_TESTLOG_RENDERER="+renderer, "CI_APEXTEST_ARTIFACT_DIR="+dir, "CI_APEXTEST_HISTORY_PATH=")
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
		})
	}
}

func TestCIScaleCodeQLJobAndAnalyzeRoutes(t *testing.T) {
	jobs := workflowJobBlocks(t, readWorkflowFile(t, "security.yml"))
	job := jobs["codeql"]
	if !strings.Contains(job, "    timeout-minutes: 100\n") {
		t.Error("CodeQL job needs setup and upload time around analysis")
	}
	if !strings.Contains(workflowStepBlock(t, job, "name: Analyze CodeQL"), "timeout-minutes: 90") {
		t.Error("CodeQL analysis needs a 90-minute budget")
	}
}

func TestCIScaleSemaPullRequestHistoryRoute(t *testing.T) {
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

func TestCIScalePlaywrightWorkflowRouteTwins(t *testing.T) {
	for _, filename := range []string{"ci.yml", "browser.yml", "race.yml"} {
		var workflow struct {
			Jobs map[string]struct {
				Steps []struct {
					Run string
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
						t.Errorf("browser install route lacks resilient helper: %s", step.Run)
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
