package refactorproof

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestChangedFilesWrapsGitChangesSince(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "glade@example.test")
	runGit(t, root, "config", "user.name", "GLADE")

	classPath := filepath.Join(root, "force-app", "main", "default", "classes", "InvoiceService.cls")
	if err := os.MkdirAll(filepath.Dir(classPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(classPath, []byte("public class InvoiceService {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "baseline")

	if err := os.WriteFile(classPath, []byte("public class InvoiceService { public void touch() {} }\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	changes, err := ChangedFiles(root, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 {
		t.Fatalf("changes = %#v, want one change", changes)
	}
	change := changes[0]
	if change.Path != classPath || change.Kind != "apex_class" || change.Operation != "modified" || change.Symbol != "InvoiceService" {
		t.Fatalf("change = %#v", change)
	}
}

func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
