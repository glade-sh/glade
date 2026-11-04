package playground

import (
	"os"
	"path/filepath"
	"testing"
)

// The combined operation must preserve both established digest contracts.
func TestWorkspaceCombinedHashesMatchSeparateHashes(t *testing.T) {
	root := t.TempDir()
	w := &Workspace{Root: root}
	combined, ok := any(w).(interface {
		workspaceHashes() (string, string, error)
	})
	if !ok {
		t.Fatal("workspace cannot compute both hashes in one operation")
	}
	put := func(path, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, path), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	check := func() {
		t.Helper()
		wantFull, err := w.Hash()
		if err != nil {
			t.Fatal(err)
		}
		wantRuntime, err := w.RuntimeSourceHash()
		if err != nil {
			t.Fatal(err)
		}
		gotFull, gotRuntime, err := combined.workspaceHashes()
		if err != nil || gotFull != wantFull || gotRuntime != wantRuntime {
			t.Fatalf("combined = (%q, %q, %v), separate = (%q, %q)", gotFull, gotRuntime, err, wantFull, wantRuntime)
		}
	}
	check()
	put("Éclair.cls", "public class Eclair {}")
	put("Run.apex", "System.debug(1);")
	put("seed.json", "{}")
	put("config.json", "{\"version\":1}")
	put(".hidden.cls", "ignored")
	put("notes.txt", "ignored")
	check()
	put("Run.apex", "System.debug(2);")
	put("seed.json", "[]")
	check()
	if err := os.Remove(filepath.Join(root, "Éclair.cls")); err != nil {
		t.Fatal(err)
	}
	check()
}
