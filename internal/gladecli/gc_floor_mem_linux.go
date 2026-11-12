//go:build linux

package gladecli

import "os"

// hostMemoryBudget returns the smaller of physical memory and the cgroup
// memory limit, or 0 when neither can be read.
func hostMemoryBudget() uint64 {
	var budget uint64
	if data, err := os.ReadFile("/proc/meminfo"); err == nil {
		budget = parseMeminfoTotal(string(data))
	}
	for _, path := range []string{"/sys/fs/cgroup/memory.max", "/sys/fs/cgroup/memory/memory.limit_in_bytes"} {
		if data, err := os.ReadFile(path); err == nil {
			if limit := parseCgroupLimit(string(data)); limit != 0 && (budget == 0 || limit < budget) {
				budget = limit
			}
		}
	}
	return budget
}
