package semanticcache

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/filestamp"
)

// requireStamps skips tests of the cached path on file systems whose ctime
// cannot be relied on; there every call hashes.
func requireStamps(t *testing.T) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "probe")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := filestamp.Stat(path); !ok {
		t.Skip("file system without a trusted ctime")
	}
}

func sha(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func writeExe(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "glade-test-exe")
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func digestOf(t *testing.T, path, cacheDir string, now func() time.Time) string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	got, err := executableDigest(file, cacheDir, now)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func inAnHour() time.Time { return time.Now().Add(time.Hour) }

func countHashes(t *testing.T) *int {
	t.Helper()
	count := new(int)
	old := executableHashHook
	executableHashHook = func() { *count++ }
	t.Cleanup(func() { executableHashHook = old })
	return count
}

func TestExecutableDigestMatchesSHA256WithAndWithoutCache(t *testing.T) {
	path := writeExe(t, "executable bytes")
	want := sha("executable bytes")
	for name, dir := range map[string]string{"no cache dir": "", "cache dir": t.TempDir(), "unusable cache dir": filepath.Join(path, "nope")} {
		for pass := range 2 {
			if got := digestOf(t, path, dir, inAnHour); got != want {
				t.Fatalf("%s pass %d: digest %q, want %q", name, pass, got, want)
			}
		}
	}
}

func TestExecutableDigestIsReusedAcrossCalls(t *testing.T) {
	requireStamps(t)
	path := writeExe(t, "executable bytes")
	cacheDir := t.TempDir()
	hashes := countHashes(t)
	for range 4 {
		if got := digestOf(t, path, cacheDir, inAnHour); got != sha("executable bytes") {
			t.Fatalf("digest = %q", got)
		}
	}
	if *hashes != 1 {
		t.Fatalf("executable hashed %d times over 4 calls, want 1", *hashes)
	}
}

func TestExecutableDigestNeverServesStaleEntry(t *testing.T) {
	requireStamps(t)
	for name, edit := range map[string]func(t *testing.T, path string){
		"rewrite same size and restore mtime": func(t *testing.T, path string) {
			info, _ := os.Stat(path)
			if err := os.WriteFile(path, []byte("executable BYTES"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
				t.Fatal(err)
			}
		},
		"atomic replace": func(t *testing.T, path string) {
			info, _ := os.Stat(path)
			next := path + ".new"
			if err := os.WriteFile(next, []byte("executable BYTES"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.Chtimes(next, info.ModTime(), info.ModTime()); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(next, path); err != nil {
				t.Fatal(err)
			}
		},
		"append": func(t *testing.T, path string) {
			f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			if _, err := f.WriteString("!"); err != nil {
				t.Fatal(err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			path := writeExe(t, "executable bytes")
			cacheDir := t.TempDir()
			if got := digestOf(t, path, cacheDir, inAnHour); got != sha("executable bytes") {
				t.Fatalf("first digest = %q", got)
			}
			time.Sleep(30 * time.Millisecond)
			edit(t, path)
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if got := digestOf(t, path, cacheDir, inAnHour); got != sha(string(content)) {
				t.Fatalf("digest after edit = %q, want %q (stale entry served)", got, sha(string(content)))
			}
		})
	}
}

func TestExecutableDigestIgnoresDamagedEntries(t *testing.T) {
	requireStamps(t)
	path := writeExe(t, "executable bytes")
	cacheDir := t.TempDir()
	want := sha("executable bytes")
	if got := digestOf(t, path, cacheDir, inAnHour); got != want {
		t.Fatalf("digest = %q", got)
	}
	entries, err := os.ReadDir(cacheDir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("cache entries = %v, %v; want exactly one", entries, err)
	}
	entry := filepath.Join(cacheDir, entries[0].Name())
	for name, damaged := range map[string]string{
		"empty":        "",
		"short":        want[:10],
		"not hex":      "zz" + want[2:],
		"uppercase":    "ABCDEF" + want[6:],
		"trailing":     want + "00",
		"other digest": sha("something else"),
	} {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(entry, []byte(damaged), 0o600); err != nil {
				t.Fatal(err)
			}
			got := digestOf(t, path, cacheDir, inAnHour)
			if name == "other digest" {
				// A well-formed entry under the right stamp is trusted by design:
				// it is as writable as the binary itself. Reset for the next case.
				if got != damaged {
					t.Fatalf("well-formed entry not used: %q", got)
				}
				return
			}
			if got != want {
				t.Fatalf("damaged entry %q gave digest %q, want %q", name, got, want)
			}
		})
	}
}

func TestExecutableDigestSkipsCachingRecentFiles(t *testing.T) {
	requireStamps(t)
	path := writeExe(t, "executable bytes")
	cacheDir := t.TempDir()
	hashes := countHashes(t)
	for range 3 {
		if got := digestOf(t, path, cacheDir, time.Now); got != sha("executable bytes") {
			t.Fatalf("digest = %q", got)
		}
	}
	if *hashes != 3 {
		t.Fatalf("freshly written executable hashed %d times over 3 calls, want 3", *hashes)
	}
	if entries, _ := os.ReadDir(cacheDir); len(entries) != 0 {
		t.Fatalf("recent executable left %d cache entries", len(entries))
	}
}

func TestExecutableDigestPrunesOldEntries(t *testing.T) {
	requireStamps(t)
	cacheDir := t.TempDir()
	stale := filepath.Join(cacheDir, "stale")
	if err := os.WriteFile(stale, []byte(sha("x")), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-90 * 24 * time.Hour)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatal(err)
	}
	path := writeExe(t, "executable bytes")
	digestOf(t, path, cacheDir, inAnHour)
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("stale entry survived: %v", err)
	}
	if entries, _ := os.ReadDir(cacheDir); len(entries) != 1 {
		t.Fatalf("want exactly the fresh entry, have %d", len(entries))
	}
}
