// Package filestamp identifies one on-disk version of a regular file by device,
// inode, size, mtime and ctime. ctime cannot be set from user space, so
// rewriting a file and restoring its mtime still changes the stamp.
//
// Known limits, accepted and not detectable from a stat:
//   - Writes through a shared writable mapping (mmap MAP_SHARED) may change
//     content without updating mtime or ctime. A stamp is trusted for build
//     artifacts that are written then executed or read, not for files another
//     process maps writable.
//   - overlayfs is trusted on the strength of the overlay's own statfs type.
//     Its lower layer may be a file system without a real ctime, and statfs
//     does not name it. A file served from such a lower layer could keep its
//     stamp across a content change. Container images normally keep their
//     lower layers immutable.
//   - The per-device trust verdict is re-checked every 30 seconds, not on every
//     call. A device number reused by another file system inside that window is
//     still judged by the old verdict.
package filestamp

import (
	"os"
	"time"
)

// Stamp identifies one version of a file.
type Stamp struct {
	Dev, Ino     uint64
	Size         int64
	Mtime, Ctime int64
}

// Stat stamps the file at path, following symlinks. ok is false for a missing
// or non-regular file, and on file systems or platforms without a trustworthy
// ctime (see stamp_unix.go); callers then hash.
func Stat(path string) (Stamp, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return Stamp{}, false
	}
	return statStamp(path, info)
}

// Fstat stamps an open file.
func Fstat(file *os.File) (Stamp, bool) {
	info, err := file.Stat()
	if err != nil {
		return Stamp{}, false
	}
	return fstatStamp(file, info)
}

// OlderThan reports whether the newer of mtime and ctime is at least age
// before now. Filesystem timestamps can be coarser than the gap between two
// writes, so a stamp taken inside this window must never stand in for hashing
// (the "racily clean" rule).
func (s Stamp) OlderThan(now time.Time, age time.Duration) bool {
	return now.UnixNano()-max(s.Mtime, s.Ctime) >= int64(age)
}
