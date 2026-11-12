//go:build darwin || linux

package filestamp

import (
	"os"
	"sync"
	"syscall"
)

// A stamp stands in for content only on filesystems that maintain a real
// ctime (one that follows every content change and cannot be set from user
// space). exFAT, FAT, NTFS drivers, network and FUSE mounts often report
// ctime as mtime, so restoring the mtime would restore the whole stamp. The
// verdict is cached per device.
var trustedDevices sync.Map // uint64 -> bool

func fromInfo(info os.FileInfo, trusted func() bool) (Stamp, bool) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.Mode().IsRegular() {
		return Stamp{}, false
	}
	dev := uint64(st.Dev) // #nosec G115 -- identity only.
	verdict, known := trustedDevices.Load(dev)
	if !known {
		verdict = trusted()
		trustedDevices.Store(dev, verdict)
	}
	if !verdict.(bool) {
		return Stamp{}, false
	}
	return Stamp{
		Dev:   dev,
		Ino:   st.Ino,
		Size:  st.Size,
		Mtime: mtimeNanos(st),
		Ctime: ctimeNanos(st),
	}, true
}

func statStamp(path string, info os.FileInfo) (Stamp, bool) {
	return fromInfo(info, func() bool {
		var fs syscall.Statfs_t
		return syscall.Statfs(path, &fs) == nil && trustedFilesystem(&fs)
	})
}

func fstatStamp(file *os.File, info os.FileInfo) (Stamp, bool) {
	return fromInfo(info, func() bool {
		var fs syscall.Statfs_t
		return syscall.Fstatfs(int(file.Fd()), &fs) == nil && trustedFilesystem(&fs) // #nosec G115 -- descriptor fits int.
	})
}
