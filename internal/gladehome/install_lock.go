package gladehome

import (
	"fmt"
	"os"
	"path/filepath"
)

// lockInstall requires installMu. Closing the returned file releases the lock,
// including when an installer exits unexpectedly.
func lockInstall() (*os.File, error) {
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

// readInstallLock requires installMu. Readers never create or write the lock:
// a completed shared installation may live on a read-only filesystem or belong
// to another user. When the lock is unavailable, callers still verify the root.
func readInstallLock() (*os.File, error) {
	file, err := os.Open(filepath.Join(UserShareDir(), ".install.lock"))
	if os.IsNotExist(err) || os.IsPermission(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open toolchain install lock for reading: %w", err)
	}
	if err := lockInstallFile(file, false); err != nil {
		file.Close()
		return nil, fmt.Errorf("wait for toolchain install: %w", err)
	}
	return file, nil
}
