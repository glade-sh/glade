package watch

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGitChangesSinceReturnsWatchableChanges(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "glade@example.test")
	runGit(t, root, "config", "user.name", "GLADE")
	classPath := filepath.Join(root, "force-app", "main", "default", "classes", "PassingTest.cls")
	if err := os.MkdirAll(filepath.Dir(classPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(classPath, []byte("@isTest private class PassingTest {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "baseline")
	if err := os.WriteFile(classPath, []byte("@isTest private class PassingTest { @isTest static void runs() {} }\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	changes, err := GitChangesSince(root, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 {
		t.Fatalf("changes = %#v, want one change", changes)
	}
	if changes[0].Kind != FileKindApexClass || changes[0].Name != "PassingTest" || changes[0].Op != ChangeModified {
		t.Fatalf("change = %#v", changes[0])
	}
}

func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
