package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCIGofmtLeadingDashFiles(t *testing.T) {
	script, err := filepath.Abs("ci-gofmt.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name      string
		content   string
		wantError bool
	}{
		{name: "-formatted.go", content: "package fixture\n\nvar X = 1\n"},
		{name: "-unformatted.go", content: "package fixture\nvar X=1\n", wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			run := func(command string, args ...string) (string, error) {
				cmd := exec.Command(command, args...)
				cmd.Dir = dir
				out, err := cmd.CombinedOutput()
				return string(out), err
			}
			if out, err := run("git", "init", "-q"); err != nil {
				t.Fatalf("git init: %s: %v", out, err)
			}
			if err := os.WriteFile(filepath.Join(dir, tc.name), []byte(tc.content), 0644); err != nil {
				t.Fatal(err)
			}
			if out, err := run("git", "add", "--", tc.name); err != nil {
				t.Fatalf("git add: %s: %v", out, err)
			}
			out, err := run("bash", script)
			if !tc.wantError && err != nil {
				t.Fatalf("formatted Go rejected: %s: %v", out, err)
			}
			if tc.wantError && (err == nil || !strings.Contains(out, "Go files need gofmt:") || !strings.Contains(out, tc.name)) {
				t.Fatalf("unformatted Go not reported: %s: %v", out, err)
			}
		})
	}
}
