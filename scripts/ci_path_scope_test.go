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
