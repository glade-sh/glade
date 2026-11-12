package apextest

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/filestamp"
	"github.com/glade-sh/glade/internal/typesys"
)

// trustedClock makes every file look old enough to trust, as in a daemon that
// validates sources edited long ago.
func trustedClock(t *testing.T) {
	t.Helper()
	resetFileDigestTrust()
	oldNow, oldHook := fileDigestNow, fileDigestReadHook
	fileDigestNow = func() time.Time { return time.Now().Add(time.Hour) }
	t.Cleanup(func() {
		fileDigestNow, fileDigestReadHook = oldNow, oldHook
		resetFileDigestTrust()
	})
}

func countReads(t *testing.T, path string) *int {
	t.Helper()
	reads := new(int)
	fileDigestReadHook = func(p string) {
		if p == path {
			*reads++
		}
	}
	return reads
}

// ctimeTick separates two writes so a coarse filesystem clock cannot give them one ctime.
func ctimeTick() { time.Sleep(30 * time.Millisecond) }

// skipWithoutStamps skips tests of the trusted path on file systems whose
// ctime cannot be relied on (exFAT, network mounts); there every validation
// hashes, which the other tests cover.
func skipWithoutStamps(t *testing.T) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "probe")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := filestamp.Stat(path); !ok {
		t.Fatal("file system without a trusted ctime")
	}
}

func TestCurrentFileDigestMatchesSHA256(t *testing.T) {
	trustedClock(t)
	dir := t.TempDir()
	for name, content := range map[string][]byte{
		"empty":  {},
		"small":  []byte("public class A {}"),
		"binary": {0, 1, 2, 255, 254},
		"large":  bytes.Repeat([]byte("0123456789abcdef"), 1<<16),
	} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatal(err)
		}
		want := sha256.Sum256(content)
		for pass := range 2 {
			got, err := currentFileDigest(path)
			if err != nil || got != want {
				t.Fatalf("%s pass %d: digest %x err %v, want %x", name, pass, got, err, want)
			}
		}
	}
	if _, err := currentFileDigest(filepath.Join(dir, "missing")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing file error = %v, want not-exist", err)
	}
	if _, err := currentFileDigest(dir); err == nil {
		t.Fatal("directory hashed without an error")
	}
}

func TestCurrentFileDigestSkipsRereadOfUnchangedOldFile(t *testing.T) {
	skipWithoutStamps(t)
	trustedClock(t)
	path := filepath.Join(t.TempDir(), "A.cls")
	if err := os.WriteFile(path, []byte("public class A {}"), 0o600); err != nil {
		t.Fatal(err)
	}
	reads := countReads(t, path)
	for range 5 {
		if _, err := currentFileDigest(path); err != nil {
			t.Fatal(err)
		}
	}
	if *reads != 1 {
		t.Fatalf("file read %d times for 5 validations of an unchanged old file, want 1", *reads)
	}
}

func TestCurrentFileDigestRehashesRecentlyModifiedFile(t *testing.T) {
	resetFileDigestTrust()
	t.Cleanup(resetFileDigestTrust)
	oldHook := fileDigestReadHook
	t.Cleanup(func() { fileDigestReadHook = oldHook })
	path := filepath.Join(t.TempDir(), "A.cls")
	if err := os.WriteFile(path, []byte("public class A {}"), 0o600); err != nil {
		t.Fatal(err)
	}
	reads := countReads(t, path)
	for range 3 {
		if _, err := currentFileDigest(path); err != nil {
			t.Fatal(err)
		}
	}
	if *reads != 3 {
		t.Fatalf("file read %d times, want 3: a file modified inside the trust window must always be re-hashed", *reads)
	}
}

func TestCurrentFileDigestSeesEveryKindOfEdit(t *testing.T) {
	original := []byte("public class A { Integer x = 1; }")
	sameSize := []byte("public class A { Integer x = 2; }")
	for name, edit := range map[string]func(t *testing.T, path string){
		"rewrite same size": func(t *testing.T, path string) {
			if err := os.WriteFile(path, sameSize, 0o600); err != nil {
				t.Fatal(err)
			}
		},
		"rewrite same size and restore mtime": func(t *testing.T, path string) {
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, sameSize, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
				t.Fatal(err)
			}
		},
		"longer": func(t *testing.T, path string) {
			if err := os.WriteFile(path, append(append([]byte{}, original...), "\n// more"...), 0o600); err != nil {
				t.Fatal(err)
			}
		},
		"atomic replace with same size and mtime": func(t *testing.T, path string) {
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			next := path + ".next"
			if err := os.WriteFile(next, sameSize, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Chtimes(next, info.ModTime(), info.ModTime()); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(next, path); err != nil {
				t.Fatal(err)
			}
		},
		"symlink retarget": func(t *testing.T, path string) {
			other := path + ".other"
			if err := os.WriteFile(other, sameSize, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(other, path); err != nil {
				t.Fatalf("symlinks unavailable: %v", err)
			}
		},
		"deleted": func(t *testing.T, path string) {
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			trustedClock(t)
			path := filepath.Join(t.TempDir(), "A.cls")
			if err := os.WriteFile(path, original, 0o600); err != nil {
				t.Fatal(err)
			}
			first, err := currentFileDigest(path)
			if err != nil || first != sha256.Sum256(original) {
				t.Fatalf("first digest %x, %v", first, err)
			}
			ctimeTick()
			edit(t, path)
			got, err := currentFileDigest(path)
			content, readErr := os.ReadFile(path)
			if readErr != nil {
				if err == nil {
					t.Fatalf("deleted file still returned digest %x from the stamp table", got)
				}
				return
			}
			if err != nil || got != sha256.Sum256(content) {
				t.Fatalf("digest after edit = %x, %v; want %x (stale stamp served)", got, err, sha256.Sum256(content))
			}
			if got == first {
				t.Fatal("digest did not change after the edit")
			}
		})
	}
}

func TestValidatorsDetectEditAfterTrustedPass(t *testing.T) {
	skipWithoutStamps(t)
	trustedClock(t)
	dir := t.TempDir()
	source := filepath.Join(dir, "A.cls")
	meta := source + "-meta.xml"
	original := []byte("public class A { Integer x = 1; }")
	metaOriginal := []byte("<ApexClass><apiVersion>67.0</apiVersion></ApexClass>")
	if err := os.WriteFile(source, original, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(meta, metaOriginal, 0o600); err != nil {
		t.Fatal(err)
	}
	sourceDigest := sha256.Sum256(original)
	expectedMeta := typesys.ApexMetadataInput{Present: true, Digest: sha256.Sum256(metaOriginal)}
	cache := newSourceCache()
	cache.capturedFiles = map[string][sha256.Size]byte{source: sourceDigest}
	cache.capturedMetadata = map[string]typesys.ApexMetadataInput{meta: expectedMeta}

	check := func() map[string]error {
		return map[string]error{
			"raw generation": cache.validateCapturedSourceGenerationRaw(),
			"metadata input": validateApexMetadataInput(meta, expectedMeta),
		}
	}
	reads := countReads(t, source)
	for name, err := range check() {
		if err != nil {
			t.Fatalf("%s rejected unchanged files: %v", name, err)
		}
	}
	if *reads != 1 {
		t.Fatalf("first validation read the source %d times, want 1", *reads)
	}
	for range 3 {
		check()
	}
	if *reads != 1 {
		t.Fatalf("source read %d times in total across 4 validations of an unchanged old file, want 1", *reads)
	}

	ctimeTick()
	if err := os.WriteFile(source, []byte("public class A { Integer x = 2; }"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := cache.validateCapturedSourceGenerationRaw(); err == nil {
		t.Fatal("raw generation accepted an edited source")
	}
	ctimeTick()
	if err := os.WriteFile(meta, []byte("<ApexClass><apiVersion>66.0</apiVersion></ApexClass>"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateApexMetadataInput(meta, expectedMeta); err == nil {
		t.Fatal("metadata validation accepted an edited companion")
	}
}
