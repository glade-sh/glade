package playground

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

// The digest is a hash of sorted path/NUL/content/NUL records, not of file
// contents alone. Anonymous Apex and seed data do not enter the runtime digest.
func TestWorkspaceHashContract(t *testing.T) {
	root := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("Alpha.cls", "public class Alpha {}")
	write("Run.apex", "System.debug('one');")
	write("seed.json", "{}")
	write(".hidden.cls", "hidden")
	write("ignored.txt", "ignored")
	w := &Workspace{Root: root}
	want := func(parts ...string) string {
		t.Helper()
		h := sha256.New()
		for _, part := range parts {
			h.Write([]byte(part))
			h.Write([]byte{0})
		}
		return "sha256:" + hex.EncodeToString(h.Sum(nil))
	}
	check := func(full, runtime string) {
		t.Helper()
		gotFull, err := w.Hash()
		if err != nil || gotFull != full {
			t.Fatalf("Hash() = %q, %v; want %q", gotFull, err, full)
		}
		gotRuntime, err := w.RuntimeSourceHash()
		if err != nil || gotRuntime != runtime {
			t.Fatalf("RuntimeSourceHash() = %q, %v; want %q", gotRuntime, err, runtime)
		}
	}
	runtime := want("Alpha.cls", "public class Alpha {}")
	check(want("Alpha.cls", "public class Alpha {}", "Run.apex", "System.debug('one');", "seed.json", "{}"), runtime)
	write("Run.apex", "System.debug('two');")
	write("seed.json", "{\"items\":[]}")
	check(want("Alpha.cls", "public class Alpha {}", "Run.apex", "System.debug('two');", "seed.json", "{\"items\":[]}"), runtime)
	write("Beta.trigger", "trigger Beta on Account (before insert) {}")
	check(want("Alpha.cls", "public class Alpha {}", "Beta.trigger", "trigger Beta on Account (before insert) {}", "Run.apex", "System.debug('two');", "seed.json", "{\"items\":[]}"), want("Alpha.cls", "public class Alpha {}", "Beta.trigger", "trigger Beta on Account (before insert) {}"))
}
