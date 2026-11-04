package playground

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorkspaceCombinedHashesIgnoreSymlinks(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.cls")
	if err := os.WriteFile(outside, []byte("public class Outside {}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "link.cls")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	w := &Workspace{Root: root}
	beforeFull, beforeRuntime, err := w.workspaceHashes()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outside, []byte("changed outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	afterFull, afterRuntime, err := w.workspaceHashes()
	if err != nil {
		t.Fatal(err)
	}
	if beforeFull != afterFull || beforeRuntime != afterRuntime {
		t.Fatalf("outside symlink target affected workspace hashes")
	}
	full, err := w.Hash()
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := w.RuntimeSourceHash()
	if err != nil {
		t.Fatal(err)
	}
	if afterFull != full || afterRuntime != runtime {
		t.Fatalf("combined symlink hashes differ from separate hashes")
	}
}
