//go:build linux

package gladecli

import "os"

// hostMemoryBudget returns the smaller of physical memory and the effective
// cgroup memory limit of this process, or 0 when neither can be read.
func hostMemoryBudget() uint64 {
	var budget uint64
	if data, err := os.ReadFile("/proc/meminfo"); err == nil {
		budget = parseMeminfoTotal(string(data))
	}
	if self, err := os.ReadFile("/proc/self/cgroup"); err == nil {
		if limit := cgroupEffectiveLimit("/sys/fs/cgroup", string(self)); limit != 0 && (budget == 0 || limit < budget) {
			budget = limit
		}
	}
	return budget
}
