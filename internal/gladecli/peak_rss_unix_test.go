//go:build !windows

package gladecli

import (
	"runtime"
	"syscall"
)

// peakRSSBytes is the process's peak resident set size.
func peakRSSBytes() int64 {
	var usage syscall.Rusage
	if syscall.Getrusage(syscall.RUSAGE_SELF, &usage) != nil {
		return 0
	}
	if runtime.GOOS == "darwin" {
		return int64(usage.Maxrss) // bytes
	}
	return int64(usage.Maxrss) * 1024 // kilobytes
}
