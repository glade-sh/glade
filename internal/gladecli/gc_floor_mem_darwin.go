//go:build darwin

package gladecli

import "golang.org/x/sys/unix"

// hostMemoryBudget returns physical memory, or 0 when it cannot be read.
func hostMemoryBudget() uint64 {
	total, err := unix.SysctlUint64("hw.memsize")
	if err != nil {
		return 0
	}
	return total
}
