//go:build darwin || linux

package filestamp

import (
	"os"
	"syscall"
)

func fromInfo(info os.FileInfo) (Stamp, bool) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.Mode().IsRegular() {
		return Stamp{}, false
	}
	return Stamp{
		Dev:   uint64(st.Dev), // #nosec G115 -- identity only.
		Ino:   st.Ino,
		Size:  st.Size,
		Mtime: mtimeNanos(st),
		Ctime: ctimeNanos(st),
	}, true
}
