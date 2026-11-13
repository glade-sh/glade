//go:build darwin || linux

package filestamp

import (
	"os"
	"sync"
	"syscall"
	"time"
)

// A stamp stands in for content only on filesystems that maintain a real
// ctime (one that follows every content change and cannot be set from user
// space). exFAT, FAT, NTFS drivers, network and FUSE mounts often report
// ctime as mtime, so restoring the mtime would restore the whole stamp. The
// verdict is cached per device for deviceVerdictTTL only: a long-lived process
// (the test daemon) must not keep trusting a device number that was unmounted
// and reused by another file system.
const deviceVerdictTTL = 30 * time.Second

type deviceVerdict struct {
	trusted bool
	at      time.Time
}

var (
	trustedDevices sync.Map // uint64 -> deviceVerdict
	verdictClock   = time.Now
)

func fromInfo(info os.FileInfo, trusted func() bool) (Stamp, bool) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.Mode().IsRegular() {
		return Stamp{}, false
	}
	dev := uint64(st.Dev) // #nosec G115 -- identity only.
	now := verdictClock()
	cached, known := trustedDevices.Load(dev)
	verdict, _ := cached.(deviceVerdict)
	if !known || now.Sub(verdict.at) >= deviceVerdictTTL {
		verdict = deviceVerdict{trusted: trusted(), at: now}
		trustedDevices.Store(dev, verdict)
	}
	if !verdict.trusted {
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
