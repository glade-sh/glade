package gladehome

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// lockInstall requires installMu. Closing the returned file releases the lock,
// including when an installer exits unexpectedly.
func lockInstall() (*os.File, error) {
	if err := requireManagedShare(); err != nil {
		return nil, err
	}
	dir := UserShareDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create toolchain install directory: %w", err)
	}
	// Keep the lock file in place so waiting installers always lock the same inode.
	file, err := os.OpenFile(filepath.Join(dir, ".install.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open toolchain install lock: %w", err)
	}
	if err := lockInstallFile(file, true); err != nil {
		file.Close()
		return nil, fmt.Errorf("lock toolchain install: %w", err)
	}
	return file, nil
}

// readInstallLock requires installMu. Readers create the lock when absent in
// an existing writable installation, then hold it across validation.
func readInstallLock() (*os.File, error) {
	return readInstallLockAt(selectedInstallDir())
}

func readInstallLockAt(dir string) (*os.File, error) {
	return readInstallLockAtWith(dir, os.OpenFile, lockInstallFile)
}

func readInstallLockAtWith(dir string, open func(string, int, os.FileMode) (*os.File, error), lock func(*os.File, bool) error) (*os.File, error) {
	return readInstallLockAtWithMode(dir, open, lock, false)
}

func readInstallLockAtWithMode(dir string, open func(string, int, os.FileMode) (*os.File, error), lock func(*os.File, bool) error, noRecovery bool) (*os.File, error) {
	path := filepath.Join(dir, ".install.lock")
	file, err := open(path, os.O_RDONLY, 0)
	if os.IsNotExist(err) {
		file, err = open(path, os.O_CREATE|os.O_RDONLY, 0600)
	}
	if os.IsNotExist(err) || errors.Is(err, syscall.ENOTDIR) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open toolchain install lock for reading: %w", err)
	}
	if err := lock(file, false); err != nil {
		file.Close()
		return nil, fmt.Errorf("wait for toolchain install: %w", err)
	}
	if noRecovery {
		marker, markerErr := readInstallMarker(file)
		if markerErr != nil || marker == 'P' {
			file.Close()
			if markerErr != nil {
				return nil, markerErr
			}
			return nil, fmt.Errorf("toolchain recovery busy; retry after installation")
		}
	}
	file, err = recoverReadLockAt(file, dir)
	if err != nil {
		return nil, err
	}
	if err := rejectLegacyShellTransaction(dir); err != nil {
		file.Close()
		return nil, err
	}
	return file, nil
}

// Older installers recorded a custom destination transaction on the XDG lock.
// Check the target itself while holding its new lock, including across XDG homes.
func rejectLegacyShellTransaction(dir string) error {
	if filepath.Clean(dir) == filepath.Clean(UserShareDir()) || sameDirectory(dir, UserShareDir()) {
		return nil
	}
	return rejectPendingShellTransactions(dir)
}

// The shell transaction directory can precede its lock-file marker. Writers
// must inspect it while holding the exclusive lock before modifying assets.
func rejectPendingShellTransactions(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("inspect toolchain recovery state: %w", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".install-txn.") {
			return fmt.Errorf("interrupted installer transaction; rerun the installer to recover")
		}
	}
	return nil
}

// The existing lock file also holds a one-byte transaction marker. Readers
// check it under the shared lock without opening or scanning other paths.
func readInstallMarker(file *os.File) (byte, error) {
	var marker [1]byte
	n, err := file.ReadAt(marker[:], 0)
	if err != nil && !errors.Is(err, io.EOF) {
		return 0, err
	}
	if n == 0 {
		return 0, nil
	}
	return marker[0], nil
}

func writeInstallMarker(file *os.File, marker byte) error {
	if err := file.Truncate(0); err != nil {
		return err
	}
	if marker != 0 {
		if _, err := file.WriteAt([]byte{marker}, 0); err != nil {
			return err
		}
	}
	return file.Sync()
}

// recoverExclusive requires the exclusive install lock and clears the marker
// only after all interrupted source replacements have been recovered.
func recoverExclusive(file *os.File) error {
	if err := rejectPendingShellTransactions(UserShareDir()); err != nil {
		return err
	}
	marker, err := readInstallMarker(file)
	if err != nil {
		return err
	}
	switch marker {
	case 'S':
		return fmt.Errorf("interrupted installer transaction; rerun the installer to recover")
	case 'P':
		if err := recoverSourceInstalls(UserShareDir()); err != nil {
			return err
		}
		return writeInstallMarker(file, 0)
	default:
		return nil
	}
}

// recoverReadLock owns file. It upgrades a pending source recovery to an
// exclusive lock, then returns the still-held descriptor.
func recoverReadLock(file *os.File) (*os.File, error) {
	return recoverReadLockAt(file, UserShareDir())
}

func recoverReadLockAt(file *os.File, dir string) (*os.File, error) {
	marker, err := readInstallMarker(file)
	if err != nil {
		file.Close()
		return nil, err
	}
	if marker == 'S' {
		file.Close()
		return nil, fmt.Errorf("interrupted installer transaction; rerun the installer to recover")
	}
	if marker != 'P' {
		return file, nil
	}
	if !sameDirectory(dir, UserShareDir()) {
		file.Close()
		return nil, fmt.Errorf("pending source recovery in non-default toolchain directory")
	}
	file.Close()
	file, err = lockInstall()
	if err != nil {
		return nil, err
	}
	if err := recoverExclusive(file); err != nil {
		file.Close()
		return nil, fmt.Errorf("recover source toolchain install: %w", err)
	}
	return file, nil
}
