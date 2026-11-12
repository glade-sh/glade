// Package filestamp identifies one on-disk version of a regular file by device,
// inode, size, mtime and ctime. ctime cannot be set from user space, so
// rewriting a file and restoring its mtime still changes the stamp.
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
