//go:build unix

package vm

import (
	"syscall"
	"time"
)

// regexProcessCPUTime returns this process's user plus system CPU time.
func regexProcessCPUTime() (time.Duration, bool) {
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		return 0, false
	}
	return time.Duration(usage.Utime.Nano() + usage.Stime.Nano()), true
}
