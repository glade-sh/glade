//go:build unix

package gladehome

import (
	"os"
	"syscall"
)

func lockInstallFile(file *os.File, exclusive bool) error {
	lock := syscall.Flock_t{Type: syscall.F_RDLCK}
	if exclusive {
		lock.Type = syscall.F_WRLCK
	}
	for {
		err := syscall.FcntlFlock(file.Fd(), syscall.F_SETLKW, &lock)
		if err != syscall.EINTR {
			return err
		}
	}
}

func tryLockInstallFile(file *os.File, exclusive bool) error {
	lock := syscall.Flock_t{Type: syscall.F_RDLCK}
	if exclusive {
		lock.Type = syscall.F_WRLCK
	}
	return syscall.FcntlFlock(file.Fd(), syscall.F_SETLK, &lock)
}

func isInstallLockBusy(err error) bool {
	return err == syscall.EAGAIN || err == syscall.EACCES || err == syscall.EINTR
}
