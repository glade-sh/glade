package apextest

import (
	"os"
	"strings"
)

// childEnvWithoutShardSelector returns this process's environment without the
// GLADE_APEXTEST_SHARD selector. Tests that re-exec the test binary with an
// explicit -test.run must pass it to their children: a child that inherits the
// selector narrows -test.run to the parent's shard and, whenever the requested
// test belongs to another shard, runs nothing and exits 0. The selector is a
// property of the parent's process, not of the child's test selection.
func childEnvWithoutShardSelector() []string {
	environ := os.Environ()
	filtered := make([]string, 0, len(environ))
	for _, entry := range environ {
		if strings.HasPrefix(entry, shardSelectorEnv+"=") {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}
