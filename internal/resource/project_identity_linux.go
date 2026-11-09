package resource

import (
	"fmt"
	"golang.org/x/sys/unix"
)

func projectDirectoryIdentity(path string) string {
	var stat unix.Statx_t
	if unix.Statx(unix.AT_FDCWD, path, 0, unix.STATX_INO|unix.STATX_BTIME, &stat) != nil || stat.Mask&unix.STATX_BTIME == 0 {
		return ""
	}
	return fmt.Sprintf("linux:%d:%d:%d:%d:%d", stat.Dev_major, stat.Dev_minor, stat.Ino, stat.Btime.Sec, stat.Btime.Nsec)
}
