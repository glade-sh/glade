package gladehome

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// POSIX record locks belong to a process: closing any descriptor for the
// lock file releases that process's locks. Readers of one root therefore share
// one descriptor until their last lease ends; writers take installMu exclusively.
type sharedReadLease struct {
	mu       sync.Mutex
	users    int // protected by readLeaseRegistry.mu, includes waiters
	refs     int // protected by mu
	file     *os.File
	path     string
	dirInfo  os.FileInfo // identity fields protected by readLeaseRegistry.mu
	lockInfo os.FileInfo
}

var readLeaseRegistry = struct {
	mu    sync.Mutex
	roots map[*sharedReadLease]struct{}
}{roots: make(map[*sharedReadLease]struct{})}

func readLeaseKey(dir string) string {
	absolute, err := filepath.Abs(dir)
	if err != nil {
		absolute = filepath.Clean(dir)
	}
	return absolute
}

func acquireReadLease(dir string, open func(string, int, os.FileMode) (*os.File, error), lock func(*os.File, bool) error) (*os.File, func(), error) {
	installMu.RLock()
	key := readLeaseKey(dir)
	dirInfo, _ := os.Stat(dir)
	lockInfo, _ := os.Stat(filepath.Join(dir, ".install.lock"))
	readLeaseRegistry.mu.Lock()
	var entry *sharedReadLease
	for candidate := range readLeaseRegistry.roots {
		if dirInfo != nil && candidate.dirInfo != nil && os.SameFile(dirInfo, candidate.dirInfo) &&
			candidate.lockInfo != nil && (lockInfo == nil || !os.SameFile(lockInfo, candidate.lockInfo)) {
			// The initial stat may have preceded the first reader creating it.
			lockInfo, _ = os.Stat(filepath.Join(dir, ".install.lock"))
			if lockInfo == nil || !os.SameFile(lockInfo, candidate.lockInfo) {
				readLeaseRegistry.mu.Unlock()
				installMu.RUnlock()
				return nil, nil, fmt.Errorf("toolchain install lock changed during read lease")
			}
		}
		if lockInfo != nil && candidate.lockInfo != nil && os.SameFile(lockInfo, candidate.lockInfo) ||
			dirInfo != nil && candidate.dirInfo != nil && os.SameFile(dirInfo, candidate.dirInfo) ||
			(dirInfo == nil || candidate.dirInfo == nil) && key == candidate.path {
			entry = candidate
			break
		}
	}
	if entry == nil {
		entry = &sharedReadLease{path: key, dirInfo: dirInfo, lockInfo: lockInfo}
		readLeaseRegistry.roots[entry] = struct{}{}
	}
	entry.users++
	readLeaseRegistry.mu.Unlock()
	retire := func() {
		readLeaseRegistry.mu.Lock()
		entry.users--
		if entry.users == 0 {
			delete(readLeaseRegistry.roots, entry)
		}
		readLeaseRegistry.mu.Unlock()
		installMu.RUnlock()
	}
	entry.mu.Lock()
	if entry.refs == 0 {
		// Publish a newly created lock's identity before another root can
		// discover a hard link to it. Never hold the registry mutex while
		// waiting for the operating system lock.
		trackedOpen := func(path string, flags int, mode os.FileMode) (*os.File, error) {
			readLeaseRegistry.mu.Lock()
			defer readLeaseRegistry.mu.Unlock()
			file, err := open(path, flags, mode)
			if err == nil {
				entry.lockInfo, _ = file.Stat()
			}
			return file, err
		}
		file, err := readInstallLockAtWith(dir, trackedOpen, lock)
		if err != nil {
			entry.mu.Unlock()
			retire()
			return nil, nil, err
		}
		entry.file = file
	}
	entry.refs++
	file := entry.file
	entry.mu.Unlock()
	var once sync.Once
	release := func() {
		once.Do(func() {
			entry.mu.Lock()
			entry.refs--
			if entry.refs == 0 && entry.file != nil {
				_ = entry.file.Close()
				entry.file = nil
			}
			entry.mu.Unlock()
			retire()
		})
	}
	return file, release, nil
}
