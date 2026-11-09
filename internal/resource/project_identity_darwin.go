package resource

import (
	"fmt"
	"syscall"
)

func projectDirectoryIdentity(path string) string {
	var stat syscall.Stat_t
	if syscall.Stat(path, &stat) != nil || stat.Birthtimespec.Sec == 0 {
		return ""
	}
	return fmt.Sprintf("darwin:%d:%d:%d:%d", stat.Dev, stat.Ino, stat.Birthtimespec.Sec, stat.Birthtimespec.Nsec)
}
