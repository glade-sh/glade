package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCIGofmtEnumerationFailure(t *testing.T) {
	script, err := filepath.Abs("ci-gofmt.sh")
	if err != nil {
		t.Fatal(err)
	}
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		output string
	}{
		{name: "empty"},
		{name: "partial", output: "printf 'good.go\\0'"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			cmd := exec.Command(realGit, "init", "-q", dir)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git init: %s: %v", out, err)
			}
			if err := os.WriteFile(filepath.Join(dir, "good.go"), []byte("package good\n"), 0644); err != nil {
				t.Fatal(err)
			}
			bin := filepath.Join(dir, "bin")
			if err := os.Mkdir(bin, 0755); err != nil {
				t.Fatal(err)
			}
			shim := "#!/bin/sh\nif [ \"$1\" = ls-files ]; then\n" + tc.output + "\nexit 42\nfi\nexec \"$REAL_GIT\" \"$@\"\n"
			if err := os.WriteFile(filepath.Join(bin, "git"), []byte(shim), 0755); err != nil {
				t.Fatal(err)
			}
			cmd = exec.Command("bash", script)
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "REAL_GIT="+realGit)
			if out, err := cmd.CombinedOutput(); err == nil {
				t.Fatalf("failed enumeration unexpectedly passed: %s", out)
			}
		})
	}
}
