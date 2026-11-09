package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestLeakScan exercises scripts/leak-scan.sh on a throwaway repository: a
// clean change passes; personal paths, host volume paths and private temp
// paths fail with their class; denylist entries match case-insensitively and
// are reported without the matched text; the allowlist exempts a path prefix
// from the denylist only.
func TestLeakScan(t *testing.T) {
	script, err := filepath.Abs("leak-scan.sh")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	run := func(env []string, args ...string) (string, error) {
		cmd := exec.Command("bash", append([]string{script}, args...)...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), env...)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s: %v", args, out, err)
		}
	}
	write := func(name, content string) {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	commit := func(msg string) {
		t.Helper()
		git("add", "-A")
		git("commit", "-q", "-m", msg)
	}
	git("init", "-q")
	write("README.md", "base\n")
	write(".github/leak-scan-allow.txt", "# allowlist\ndocs/pinned-\n")
	commit("base")

	const deny = "Zephyr-Internal-Project\n# a comment\n\nmarmoset.local\n"
	// The patterns are assembled here so this source file never contains them.
	personal := "/Us" + "ers/someone/work/notes.txt"
	linuxHome := "/ho" + "me/someone/work"
	runnerHome := "/ho" + "me/runner/.cache/ms-playwright"
	volume := "/Vol" + "umes/Archive/x"
	privateTmp := "/pri" + "vate/tmp/run.log"

	cases := []struct {
		name     string
		file     string
		content  string
		env      []string
		wantFail bool
		wantOut  string
		notOut   string
	}{
		{name: "clean addition", file: "docs/notes.md", content: "base\nplain documentation line\n", wantOut: "leak-scan: clean"},
		{name: "personal path", file: "docs/notes.md", content: "base\nsee " + personal + "\n", wantFail: true, wantOut: "docs/notes.md:2: personal-path", notOut: "someone"},
		{name: "linux home path", file: "docs/notes.md", content: "base\nsee " + linuxHome + "\n", wantFail: true, wantOut: "personal-path"},
		{name: "runner home path is fine", file: "docs/notes.md", content: "base\nPLAYWRIGHT_BROWSERS_PATH: " + runnerHome + "\n", wantOut: "leak-scan: clean"},
		{name: "volume path", file: "docs/notes.md", content: "base\ncopied from " + volume + "\n", wantFail: true, wantOut: "volume-path"},
		{name: "private tmp path", file: "docs/notes.md", content: "base\nlog at " + privateTmp + "\n", wantFail: true, wantOut: "private-tmp"},
		{name: "denylist entry case-insensitive", file: "docs/notes.md", content: "base\ntested against zephyr-internal-project yesterday\n", env: []string{"LEAK_SCAN_DENYLIST=" + deny}, wantFail: true, wantOut: "docs/notes.md:2: denylist", notOut: "zephyr"},
		{name: "second denylist entry", file: "docs/notes.md", content: "base\nssh marmoset.local\n", env: []string{"LEAK_SCAN_DENYLIST=" + deny}, wantFail: true, wantOut: "denylist"},
		{name: "allowlisted path skips denylist", file: "docs/pinned-projects.md", content: "Zephyr-Internal-Project at v1\n", env: []string{"LEAK_SCAN_DENYLIST=" + deny}, wantOut: "leak-scan: clean"},
		{name: "allowlisted path keeps builtins", file: "docs/pinned-projects.md", content: personal + "\n", env: []string{"LEAK_SCAN_DENYLIST=" + deny}, wantFail: true, wantOut: "personal-path"},
		{name: "removed lines are not scanned", file: "README.md", content: "", wantOut: "leak-scan: clean"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			write(tc.file, tc.content)
			commit(tc.name)
			out, err := run(tc.env, "--range", "HEAD~1..HEAD")
			if tc.wantFail && err == nil {
				t.Fatalf("expected a finding, got success: %s", out)
			}
			if !tc.wantFail && err != nil {
				t.Fatalf("expected clean, got %v: %s", err, out)
			}
			if !strings.Contains(out, tc.wantOut) {
				t.Fatalf("output %q lacks %q", out, tc.wantOut)
			}
			if tc.notOut != "" && strings.Contains(strings.ToLower(out), tc.notOut) {
				t.Fatalf("output leaks the matched text: %q", out)
			}
			// Restore the base content so the next case diffs against a known tree.
			write("README.md", "base\n")
			if tc.file != "README.md" {
				if err := os.Remove(filepath.Join(dir, tc.file)); err != nil {
					t.Fatal(err)
				}
			}
			commit("reset " + tc.name)
		})
	}
	t.Run("diff file input", func(t *testing.T) {
		diff := "diff --git a/x.txt b/x.txt\n--- a/x.txt\n+++ b/x.txt\n@@ -0,0 +1,2 @@\n+fine\n+" + volume + "\n"
		path := filepath.Join(dir, "input.diff")
		if err := os.WriteFile(path, []byte(diff), 0o644); err != nil {
			t.Fatal(err)
		}
		out, err := run(nil, "--diff", path)
		if err == nil || !strings.Contains(out, "x.txt:2: volume-path") {
			t.Fatalf("diff input: %v: %s", err, out)
		}
	})
	t.Run("usage error", func(t *testing.T) {
		if out, err := run(nil, "--bogus"); err == nil || !strings.Contains(out, "usage") {
			t.Fatalf("usage: %v: %s", err, out)
		}
	})
}

// TestLeakScanAdversarialDiffs covers diff content that must not control parsing or output.
func TestLeakScanAdversarialDiffs(t *testing.T) {
	script, err := filepath.Abs("leak-scan.sh")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git: %s: %v", out, err)
		}
	}
	write := func(name, content string) {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	run := func(args ...string) (string, error) {
		t.Helper()
		cmd := exec.Command("bash", append([]string{script}, args...)...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "LEAK_SCAN_DENYLIST=synthetic-token")
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	git("init", "-q")
	write(".github/leak-scan-allow.txt", "docs/allowed\n")
	write("base", "base\n")
	git("add", "-A")
	git("commit", "-qm", "base")
	private := "/ho" + "me/person/private"
	runner := "/ho" + "me/runner/cache"
	for _, tc := range []struct{ name, content, class string }{
		{"null header", "++ /dev/null\n" + private + "\n", "personal-path"},
		{"allow header", "++ b/docs/allowed\nsynthetic-token\n", "denylist"},
		{"secret header", "++ synthetic-secret\n" + private + "\n", "personal-path"},
		{"runner first", runner + " " + private + "\n", "personal-path"},
		{"runner last", private + " " + runner + "\n", "personal-path"},
		{"multiple runners", runner + " " + runner + " " + private + "\n", "personal-path"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			write("input", tc.content)
			git("add", "input")
			git("commit", "-qm", tc.name)
			out, err := run("--range", "HEAD~1..HEAD")
			if err == nil || !strings.Contains(out, tc.class) || strings.Contains(out, "synthetic-secret") {
				t.Fatalf("expected redacted %s finding: %v: %s", tc.class, err, out)
			}
		})
	}
	for _, tc := range []struct{ name, attrs, content string }{
		{"nul", "", "\x00" + private},
		{"attribute", "*.dat -diff\n", private + "\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			write(".gitattributes", tc.attrs)
			write(tc.name+".dat", tc.content)
			git("add", "-A")
			git("commit", "-qm", tc.name)
			out, err := run("--range", "HEAD~1..HEAD")
			if err == nil || !strings.Contains(out, "binary") {
				t.Fatalf("expected binary finding: %v: %s", err, out)
			}
		})
	}
	t.Run("sensitive filename", func(t *testing.T) {
		write("synthetic-secret.txt", private+"\n")
		git("add", "-A")
		git("commit", "-qm", "filename")
		out, err := run("--range", "HEAD~1..HEAD")
		if err == nil || !strings.Contains(out, "personal-path") || strings.Contains(out, "synthetic-secret") {
			t.Fatalf("expected redacted filename: %v: %s", err, out)
		}
	})
}

// TestLeakScanRequiredWorkflow keeps the leak-scan job wired into Required CI
// with the denylist secret and a base for every event the workflow runs on.
func TestLeakScanRequiredWorkflow(t *testing.T) {
	workflow, jobs := readCIWorkflow(t)
	job, ok := jobs["leak-scan"]
	if !ok {
		t.Fatal("ci.yml lacks the leak-scan job")
	}
	for _, marker := range []string{
		"runs-on: ubuntu-24.04",
		"uses: actions/checkout@df4cb1c069e1874edd31b4311f1884172cec0e10 # v6.0.3",
		"fetch-depth: 0",
		"persist-credentials: false",
		"LEAK_SCAN_DENYLIST: ${{ secrets.LEAK_SCAN_DENYLIST }}",
		"merge_group) range=\"$MERGE_GROUP_BASE_SHA..HEAD\"",
		"scripts/leak-scan.sh --range \"$range\"",
	} {
		if !strings.Contains(job, marker) {
			t.Errorf("leak-scan job missing %q", marker)
		}
	}
	if !strings.Contains(workflow, "  pull_request:\n  merge_group:\n") {
		t.Error("ci.yml must trigger on merge_group so the merge queue can run Required CI")
	}
	if !strings.Contains(jobs["required-ci"], "      - leak-scan\n") {
		t.Error("required-ci must depend on leak-scan")
	}
}
