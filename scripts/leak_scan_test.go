package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestLeakScan exercises scripts/leak-scan.sh on a throwaway repository: a
// clean change passes; personal paths, host volume paths and private temp
// paths fail with their class; denylist entries match case-insensitively and
// are reported without the matched text; the allowlist exempts a path prefix
// from the denylist only. A denylist with no entries (empty or comments only)
// fails the scan, and LEAK_SCAN_ALLOW_EMPTY_DENYLIST=1 opts out with a warning
// while the pattern checks keep running.
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
		wantAll  []string
		notOut   string
	}{
		{name: "clean addition", file: "docs/notes.md", content: "base\nplain documentation line\n", env: []string{"LEAK_SCAN_DENYLIST=" + deny}, wantOut: "leak-scan: clean"},
		{name: "personal path", file: "docs/notes.md", content: "base\nsee " + personal + "\n", wantFail: true, wantOut: "file 1:2: personal-path", notOut: "someone"},
		{name: "linux home path", file: "docs/notes.md", content: "base\nsee " + linuxHome + "\n", wantFail: true, wantOut: "personal-path"},
		{name: "uppercase home", file: "docs/notes.md", content: "/ho" + "me/Alice/private\n", env: []string{"LEAK_SCAN_DENYLIST="}, wantFail: true, wantOut: "personal-path"},
		{name: "underscore home", file: "docs/notes.md", content: "/ho" + "me/_service/private\n", env: []string{"LEAK_SCAN_DENYLIST="}, wantFail: true, wantOut: "personal-path"},
		{name: "numeric home", file: "docs/notes.md", content: "/ho" + "me/123/private\n", env: []string{"LEAK_SCAN_DENYLIST="}, wantFail: true, wantOut: "personal-path"},
		{name: "runner lookalike", file: "docs/notes.md", content: "/ho" + "me/runner-other/private\n", env: []string{"LEAK_SCAN_DENYLIST="}, wantFail: true, wantOut: "personal-path"},
		{name: "hidden mac home", file: "docs/notes.md", content: "/Us" + "ers/.hidden/private\n", env: []string{"LEAK_SCAN_DENYLIST="}, wantFail: true, wantOut: "personal-path"},
		{name: "punctuation mac home", file: "docs/notes.md", content: "/Us" + "ers/-person/private\n", env: []string{"LEAK_SCAN_DENYLIST="}, wantFail: true, wantOut: "personal-path"},
		{name: "runner home path is fine", file: "docs/notes.md", content: "base\nPLAYWRIGHT_BROWSERS_PATH: " + runnerHome + "\n", env: []string{"LEAK_SCAN_DENYLIST=" + deny}, wantOut: "leak-scan: clean"},
		{name: "volume path", file: "docs/notes.md", content: "base\ncopied from " + volume + "\n", wantFail: true, wantOut: "volume-path"},
		{name: "private tmp path", file: "docs/notes.md", content: "base\nlog at " + privateTmp + "\n", wantFail: true, wantOut: "private-tmp"},
		{name: "denylist entry case-insensitive", file: "docs/notes.md", content: "base\ntested against zephyr-internal-project yesterday\n", env: []string{"LEAK_SCAN_DENYLIST=" + deny}, wantFail: true, wantOut: "file 1:2: denylist", notOut: "zephyr"},
		{name: "second denylist entry", file: "docs/notes.md", content: "base\nssh marmoset.local\n", env: []string{"LEAK_SCAN_DENYLIST=" + deny}, wantFail: true, wantOut: "denylist"},
		{name: "allowlisted path skips denylist", file: "docs/pinned-projects.md", content: "Zephyr-Internal-Project at v1\n", env: []string{"LEAK_SCAN_DENYLIST=" + deny}, wantOut: "leak-scan: clean"},
		{name: "allowlisted path keeps builtins", file: "docs/pinned-projects.md", content: personal + "\n", env: []string{"LEAK_SCAN_DENYLIST=" + deny}, wantFail: true, wantOut: "personal-path"},
		{name: "removed lines are not scanned", file: "README.md", content: "", env: []string{"LEAK_SCAN_DENYLIST=" + deny}, wantOut: "leak-scan: clean"},
		{name: "empty denylist fails", file: "docs/notes.md", content: "base\nplain empty-denylist case line\n", env: []string{"LEAK_SCAN_DENYLIST=", "LEAK_SCAN_ALLOW_EMPTY_DENYLIST=0"}, wantFail: true, wantOut: "leak-scan: FAIL LEAK_SCAN_DENYLIST is empty"},
		{name: "comments-only denylist fails", file: "docs/notes.md", content: "base\nplain comments-only case line\n", env: []string{"LEAK_SCAN_DENYLIST=# nothing but a comment\n", "LEAK_SCAN_ALLOW_EMPTY_DENYLIST=0"}, wantFail: true, wantOut: "leak-scan: FAIL LEAK_SCAN_DENYLIST is empty"},
		{name: "empty denylist opt-out passes and warns", file: "docs/notes.md", content: "base\nplain opt-out case line\n", env: []string{"LEAK_SCAN_DENYLIST=", "LEAK_SCAN_ALLOW_EMPTY_DENYLIST=1"}, wantOut: "leak-scan: WARNING", wantAll: []string{"denylist check skipped", "leak-scan: clean"}},
		{name: "empty denylist opt-out still scans patterns", file: "docs/notes.md", content: "base\nsee " + personal + "\n", env: []string{"LEAK_SCAN_DENYLIST=", "LEAK_SCAN_ALLOW_EMPTY_DENYLIST=1"}, wantFail: true, wantOut: "personal-path", wantAll: []string{"leak-scan: WARNING denylist check skipped", "leak-scan: 1 finding(s) in 1 file(s)"}},
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
			for _, want := range tc.wantAll {
				if !strings.Contains(out, want) {
					t.Fatalf("output %q lacks %q", out, want)
				}
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
		if err == nil || !strings.Contains(out, "file 1:2: volume-path") {
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
		{"overlapping paths", "/ho" + "me/runner" + private + "\n", "personal-path"},
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
	for _, tc := range []struct{ name, attrs, content string }{
		{"nul deletion", "", "\x00deleted"},
		{"attribute deletion", "*.bin -diff\n", "deleted text\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			write(".gitattributes", tc.attrs)
			write("remove.bin", tc.content)
			git("add", "-A")
			git("commit", "-qm", "prepare "+tc.name)
			git("rm", "-q", "remove.bin")
			git("commit", "-qm", tc.name)
			out, err := run("--range", "HEAD~1..HEAD")
			if err != nil || !strings.Contains(out, "leak-scan: clean") {
				t.Fatalf("binary deletion should pass: %v: %s", err, out)
			}
		})
	}
	t.Run("binary modification", func(t *testing.T) {
		write("modify.bin", "\x00before")
		git("add", "-A")
		git("commit", "-qm", "prepare modification")
		write("modify.bin", "\x00after")
		git("add", "-A")
		git("commit", "-qm", "modification")
		out, err := run("--range", "HEAD~1..HEAD")
		if err == nil || !strings.Contains(out, "binary") {
			t.Fatalf("binary modification must fail: %v: %s", err, out)
		}
	})
	t.Run("binary delete and add", func(t *testing.T) {
		write("a-remove.bin", "\x00before")
		git("add", "-A")
		git("commit", "-qm", "prepare mixed change")
		git("rm", "-q", "a-remove.bin")
		write("z-add.bin", "\x00different")
		git("add", "-A")
		git("commit", "-qm", "mixed change")
		out, err := run("--range", "HEAD~1..HEAD")
		if err == nil || !strings.Contains(out, "binary") {
			t.Fatalf("binary addition must fail: %v: %s", err, out)
		}
	})
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

// TestLeakScanCommits exercises scripts/leak-scan-commits.sh on throwaway
// repositories: a denylist finding names the commit that introduced it and no
// clean commit; a clean range passes; an empty denylist fails the range, and
// LEAK_SCAN_ALLOW_EMPTY_DENYLIST=1 opts out with one warning for the whole
// range; a lone root commit is scanned against the empty tree; a bad or empty
// range fails instead of reporting a clean result.
func TestLeakScanCommits(t *testing.T) {
	script, err := filepath.Abs("leak-scan-commits.sh")
	if err != nil {
		t.Fatal(err)
	}
	gitIn := func(dir string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s: %v", args, out, err)
		}
		return string(out)
	}
	writeIn := func(dir, name, content string) {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	commitIn := func(dir, msg string) string {
		t.Helper()
		gitIn(dir, "add", "-A")
		gitIn(dir, "commit", "-q", "-m", msg)
		return strings.TrimSpace(gitIn(dir, "rev-parse", "HEAD"))
	}
	runIn := func(dir string, env []string, args ...string) (string, error) {
		cmd := exec.Command("bash", append([]string{script}, args...)...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), env...)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	shortIn := func(dir, sha string) string {
		t.Helper()
		return strings.TrimSpace(gitIn(dir, "rev-parse", "--short", sha))
	}

	// A range whose middle commit adds a denylist entry.
	dir := t.TempDir()
	gitIn(dir, "init", "-q")
	writeIn(dir, "README.md", "base\n")
	base := commitIn(dir, "base")
	writeIn(dir, "docs/one.md", "plain documentation\n")
	first := commitIn(dir, "clean")
	writeIn(dir, "docs/two.md", "tested against Zephyr-Internal-Project\n")
	hit := commitIn(dir, "denylist hit")
	writeIn(dir, "docs/three.md", "plain documentation\n")
	last := commitIn(dir, "clean again")

	t.Run("range names the offending commit", func(t *testing.T) {
		out, err := runIn(dir, []string{"LEAK_SCAN_DENYLIST=Zephyr-Internal-Project"}, base+"..HEAD")
		if err == nil {
			t.Fatalf("expected a failure naming %s: %s", shortIn(dir, hit), out)
		}
		for _, want := range []string{
			"leak-scan-commits: commit " + shortIn(dir, hit),
			"denylist",
			"leak-scan-commits: 1 of 3 commit(s) have findings",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("output lacks %q:\n%s", want, out)
			}
		}
		for _, clean := range []string{shortIn(dir, first), shortIn(dir, last)} {
			if strings.Contains(out, "commit "+clean) {
				t.Errorf("output names a clean commit %s:\n%s", clean, out)
			}
		}
	})
	t.Run("clean range passes", func(t *testing.T) {
		out, err := runIn(dir, []string{"LEAK_SCAN_DENYLIST=marmoset.invalid"}, base+"..HEAD")
		if err != nil {
			t.Fatalf("expected clean, got %v: %s", err, out)
		}
		if !strings.Contains(out, "leak-scan-commits: 3 commit(s) clean") {
			t.Fatalf("output lacks the clean summary: %s", out)
		}
	})
	t.Run("empty denylist fails the range", func(t *testing.T) {
		out, err := runIn(dir, []string{"LEAK_SCAN_DENYLIST=", "LEAK_SCAN_ALLOW_EMPTY_DENYLIST=0"}, base+"..HEAD")
		if err == nil || !strings.Contains(out, "LEAK_SCAN_DENYLIST is empty") {
			t.Fatalf("an empty denylist must fail the whole range: %v: %s", err, out)
		}
	})
	t.Run("empty denylist opt-out warns once and passes", func(t *testing.T) {
		out, err := runIn(dir, []string{"LEAK_SCAN_DENYLIST=", "LEAK_SCAN_ALLOW_EMPTY_DENYLIST=1"}, base+"..HEAD")
		if err != nil {
			t.Fatalf("expected clean with the skip warning, got %v: %s", err, out)
		}
		if strings.Count(out, "leak-scan: WARNING") != 1 {
			t.Fatalf("the skip warning must appear once per range, got %d:\n%s", strings.Count(out, "leak-scan: WARNING"), out)
		}
		if !strings.Contains(out, "denylist check skipped") || !strings.Contains(out, "leak-scan-commits: 3 commit(s) clean") {
			t.Fatalf("output lacks the skip warning and the clean summary: %s", out)
		}
	})
	// A pattern finding while the skip is in force must name the offending
	// commit, keep scanning the remaining commits, and must not be reported as
	// the empty denylist failing every commit.
	writeIn(dir, "docs/four.md", "see /Us"+"ers/someone/notes.txt\n")
	patternHit := commitIn(dir, "pattern hit")
	writeIn(dir, "docs/five.md", "plain documentation\n")
	commitIn(dir, "clean after the hit")
	t.Run("opt-out pattern finding names the commit and keeps scanning", func(t *testing.T) {
		out, err := runIn(dir, []string{"LEAK_SCAN_DENYLIST=", "LEAK_SCAN_ALLOW_EMPTY_DENYLIST=1"}, base+"..HEAD")
		if err == nil {
			t.Fatalf("expected the range to fail: %s", out)
		}
		for _, want := range []string{
			"leak-scan-commits: commit " + shortIn(dir, patternHit),
			"personal-path",
			"leak-scan: WARNING denylist check skipped",
			"leak-scan-commits: 1 of 5 commit(s) have findings",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("output lacks %q:\n%s", want, out)
			}
		}
		if strings.Contains(out, "LEAK_SCAN_DENYLIST is empty for every commit") {
			t.Errorf("a pattern finding must not be reported as the empty denylist:\n%s", out)
		}
	})
	t.Run("usage and range errors", func(t *testing.T) {
		if out, err := runIn(dir, nil); err == nil || !strings.Contains(out, "usage") {
			t.Fatalf("usage: %v: %s", err, out)
		}
		if out, err := runIn(dir, nil, "nosuchref..HEAD"); err == nil || !strings.Contains(out, "bad range") {
			t.Fatalf("bad range: %v: %s", err, out)
		}
		if out, err := runIn(dir, nil, base+".."+base); err == nil || !strings.Contains(out, "no commits in") {
			t.Fatalf("empty range: %v: %s", err, out)
		}
	})

	// A lone root commit: the range is the commit itself, with no parent.
	solo := t.TempDir()
	gitIn(solo, "init", "-q")
	writeIn(solo, "docs/solo.md", "see /Us"+"ers/someone/notes.txt\n")
	root := commitIn(solo, "root")
	t.Run("root commit scans against the empty tree", func(t *testing.T) {
		out, err := runIn(solo, []string{"LEAK_SCAN_DENYLIST=", "LEAK_SCAN_ALLOW_EMPTY_DENYLIST=1"}, root)
		if err == nil {
			t.Fatalf("expected the root commit to fail: %s", out)
		}
		for _, want := range []string{"leak-scan-commits: commit " + shortIn(solo, root), "personal-path"} {
			if !strings.Contains(out, want) {
				t.Errorf("output lacks %q:\n%s", want, out)
			}
		}
	})
}

// TestLeakScanRequiredWorkflow keeps the leak-scan job wired into Required CI
// with the denylist secret and a base for every event the workflow runs on.
// The empty-denylist opt-out is passed only for pull requests that cannot see
// repository secrets: a fork head, or any head from a different repository.
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
		"LEAK_SCAN_ALLOW_EMPTY_DENYLIST: ${{ github.event_name == 'pull_request' && (github.event.pull_request.head.repo.fork == true || github.event.pull_request.head.repo.full_name != github.event.pull_request.base.repo.full_name) && '1' || '0' }}",
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

// TestLeakScanUsesTrustedTree runs the workflow's shell steps against a local
// Git remote. Candidate commits are inputs to the scan, never the source of
// the scanner, gzip helper or allowlist for a PR or merge group.
func TestLeakScanUsesTrustedTree(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		Jobs map[string]struct {
			Steps []struct {
				Run  string            `yaml:"run"`
				Uses string            `yaml:"uses"`
				Env  map[string]string `yaml:"env"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		t.Fatal(err)
	}
	steps := workflow.Jobs["leak-scan"].Steps
	if len(steps) != 2 || steps[0].Run == "" || steps[1].Run == "" || steps[0].Uses != "" || steps[1].Uses != "" {
		t.Fatal("leak scan must prepare and scan with runner shell commands only")
	}
	if steps[0].Env["LEAK_SCAN_DENYLIST"] != "" || steps[1].Env["LEAK_SCAN_DENYLIST"] != "${{ secrets.LEAK_SCAN_DENYLIST }}" {
		t.Fatal("only the scan step may receive the denylist")
	}

	server := t.TempDir()
	const repo = "example/scan"
	source := filepath.Join(server, repo+".git")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = source
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	write := func(path string, content []byte) {
		t.Helper()
		path = filepath.Join(source, path)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, content, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	git("init", "-q", "-b", "main")
	for _, file := range []string{"leak-scan.sh", "leak-scan-expand-gzip.py"} {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		write("scripts/"+file, content)
	}
	write(".github/leak-scan-allow.txt", []byte("docs/safe/\n"))
	git("add", "-A")
	git("commit", "-qm", "trusted base")
	base := git("rev-parse", "HEAD")
	git("checkout", "-qb", "candidate")
	write("scripts/leak-scan.sh", []byte("#!/bin/bash\nexit 0\n"))
	write("scripts/leak-scan-expand-gzip.py", []byte("raise SystemExit(42)\n"))
	write(".github/leak-scan-allow.txt", []byte("docs/\n"))
	const deny = "synthetic-guard-term"
	write("docs/added.txt", []byte(deny+"\n"))
	write("docs/safe/allowed.txt", []byte(deny+"\n"))
	git("add", "-A")
	git("commit", "-qm", "candidate")
	git("checkout", "-q", "main")
	git("merge", "-q", "--no-ff", "-m", "test merge", "candidate")
	merge := git("rev-parse", "HEAD")

	for _, event := range []string{"pull_request", "merge_group"} {
		t.Run(event, func(t *testing.T) {
			workspace := t.TempDir()
			output := filepath.Join(t.TempDir(), "output")
			prepare := exec.Command("bash", "-eo", "pipefail", "-c", steps[0].Run)
			prepare.Dir = workspace
			prepare.Env = append(os.Environ(), "GITHUB_SERVER_URL=file://"+server, "GITHUB_REPOSITORY="+repo,
				"GITHUB_SHA="+merge, "EVENT_NAME="+event, "PR_BASE_SHA="+base, "MERGE_GROUP_BASE_SHA="+base, "GITHUB_OUTPUT="+output)
			if out, err := prepare.CombinedOutput(); err != nil {
				t.Fatalf("prepare trusted workspace: %v: %s", err, out)
			}
			bindings, err := os.ReadFile(output)
			if err != nil || string(bindings) != "range="+base+".."+merge+"\n" {
				t.Fatalf("scan must use the exact base and merge commits: %v: %q", err, bindings)
			}
			if _, err := os.Stat(filepath.Join(workspace, "docs", "added.txt")); !os.IsNotExist(err) {
				t.Fatalf("candidate file must stay out of the worktree: %v", err)
			}
			scan := exec.Command("bash", "-eo", "pipefail", "-c", steps[1].Run)
			scan.Dir = workspace
			scan.Env = append(os.Environ(), "SCAN_RANGE="+base+".."+merge, "LEAK_SCAN_DENYLIST="+deny, "LEAK_SCAN_ALLOW_EMPTY_DENYLIST=0")
			out, err := scan.CombinedOutput()
			if err == nil || !strings.Contains(string(out), "1 finding(s) in 1 file(s)") || !strings.Contains(string(out), "denylist") || strings.Contains(string(out), deny) {
				t.Fatalf("trusted scan must reject and redact one candidate finding: %v: %s", err, out)
			}
		})
	}
}
