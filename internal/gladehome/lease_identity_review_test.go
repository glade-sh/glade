//go:build unix

package gladehome

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func identityFence(t *testing.T, path string) string {
	t.Helper()
	out, err := exec.Command("python3", "-c", `import fcntl,sys
f=open(sys.argv[1], 'r+')
try: fcntl.lockf(f, fcntl.LOCK_EX | fcntl.LOCK_NB); print('free')
except BlockingIOError: print('held')`, path).CombinedOutput()
	if err != nil {
		t.Fatalf("probe: %v: %s", err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestReadLeaseFilesystemIdentity(t *testing.T) {
	for _, mode := range []string{"case", "hardlink", "distinct"} {
		t.Run(mode, func(t *testing.T) {
			base := t.TempDir()
			firstDir := filepath.Join(base, "First")
			secondDir := filepath.Join(base, "Second")
			if err := os.Mkdir(firstDir, 0755); err != nil {
				t.Fatal(err)
			}
			if mode == "case" {
				secondDir = filepath.Join(base, "first")
				a, err := os.Stat(firstDir)
				if err != nil {
					t.Fatal(err)
				}
				b, err := os.Stat(secondDir)
				if err != nil || !os.SameFile(a, b) {
					t.Skip("filesystem is case-sensitive")
				}
			} else if err := os.Mkdir(secondDir, 0755); err != nil {
				t.Fatal(err)
			}
			firstLock := filepath.Join(firstDir, ".install.lock")
			secondLock := filepath.Join(secondDir, ".install.lock")
			if err := os.WriteFile(firstLock, nil, 0600); err != nil {
				t.Fatal(err)
			}
			if mode == "hardlink" {
				if err := os.Link(firstLock, secondLock); err != nil {
					t.Fatal(err)
				}
			} else if mode == "distinct" {
				if err := os.WriteFile(secondLock, nil, 0600); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("XDG_DATA_HOME", base)
			t.Setenv("GLADE_HOME", "")
			t.Setenv("GLADE_ROOT", "")
			_, first, err := acquireReadLease(firstDir, os.OpenFile, lockInstallFile)
			if err != nil {
				t.Fatal(err)
			}
			_, second, err := acquireReadLease(secondDir, os.OpenFile, lockInstallFile)
			if err != nil {
				first()
				t.Fatal(err)
			}
			first()
			defer second()
			if got := identityFence(t, secondLock); got != "held" {
				t.Fatalf("remaining lease fence = %s", got)
			}
			if mode == "distinct" {
				if got := identityFence(t, firstLock); got != "free" {
					t.Fatalf("unrelated released lock = %s", got)
				}
			}
		})
	}
}

func TestReadLeaseReplacedLockRejected(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Dir(dir))
	t.Setenv("GLADE_HOME", "")
	t.Setenv("GLADE_ROOT", "")
	path := filepath.Join(dir, ".install.lock")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	_, release, err := acquireReadLease(dir, os.OpenFile, lockInstallFile)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if err := os.Rename(path, path+".old"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	_, second, err := acquireReadLease(dir, os.OpenFile, lockInstallFile)
	if err == nil {
		second()
		t.Fatal("accepted a replacement lock while the original lock was leased")
	}
}
