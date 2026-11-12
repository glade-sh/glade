//go:build darwin || linux

package apextest

import (
	"os"
	"syscall"
)

func stampFromInfo(info os.FileInfo) (fileStamp, bool) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.Mode().IsRegular() {
		return fileStamp{}, false
	}
	return fileStamp{
		dev:   uint64(st.Dev), // #nosec G115 -- identity only.
		ino:   st.Ino,
		size:  st.Size,
		mtime: statMtimeNanos(st),
		ctime: statCtimeNanos(st),
	}, true
}

func statFileStamp(path string) (fileStamp, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return fileStamp{}, false
	}
	return stampFromInfo(info)
}

func fstatFileStamp(file *os.File) (fileStamp, bool) {
	info, err := file.Stat()
	if err != nil {
		return fileStamp{}, false
	}
	return stampFromInfo(info)
}
