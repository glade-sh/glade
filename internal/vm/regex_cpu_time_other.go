//go:build !unix

package vm

import "time"

// regexProcessCPUTime is unavailable here; regex timeouts then fall back to
// wall-clock classification only.
func regexProcessCPUTime() (time.Duration, bool) {
	return 0, false
}
