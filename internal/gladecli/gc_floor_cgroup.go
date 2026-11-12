package gladecli

import (
	"strconv"
	"strings"
)

// parseCgroupLimit reads a cgroup v2 memory.max or v1 memory.limit_in_bytes
// value. "max" and the v1 no-limit sentinel (a value near 2^63) mean no limit.
func parseCgroupLimit(text string) uint64 {
	text = strings.TrimSpace(text)
	if text == "" || text == "max" {
		return 0
	}
	limit, err := strconv.ParseUint(text, 10, 64)
	if err != nil || limit >= 1<<62 {
		return 0
	}
	return limit
}

func parseMeminfoTotal(text string) uint64 {
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 3 && fields[0] == "MemTotal:" && fields[2] == "kB" {
			if kb, err := strconv.ParseUint(fields[1], 10, 64); err == nil {
				return kb << 10
			}
		}
	}
	return 0
}
