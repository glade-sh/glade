package gladecli

import (
	"os"
	"path"
	"path/filepath"
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

// cgroupEffectiveLimit returns the smallest memory limit on the process's
// cgroup or any ancestor, or 0 when none is set or readable. root is the cgroup
// mount directory (/sys/fs/cgroup) and selfCgroup is /proc/self/cgroup. v2
// (0::/path) reads memory.max under root; v1 (N:memory:/path) reads
// memory.limit_in_bytes under root/memory. A cgroup path that does not exist
// under the mount (a namespaced view) is walked upward until one does.
func cgroupEffectiveLimit(root, selfCgroup string) uint64 {
	var limit uint64
	consider := func(v uint64) {
		if v != 0 && (limit == 0 || v < limit) {
			limit = v
		}
	}
	for _, line := range strings.Split(selfCgroup, "\n") {
		parts := strings.SplitN(strings.TrimSpace(line), ":", 3)
		if len(parts) != 3 {
			continue
		}
		var base, file string
		switch {
		case parts[0] == "0" && parts[1] == "":
			base, file = root, "memory.max"
		case containsController(parts[1], "memory"):
			base, file = filepath.Join(root, "memory"), "memory.limit_in_bytes"
		default:
			continue
		}
		rel := path.Clean("/" + parts[2])
		for {
			if data, err := os.ReadFile(filepath.Join(base, filepath.FromSlash(rel), file)); err == nil {
				consider(parseCgroupLimit(string(data)))
			}
			if rel == "/" {
				break
			}
			rel = path.Dir(rel)
		}
	}
	return limit
}

func containsController(list, name string) bool {
	for _, c := range strings.Split(list, ",") {
		if c == name {
			return true
		}
	}
	return false
}
