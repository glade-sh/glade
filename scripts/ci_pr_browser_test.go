package scripts

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCIPRBrowserSelection(t *testing.T) {
	for _, tc := range []struct {
		path string
		want bool
	}{
		{"internal/vm/example.go", true}, {"go.mod", true}, {"lwcruntime/package.json", true},
		{"scripts/ci-install-playwright.sh", true}, {".github/workflows/browser.yml", true},
		{"docs/guide.md", false}, {"site/index.md", false}, {"scripts/ci-go-test.sh", false},
	} {
		t.Run(strings.ReplaceAll(tc.path, "/", "_"), func(t *testing.T) {
			selected := filepath.Join(t.TempDir(), "selected.json")
			if err := os.WriteFile(selected, []byte(`[]`), 0600); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("python3", "ci-pr-browser-select.py", selected)
			cmd.Stdin = strings.NewReader(tc.path + "\x00")
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("selector: %v: %s", err, out)
			}
			var got struct {
				Browser  bool     `json:"browser"`
				Selected []string `json:"selected"`
			}
			if err := json.Unmarshal(out, &got); err != nil {
				t.Fatal(err)
			}
			if got.Browser != tc.want || (tc.want && len(got.Selected) != 3) || (!tc.want && len(got.Selected) != 0) {
				t.Fatalf("selection %s", out)
			}
		})
	}
}

func TestCIPRBrowserEvidence(t *testing.T) {
	good := []string{
		`{"Action":"pass","Package":"github.com/glade-sh/glade/internal/lwcruntime","Test":"TestBrowserRuntimeSuite"}`,
		`{"Action":"pass","Package":"github.com/glade-sh/glade/internal/lwcbrowser","Test":"TestGeneratedPhase3BaseComponentsRunInBrowser"}`,
		`{"Action":"pass","Package":"github.com/glade-sh/glade/internal/lwcbrowser","Test":"TestLWCAPI67RegistrationRunsInBrowser"}`,
		`{"Action":"pass","Package":"github.com/glade-sh/glade/internal/server","Test":"TestL08SalesforceConformance"}`,
	}
	for _, tc := range []struct {
		name    string
		events  []string
		node    string
		success bool
	}{
		{"complete", good, "# tests 2\n# pass 2\n# fail 0\n# skipped 0\n", true},
		{"missing", good[:3], "# tests 2\n# pass 2\n# fail 0\n# skipped 0\n", false},
		{"skipped", append(append([]string{}, good...), strings.Replace(good[0], `"pass"`, `"skip"`, 1)), "# tests 2\n# pass 2\n# fail 0\n# skipped 0\n", false},
		{"node skip", good, "# tests 2\n# pass 1\n# fail 0\n# skipped 1\n", false},
		{"missing node", good, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			input := filepath.Join(root, "input")
			output := filepath.Join(root, "output")
			if err := os.MkdirAll(input, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(input, "events.jsonl"), []byte(strings.Join(tc.events, "\n")+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if tc.node != "" {
				if err := os.WriteFile(filepath.Join(input, "node-test.log"), []byte(tc.node), 0600); err != nil {
					t.Fatal(err)
				}
			}
			cmd := exec.Command("python3", "ci-pr-browser-evidence.py", input, output)
			out, err := cmd.CombinedOutput()
			if (err == nil) != tc.success {
				t.Fatalf("evidence %v: %s", err, out)
			}
		})
	}
}

func TestCIPRBrowserWorkflow(t *testing.T) {
	browser, err := os.ReadFile("../.github/workflows/browser.yml")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(browser), "  pull_request:\n") {
		t.Error("Browser still runs on PRs")
	}
	_, jobs := readCIWorkflow(t)
	if jobs["pr-browser-evidence"] == "" || !strings.Contains(jobs["required-ci"], "      - pr-browser-evidence\n") {
		t.Error("PR browser evidence is not required")
	}
}

func TestCIPRBrowserEvidenceUsesLatestWorkerAttempt(t *testing.T) {
	root := t.TempDir()
	older := filepath.Join(root, "input", "pr-browser-1-1-12")
	newer := filepath.Join(root, "input", "pr-browser-1-2-12")
	for _, dir := range []string{older, newer} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	good := []string{
		`{"Action":"pass","Package":"github.com/glade-sh/glade/internal/lwcruntime","Test":"TestBrowserRuntimeSuite"}`,
		`{"Action":"pass","Package":"github.com/glade-sh/glade/internal/lwcbrowser","Test":"TestGeneratedPhase3BaseComponentsRunInBrowser"}`,
		`{"Action":"pass","Package":"github.com/glade-sh/glade/internal/lwcbrowser","Test":"TestLWCAPI67RegistrationRunsInBrowser"}`,
		`{"Action":"pass","Package":"github.com/glade-sh/glade/internal/server","Test":"TestL08SalesforceConformance"}`,
	}
	if err := os.WriteFile(filepath.Join(older, "events.jsonl"), []byte("invalid old attempt\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(newer, "events.jsonl"), []byte(strings.Join(good, "\n")+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(newer, "node-test.log"), []byte("# tests 2\n# pass 2\n# fail 0\n# skipped 0\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("python3", "ci-pr-browser-evidence.py", filepath.Join(root, "input"), filepath.Join(root, "output"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("latest attempt: %v: %s", err, out)
	}
}

func TestCIPRBrowserGoTestRetainsStatus(t *testing.T) {
	for _, tc := range []struct {
		name, code string
		success    bool
	}{{"pass", "0", true}, {"failure", "7", false}} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "go"), []byte("#!/bin/sh\nprintf '%s\\n' '{\"Action\":\"pass\"}'\nexit "+tc.code+"\n"), 0700); err != nil {
				t.Fatal(err)
			}
			events := filepath.Join(root, "events.jsonl")
			cmd := exec.Command("bash", "ci-pr-browser-go-test.sh", events, "example/package")
			cmd.Env = append(os.Environ(), "PATH="+root+":"+os.Getenv("PATH"))
			out, err := cmd.CombinedOutput()
			if (err == nil) != tc.success {
				t.Fatalf("status %v: %s", err, out)
			}
			data, err := os.ReadFile(events)
			if err != nil || !strings.Contains(string(data), `"Action":"pass"`) {
				t.Fatalf("evidence %q: %v", data, err)
			}
		})
	}
}

func TestCIPRBrowserUsesBothMergeParents(t *testing.T) {
	data, err := os.ReadFile("ci-pr-select.sh")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "--name-only -z --diff-filter=ACDMRTUXB HEAD^1 HEAD^2") {
		t.Fatal("browser scope must compare base and head, including deleted paths")
	}
}

func TestCIPRBrowserEvidenceDoesNotRunOnPush(t *testing.T) {
	_, jobs := readCIWorkflow(t)
	if !strings.Contains(jobs["pr-browser-evidence"], "github.event_name == 'pull_request'") {
		t.Fatal("PR browser evidence must be explicitly limited to pull requests")
	}
}
