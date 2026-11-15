package startupcache

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/filestamp"
)

func TestFingerprintRejectsDigestWhenFileChangesDuringLookup(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "input.js")
	if err := os.WriteFile(path, []byte("first"), 0600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := filestamp.Stat(path); !ok {
		t.Skip("filesystem does not expose a trusted file stamp")
	}
	stale := sha256.Sum256([]byte("first"))
	calls := 0
	lookup := func(string) ([sha256.Size]byte, bool) {
		calls++
		if calls == 2 {
			rewriteWithRestoredMtime(t, path, "other", info.ModTime())
		}
		return stale, true
	}
	fp, err := fingerprintInput(nil, root, path, true, [sha256.Size]byte{}, false, nil, lookup)
	if err != nil {
		t.Fatal(err)
	}
	current := sha256.Sum256([]byte("other"))
	if calls != 2 || fp.SHA256 != hex.EncodeToString(current[:]) {
		t.Fatalf("calls=%d digest=%s, want current content", calls, fp.SHA256)
	}
}

func TestFingerprintScopedDigestLookupReadsOnce(t *testing.T) {
	for _, change := range []bool{false, true} {
		t.Run(map[bool]string{false: "unchanged", true: "changed during lookup"}[change], func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "input.js")
			if err := os.WriteFile(path, []byte("first"), 0600); err != nil {
				t.Fatal(err)
			}
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := filestamp.Stat(path); !ok {
				t.Skip("filesystem does not expose a trusted file stamp")
			}
			first := sha256.Sum256([]byte("first"))
			calls := 0
			lookup := func(string) ([sha256.Size]byte, bool) {
				calls++
				if change {
					rewriteWithRestoredMtime(t, path, "other", info.ModTime())
				}
				return first, true
			}
			fp, err := fingerprintInputWithScopedDigest(nil, root, path, true, [sha256.Size]byte{}, false, nil, func(string) bool { return true }, lookup)
			if err != nil {
				t.Fatal(err)
			}
			want := first
			if change {
				want = sha256.Sum256([]byte("other"))
			}
			if calls != 1 || fp.SHA256 != hex.EncodeToString(want[:]) {
				t.Fatalf("calls=%d digest=%s, want current content", calls, fp.SHA256)
			}
		})
	}
}

func TestFingerprintScopedDigestIgnoresIneligibleInput(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "input.js")
	if err := os.WriteFile(path, []byte("first"), 0600); err != nil {
		t.Fatal(err)
	}
	calls := 0
	lookup := func(string) ([sha256.Size]byte, bool) { calls++; return [sha256.Size]byte{}, true }
	fp, err := fingerprintInputWithScopedDigest(nil, root, path, true, [sha256.Size]byte{}, false, nil, func(string) bool { return false }, lookup)
	if err != nil {
		t.Fatal(err)
	}
	want := sha256.Sum256([]byte("first"))
	if calls != 0 || fp.SHA256 != hex.EncodeToString(want[:]) {
		t.Fatalf("calls=%d digest=%s, want ordinary hash", calls, fp.SHA256)
	}
}

// rewriteWithRestoredMtime replaces the content and restores the modification
// time. A filesystem with one-second stamps cannot show a rewrite inside the
// tick that created the file, so the rewrite repeats until the stamp changes.
func rewriteWithRestoredMtime(t *testing.T, path, content string, mtime time.Time) {
	t.Helper()
	before, _ := filestamp.Stat(path)
	for range 40 {
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, mtime, mtime); err != nil {
			t.Fatal(err)
		}
		if after, _ := filestamp.Stat(path); after != before {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}
