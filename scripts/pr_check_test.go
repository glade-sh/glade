package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestPRCheck exercises scripts/pr-check.sh on a throwaway repository: a
// tooling-only branch passes; a product change without a test, a test commit
// after the fix, a semantic change without its ABI bumps, an organization id,
// a new package missing from the lane manifest and an unfilled PR body each
// fail with their check name; a removed fixture line warns, and fails under
// --strict.
func TestPRCheck(t *testing.T) {
	script, err := filepath.Abs("pr-check.sh")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	git := func(args ...string) string {
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
	run := func(args ...string) (string, int) {
		t.Helper()
		cmd := exec.Command("bash", append([]string{script, "--base", "main", "--no-leak-scan"}, args...)...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		code := 0
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else if err != nil {
			t.Fatalf("run: %s: %v", out, err)
		}
		return string(out), code
	}
	branch := func(name string) {
		t.Helper()
		git("checkout", "-q", "-B", name, "main")
	}

	git("init", "-q", "-b", "main")
	write("go.mod", "module example.invalid/glade\n\ngo 1.22\n")
	write("internal/sema/sema.go", "package sema\n\nconst SemanticABI = \"sema-v1\"\nconst PlatformABI = \"platform-v1\"\n")
	write("internal/apextest/startup_cache.go", "package apextest\n\nconst testRuntimeCacheABI = \"rt-v1\"\n")
	write("internal/vm/vm.go", "package vm\n\nfunc Run() int { return 1 }\n")
	write("internal/vm/vm_test.go", "package vm\n\nimport \"testing\"\n\nfunc TestRun(t *testing.T) {\n\tif Run() != 1 {\n\t\tt.Fatal()\n\t}\n}\n")
	write("internal/vm/testdata/cases.tsv", "id\tapi\n1\t62\n2\t67\n")
	write("scripts/ci-package-lanes.json", `{"version":1,"lanes":{"remaining-go":["example.invalid/glade/internal/sema","example.invalid/glade/internal/vm","example.invalid/glade/internal/apextest"]}}`)
	write("docs/RELEASE_NOTES.md", "## Unreleased\n\n- nothing\n")
	write("scripts/tool.sh", "#!/bin/sh\necho base\n")
	commit("base")

	expect := func(out string, code int, wantCode int, wantLines ...string) {
		t.Helper()
		if code != wantCode {
			t.Errorf("exit %d, want %d\n%s", code, wantCode, out)
		}
		for _, w := range wantLines {
			if !strings.Contains(out, w) {
				t.Errorf("missing %q in\n%s", w, out)
			}
		}
	}

	// tooling-only change: PASS
	branch("tooling")
	write("scripts/tool.sh", "#!/bin/sh\necho tooling\n")
	commit("tooling: change the script")
	out, code := run()
	expect(out, code, 0, "pr-check: PASS", "SKIP test-first", "SKIP abi")

	// product change without any test change: FAIL test-first
	branch("notest")
	write("internal/vm/vm.go", "package vm\n\nfunc Run() int { return 2 }\n")
	commit("vm: change Run")
	out, code = run()
	expect(out, code, 1, "FAIL test-first", "FAIL abi", "WARN release-notes", "WARN native-lines")

	// test committed after the fix: FAIL test-first
	branch("testafter")
	write("internal/vm/vm.go", "package vm\n\nfunc Run() int { return 2 }\n")
	write("internal/apextest/startup_cache.go", "package apextest\n\nconst testRuntimeCacheABI = \"rt-v2\"\n")
	commit("fix: Run returns 2\n\nNATIVE Run returns 2 internal/vm/testdata/org.tsv:3 anonymous 67.0")
	write("internal/vm/vm_test.go", "package vm\n\nimport \"testing\"\n\nfunc TestRun(t *testing.T) {\n\tif Run() != 2 {\n\t\tt.Fatal()\n\t}\n}\n")
	commit("test: Run returns 2")
	out, code = run()
	expect(out, code, 1, "FAIL test-first: first test commit is #2, after the first product commit #1", "PASS abi", "PASS native-lines")

	// test first, then a semantic change without SemanticABI: FAIL abi; with both bumps: PASS
	branch("sema")
	write("internal/sema/sema_test.go", "package sema\n\nimport \"testing\"\n\nfunc TestABI(t *testing.T) { _ = SemanticABI }\n")
	commit("test: sema abi (local contract: sema.go)")
	write("internal/sema/resolve.go", "package sema\n\nfunc Resolve() bool { return true }\n")
	write("docs/RELEASE_NOTES.md", "## Unreleased\n\n- sema: resolve more (local contract)\n")
	commit("sema: resolve more")
	out, code = run()
	expect(out, code, 1, "FAIL abi: bump required but missing: SemanticABI", "testRuntimeCacheABI", "PASS test-first", "PASS release-notes")
	write("internal/sema/sema.go", "package sema\n\nconst SemanticABI = \"sema-v2\"\nconst PlatformABI = \"platform-v1\"\n")
	write("internal/apextest/startup_cache.go", "package apextest\n\nconst testRuntimeCacheABI = \"rt-v2\"\n")
	commit("sema: bump SemanticABI and the runtime cache")
	out, code = run()
	expect(out, code, 0, "pr-check: PASS", "PASS abi")

	// a removed fixture row warns, and fails under --strict
	branch("deleted")
	write("internal/vm/testdata/cases.tsv", "id\tapi\n1\t62\n")
	commit("test: drop a row")
	out, code = run()
	expect(out, code, 0, "WARN deleted-assertions: 1 line(s) removed")
	out, code = run("--strict")
	expect(out, code, 1, "FAIL deleted-assertions")

	// an organization id in an added line fails
	branch("orgid")
	write("internal/vm/testdata/cases.tsv", "id\tapi\torg\n1\t62\t00D"+"5g000004abcDEAQ\n")
	commit("test: add a row")
	out, code = run()
	expect(out, code, 1, "FAIL org-id: 1 added line(s)")

	// a new package absent from the lane manifest fails
	branch("newpkg")
	write("internal/newpkg/newpkg_test.go", "package newpkg\n")
	commit("test: new package")
	write("internal/newpkg/newpkg.go", "package newpkg\n\nfunc X() int { return 1 }\n")
	write("internal/apextest/startup_cache.go", "package apextest\n\nconst testRuntimeCacheABI = \"rt-v2\"\n")
	commit("newpkg: add (local contract)")
	out, code = run()
	expect(out, code, 1, "FAIL lanes-manifest: new package(s) missing from scripts/ci-package-lanes.json: internal/newpkg")

	// PR body: placeholders and empty result cells fail; a filled body passes
	branch("body")
	write("scripts/tool.sh", "#!/bin/sh\necho body\n")
	commit("tooling: body")
	write("body-bad.md", "Ticket: <id>. Class: tooling.\n\n| Check | Command | Result |\n| --- | --- | --- |\n| Focused | `go test` | |\n")
	write("body-ok.md", "Ticket: ROAD-1. Class: tooling.\n\n| Check | Command | Result |\n| --- | --- | --- |\n| Focused | `go test` | ok |\n\n- [x] done\n")
	out, code = run("--body", "body-bad.md")
	expect(out, code, 1, "FAIL pr-body: placeholders 1, empty result cells 1")
	out, code = run("--body", "body-ok.md")
	expect(out, code, 0, "PASS pr-body")
}
