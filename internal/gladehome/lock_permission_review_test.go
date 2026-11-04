package gladehome

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestReadInstallLockRejectsInaccessibleLock(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_DATA_HOME", base)
	dir := filepath.Join(base, "glade")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".install.lock"), nil, 0000); err != nil {
		t.Fatal(err)
	}
	f, err := readInstallLock()
	if f != nil {
		f.Close()
	}
	if err == nil {
		t.Fatal("unreadable existing writer lock was accepted")
	}
}

func TestLeaseManagedRootRejectsPermissionDenied(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	release, err := leaseManagedRoot(root, func(path string, flag int, mode os.FileMode) (*os.File, error) {
		return nil, &os.PathError{Op: "open", Path: path, Err: syscall.EACCES}
	})
	if release != nil {
		release()
	}
	if !errors.Is(err, syscall.EACCES) {
		t.Fatalf("lease error = %v, want permission denied", err)
	}
}

func TestReadInstallLockCreatesMissingLock(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_DATA_HOME", base)
	t.Setenv("GLADE_HOME", "")
	t.Setenv("GLADE_ROOT", "")
	if err := os.MkdirAll(filepath.Join(base, "glade"), 0755); err != nil {
		t.Fatal(err)
	}
	f, err := readInstallLock()
	if err != nil {
		t.Fatal(err)
	}
	if f == nil {
		t.Fatal("reader proceeded without creating a lock in an existing writable installation")
	}
	defer f.Close()
	if _, err := os.Stat(filepath.Join(base, "glade", ".install.lock")); err != nil {
		t.Fatal(err)
	}
}

func TestRootRejectsLegacyUnmarkedCustomTransaction(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "custom")
	t.Setenv("XDG_DATA_HOME", filepath.Join(base, "other-xdg"))
	t.Setenv("GLADE_HOME", root)
	t.Setenv("GLADE_ROOT", "")
	for _, rel := range []string{"third_party/lwc/compile.mjs", "third_party/lwc/node_modules/@lwc/compiler/index.js", "lwcruntime/src/shims/wire-adapter.mjs", "lwcruntime/src/shims/lds-cache.mjs", "lwcruntime/src/shell/app.mjs", "lwcruntime/src/slds/slds-loader.mjs", "lwcruntime/src/lightning/button.mjs", ".install-txn.legacy/started"} {
		writeMinimalToolchainFile(t, root, rel)
	}
	if result, err := Root(); err == nil {
		t.Fatalf("Root exposed interrupted legacy transaction at %s", result)
	}
}

func TestReadOnlyCustomLeaseRejectsLegacyTransaction(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	writeMinimalToolchainFile(t, root, ".install-txn.legacy/started")
	release, err := leaseManagedRoot(root, func(path string, flag int, mode os.FileMode) (*os.File, error) {
		return nil, &os.PathError{Op: "open", Path: path, Err: syscall.EROFS}
	})
	if release != nil {
		release()
	}
	if err == nil {
		t.Fatal("read-only lease exposed legacy interrupted transaction")
	}
}
