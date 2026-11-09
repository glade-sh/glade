// Package testutil holds test-only helpers shared across packages.
package testutil

import (
	"os"
	"strconv"
	"time"
)

// TimeScaleEnv names the environment variable that scales test-only wait
// budgets. Hosts that run tests under load set it above 1; unset or invalid
// values mean 1.
const TimeScaleEnv = "GLADE_TEST_TIME_SCALE"

// TimeScale returns the test budget multiplier, never below 1.
func TimeScale() float64 {
	scale, err := strconv.ParseFloat(os.Getenv(TimeScaleEnv), 64)
	if err != nil || scale < 1 || scale > 1000 {
		return 1
	}
	return scale
}

// Deadline scales a test-only wait budget by GLADE_TEST_TIME_SCALE. It never
// shortens a budget, and it must not be used for product timeouts.
func Deadline(base time.Duration) time.Duration {
	return time.Duration(float64(base) * TimeScale())
}
