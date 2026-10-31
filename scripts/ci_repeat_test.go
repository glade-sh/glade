package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCIRepeatRunner(t *testing.T) {
	root := t.TempDir()
	fake := filepath.Join(root, "go")
	log := filepath.Join(root, "calls")
	code := `#!/bin/sh
printf '%s\n' "$*" >> "$CALL_LOG"
case "$*" in
 *-list*) printf 'TestAlpha\nTestBeta\nok fixture\n';;
 *TestBeta*) [ "$FAIL_BETA" != 1 ] || exit 1;;
esac
`
	if err := os.WriteFile(fake, []byte(code), 0755); err != nil {
		t.Fatal(err)
	}
	inventory := filepath.Join(root, "tests.txt")
	run := func(rows, fail string) (string, error) {
		t.Helper()
		if err := os.WriteFile(inventory, []byte(rows), 0600); err != nil {
			t.Fatal(err)
		}
		os.Remove(log)
		cmd := exec.Command("python3", "ci-repeat.py", inventory)
		cmd.Env = append(os.Environ(), "PATH="+root+string(os.PathListSeparator)+os.Getenv("PATH"), "CALL_LOG="+log, "FAIL_BETA="+fail)
		out, err := cmd.CombinedOutput()
		calls, _ := os.ReadFile(log)
		return string(out) + string(calls), err
	}
	rows := "./internal/example TestAlpha\n./internal/example TestBeta\n"
	out, err := run(rows, "0")
	if err != nil || !strings.Contains(out, "-count=20") || !strings.Contains(out, "^TestAlpha$") || !strings.Contains(out, "^TestBeta$") {
		t.Fatalf("repeat execution: %v\n%s", err, out)
	}
	if out, err = run(rows, "1"); err == nil {
		t.Fatalf("failure hidden: %s", out)
	}
	if out, err = run("./internal/example TestMissing\n", "0"); err == nil {
		t.Fatalf("missing test accepted: %s", out)
	}
	if out, err = run("./internal/example TestAlpha\n./internal/example TestAlpha\n", "0"); err == nil {
		t.Fatalf("duplicate accepted: %s", out)
	}
}
