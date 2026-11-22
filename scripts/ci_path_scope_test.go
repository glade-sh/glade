package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCIPathScope(t *testing.T) {
	for _, tc := range []struct {
		workflow, job, step, path, want string
		deleted                         bool
	}{
		{"ci.yml", "site", "Determine site test scope", "site/index.html", "true", false},
		{"ci.yml", "site", "Determine site test scope", "site/deleted.html", "true", true},
		{"ci.yml", "site", "Determine site test scope", "site-neighbor/index.html", "false", false},
		{"ci.yml", "site", "Determine site test scope", "scripts/ci-install-playwright.sh", "true", false},
		{"browser.yml", "browser", "Determine browser test scope", "scripts/ci-install-playwright.sh", "true", false},
		{"browser.yml", "browser", "Determine browser test scope", "scripts/ci-install-playwright-other.sh", "false", false},
	} {
		t.Run(tc.workflow+"/"+tc.path, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("..", ".github", "workflows", tc.workflow))
			if err != nil {
				t.Fatal(err)
			}
			job := workflowJobBlocksText(string(data))[tc.job]
			script := workflowRunScriptText(workflowStepBlockText(job, "      - name: "+tc.step))
			if script == "" {
				t.Fatal("scope script missing")
			}
			script = strings.ReplaceAll(script, `${{ github.event_name }}`, "pull_request")
			repo := t.TempDir()
			git := func(args ...string) string {
				t.Helper()
				cmd := exec.Command("git", args...)
				cmd.Dir = repo
				out, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatalf("git: %v: %s", err, out)
				}
				return strings.TrimSpace(string(out))
			}
			git("init", "-q")
			git("config", "user.email", "ci@example.invalid")
			git("config", "user.name", "CI")
			path := filepath.Join(repo, tc.path)
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			if tc.deleted {
				if err := os.WriteFile(path, []byte("old"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("base"), 0600); err != nil {
				t.Fatal(err)
			}
			git("add", "-A")
			git("commit", "-qm", "base")
			base := git("rev-parse", "HEAD")
			if tc.deleted {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(path, []byte("new"), 0600); err != nil {
				t.Fatal(err)
			}
			git("add", "-A")
			git("commit", "-qm", "change")
			head := git("rev-parse", "HEAD")
			output := filepath.Join(repo, "output")
			if err := os.MkdirAll(filepath.Join(repo, "ci-artifacts", "browser"), 0700); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("bash", "-c", script)
			cmd.Dir = repo
			cmd.Env = append(os.Environ(), "BASE_SHA="+base, "HEAD_SHA="+head, "UNCONDITIONAL=false", "GITHUB_OUTPUT="+output, "GITHUB_WORKSPACE="+repo, "RUNNER_TEMP="+t.TempDir())
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("scope: %v: %s", err, out)
			}
			got, err := os.ReadFile(output)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != "run_expensive="+tc.want+"\n" {
				t.Fatalf("scope = %q, want %s", got, tc.want)
			}
		})
	}
}

func TestCISiteScopeFailClosed(t *testing.T) {
	data, err := os.ReadFile("../.github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	job := workflowJobBlocksText(string(data))["site"]
	script := workflowRunScriptText(workflowStepBlockText(job, "      - name: Determine site test scope"))
	if script == "" {
		t.Fatal("site scope script missing")
	}
	for _, tc := range []struct {
		name, unconditional, base, head string
		success                         bool
		output                          string
	}{
		{"missing SHAs", "false", "", "", false, ""}, {"invalid diff", "false", "missing", "missing", false, ""}, {"full run", "true", "", "", true, "run_expensive=true\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			output := filepath.Join(t.TempDir(), "output")
			cmd := exec.Command("bash", "-c", script)
			cmd.Env = append(os.Environ(), "BASE_SHA="+tc.base, "HEAD_SHA="+tc.head, "UNCONDITIONAL="+tc.unconditional, "GITHUB_OUTPUT="+output, "RUNNER_TEMP="+t.TempDir())
			out, err := cmd.CombinedOutput()
			if (err == nil) != tc.success {
				t.Fatalf("scope error %v: %s", err, out)
			}
			if tc.success {
				got, err := os.ReadFile(output)
				if err != nil || string(got) != tc.output {
					t.Fatalf("output %q: %v", got, err)
				}
			}
		})
	}
	if !strings.Contains(job, "github.event_name != 'pull_request' || startsWith(github.head_ref, 'train/') || startsWith(github.head_ref, 'release/')") {
		t.Fatal("full-run scope lost")
	}
	for _, name := range []string{"npm test --prefix site", "npm run build --prefix site", "npm run check:built --prefix site", "Install Playwright Chromium", "Run rendered site checks", "Smoke-test the built preview"} {
		if !strings.Contains(job, name) {
			t.Errorf("site job missing %s", name)
		}
	}
	_, parsed := readCIWorkflow(t)
	aggregate := requiredAggregateShell(t, parsed["required-ci"])
	for _, result := range []string{"failure", "skipped", "cancelled"} {
		cmd := exec.Command("bash", "-c", aggregate)
		cmd.Env = append(os.Environ(), `REQUIRED_RESULTS={"site":{"result":"`+result+`"}}`, "EVENT_NAME=pull_request", "HEAD_REF=feature")
		if out, err := cmd.CombinedOutput(); err == nil || !strings.Contains(string(out), "site="+result) {
			t.Fatalf("site %s accepted: %v %s", result, err, out)
		}
	}
}

func TestCIRequiredAggregateRequiresSiteOnPullRequests(t *testing.T) {
	workflow := readWorkflowFile(t, "ci.yml")
	if problem := ciRequiredAggregateProblem(workflow); problem != "" {
		t.Fatalf("combined required aggregate: %s", problem)
	}
	mutated := strings.Replace(workflow, ` or .key == "site"`, "", 1)
	if mutated == workflow {
		t.Fatal("site selection not found")
	}
	if problem := ciRequiredAggregateProblem(mutated); problem == "" {
		t.Fatal("aggregate guard accepted a missing site result")
	}
}
