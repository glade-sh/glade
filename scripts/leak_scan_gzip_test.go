package scripts

import (
	"bytes"
	"compress/gzip"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The generated policy is gzip-compressed text. The scanner must inspect its
// contents and continue to reject malformed or non-text binary assets.
func TestLeakScanGzipText(t *testing.T) {
	script, err := filepath.Abs("leak-scan.sh")
	if err != nil {
		t.Fatal(err)
	}
	personal := "/Us" + "ers/someone/private"
	for _, tc := range []struct {
		name       string
		file       string
		content    []byte
		compress   bool
		wantFail   bool
		wantOutput string
	}{
		{"safe text", "policy.json.gz", []byte("synthetic policy\n"), true, false, "leak-scan: clean"},
		{"personal path", "policy.json.gz", []byte("synthetic policy\n" + personal + "\n"), true, true, "personal-path"},
		{"denylist entry", "policy.json.gz", []byte("synthetic-private-token\n"), true, true, "denylist"},
		{"binary payload", "policy.json.gz", []byte{'a', 0, 'b'}, true, true, "binary"},
		{"invalid gzip", "policy.json.gz", []byte{'a', 0, 'b'}, false, true, "binary"},
		{"truncated gzip", "policy.json.gz", []byte{0x1f, 0x8b, 0x08}, false, true, "binary"},
		{"unsupported filename", "unsafe name.gz", []byte("synthetic policy\n"), true, true, "binary"},
		{"oversized expansion", "policy.json.gz", bytes.Repeat([]byte{'x'}, 64*1024*1024+1), true, true, "binary"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			git := func(args ...string) {
				t.Helper()
				cmd := exec.Command("git", args...)
				cmd.Dir = dir
				cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid")
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("git: %s: %v", out, err)
				}
			}
			git("init", "-q")
			if err := os.WriteFile(filepath.Join(dir, "base"), []byte("base\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			git("add", "base")
			git("commit", "-qm", "base")
			data := tc.content
			if tc.compress {
				var buf bytes.Buffer
				zw := gzip.NewWriter(&buf)
				if _, err := zw.Write(data); err != nil {
					t.Fatal(err)
				}
				if err := zw.Close(); err != nil {
					t.Fatal(err)
				}
				data = buf.Bytes()
			}
			if err := os.WriteFile(filepath.Join(dir, tc.file), data, 0o644); err != nil {
				t.Fatal(err)
			}
			git("add", tc.file)
			git("commit", "-qm", "change")
			cmd := exec.Command("bash", script, "--range", "HEAD~1..HEAD")
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "LEAK_SCAN_DENYLIST=synthetic-private-token")
			out, err := cmd.CombinedOutput()
			if tc.wantFail != (err != nil) || !strings.Contains(string(out), tc.wantOutput) {
				t.Fatalf("want failure %v and %q; got %v: %s", tc.wantFail, tc.wantOutput, err, out)
			}
			if strings.Contains(string(out), "someone") || strings.Contains(string(out), "synthetic-private-token") {
				t.Fatalf("scanner exposed matched content: %s", out)
			}
		})
	}
}
