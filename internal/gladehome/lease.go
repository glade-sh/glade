package gladehome

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// LeaseManagedRoot keeps an installed toolchain stable while its files are used.
// Explicit source checkouts outside the managed share do not need its lock.
func LeaseManagedRoot(root string) (func(), error) {
	return leaseManagedRoot(root, os.OpenFile)
}

func leaseManagedRoot(root string, open func(string, int, os.FileMode) (*os.File, error)) (func(), error) {
	if !sameDirectory(root, UserShareDir()) && hasGoMod(root) {
		return func() {}, nil
	}
	file, release, err := acquireReadLease(root, open, lockInstallFile)
	if err != nil {
		return nil, fmt.Errorf("acquire toolchain read lease: %w", err)
	}
	if file == nil {
		// The directory may have appeared after the first open. Reacquire
		// against the selected root before allowing a managed reader through.
		release()
		file, release, err = acquireReadLease(root, open, lockInstallFile)
		if err != nil {
			return nil, fmt.Errorf("acquire toolchain read lease: %w", err)
		}
		if file == nil {
			release()
			return nil, fmt.Errorf("toolchain install lock unavailable")
		}
	}
	if err := rejectLegacyShellTransaction(root); err != nil {
		release()
		return nil, err
	}
	if _, ok := validateRoot(root); !ok {
		release()
		return nil, fmt.Errorf("toolchain changed while acquiring read lease")
	}
	return release, nil
}

// LeaseRoot selects the active root and retains its install lock until release.
func LeaseRoot() (string, func(), error) { return leaseActiveRoot(false) }

// LeaseEnsuredRoot installs a missing managed toolchain before leasing it.
func LeaseEnsuredRoot() (string, func(), error) { return leaseActiveRoot(true) }

// A selected root was already validated. Avoid filesystem identity checks on
// the common path where it is spelled exactly like the managed share.
func selectedRootIsShare(root string) bool {
	return filepath.Clean(root) == filepath.Clean(UserShareDir()) || sameDirectory(root, UserShareDir())
}

func leaseActiveRoot(ensure bool) (string, func(), error) {
	return leaseActiveRootWith(ensure, os.OpenFile, lockInstallFile)
}

func leaseActiveRootWith(ensure bool, open func(string, int, os.FileMode) (*os.File, error), lock func(*os.File, bool) error) (string, func(), error) {
	return leaseActiveRootContext(context.Background(), ensure, open, lock, false)
}

func leaseActiveRootContext(ctx context.Context, ensure bool, open func(string, int, os.FileMode) (*os.File, error), lock func(*os.File, bool) error, bounded bool) (string, func(), error) {
	if root, ok := explicitToolchainRoot(); ok && !selectedRootIsShare(root) && hasGoMod(root) {
		return root, func() {}, nil
	}
	if _, ok := explicitToolchainRoot(); !ok {
		if err := requireManagedShare(); err != nil {
			return "", nil, err
		}
	}
	dir := selectedInstallDir()
	file, release, err := acquireReadLeaseContext(ctx, dir, open, lock, bounded)
	if err != nil {
		return "", nil, err
	}
	var root string
	if ensure {
		if explicit, ok := explicitToolchainRoot(); ok {
			root = explicit
		} else if managed, ok := userShareRoot(); ok {
			root = managed
		} else {
			err = fmt.Errorf("managed toolchain not installed")
		}
	} else {
		root, _, err = findActiveToolchainRoot()
	}
	if err != nil {
		release()
		if ensure {
			if _, installErr := EnsureRoot(); installErr != nil {
				return "", nil, installErr
			}
			return leaseActiveRoot(true)
		}
		return "", nil, err
	}
	if !selectedRootIsShare(root) && hasGoMod(root) {
		release()
		return root, func() {}, nil
	}
	if filepath.Clean(root) != filepath.Clean(dir) && !sameDirectory(root, dir) {
		release()
		if bounded {
			return "", nil, fmt.Errorf("toolchain changed during status check; retry")
		}
		replacement, err := leaseManagedRoot(root, open)
		if err != nil {
			return "", nil, err
		}
		return root, replacement, nil
	}
	if file == nil {
		// A first install may have created the selected root after the
		// initial missing-directory probe. Never return it without a fence.
		release()
		file, release, err = acquireReadLeaseContext(ctx, root, open, lock, bounded)
		if err != nil {
			return "", nil, err
		}
		if file == nil {
			release()
			return "", nil, fmt.Errorf("toolchain install lock unavailable")
		}
		selected, _, selectErr := findActiveToolchainRoot()
		if selectErr != nil || (filepath.Clean(selected) != filepath.Clean(root) && !sameDirectory(selected, root)) {
			release()
			return "", nil, fmt.Errorf("toolchain changed while acquiring read lease")
		}
	}
	if err := rejectLegacyShellTransaction(root); err != nil {
		release()
		return "", nil, err
	}
	return root, release, nil
}
