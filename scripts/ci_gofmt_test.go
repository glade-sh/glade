package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCIGofmt(t *testing.T) {
	script, err := filepath.Abs("ci-gofmt.sh")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	run := func(name string, args ...string) (string, error) {
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	if out, err := run("git", "init", "-q"); err != nil {
		t.Fatalf("git init: %s: %v", out, err)
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
	write("nested dir/formatted.go", "package nested\n\nvar X = 1\n")
	write("untracked.go", "package x\nvar X=1\n")
	if out, err := run("git", "add", "nested dir/formatted.go"); err != nil {
		t.Fatalf("git add: %s: %v", out, err)
	}
	if out, err := run("bash", script); err != nil {
		t.Fatalf("formatted tracked file: %s: %v", out, err)
	}
	write("nested dir/formatted.go", "package nested\nvar X=1\n")
	if out, err := run("bash", script); err == nil || !strings.Contains(out, "nested dir/formatted.go") {
		t.Fatalf("unformatted tracked file: %q, %v", out, err)
	}
	write("nested dir/formatted.go", "package nested\nfunc (\n")
	if out, err := run("bash", script); err == nil {
		t.Fatalf("invalid Go unexpectedly passed: %s", out)
	}
}

func TestCIGofmtRequiredWorkflow(t *testing.T) {
	data, err := os.ReadFile("../.github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(data)
	jobs := workflowJobBlocks(t, workflow)
	if !strings.Contains(jobs["workflow-lint"], "scripts/ci-gofmt.sh") {
		t.Fatal("workflow-lint must run gofmt check")
	}
	if !strings.Contains(jobs["required-ci"], "      - workflow-lint\n") {
		t.Fatal("Required CI must depend on workflow-lint")
	}
}
