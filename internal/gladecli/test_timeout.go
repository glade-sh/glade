package gladecli

import (
	"fmt"
	"os"
	"time"

	"github.com/glade-sh/glade/internal/apextest"
)

const defaultTestTimeout = 10 * time.Minute

func resolveTestTimeout(flagValue string, flagSet bool) (time.Duration, string, error) {
	value, source, setting := flagValue, "flag", "--test-timeout"
	if !flagSet {
		var present bool
		value, present = os.LookupEnv("GLADE_TEST_TIMEOUT")
		if !present {
			return defaultTestTimeout, "default", nil
		}
		source, setting = "environment", "GLADE_TEST_TIMEOUT"
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, "", fmt.Errorf("%s: %w", setting, err)
	}
	if duration < 0 {
		return 0, "", fmt.Errorf("%s must be non-negative", setting)
	}
	return duration, source, nil
}

func testTimeoutOptions(duration time.Duration, source string) apextest.Options {
	// The runner and daemon use milliseconds. A positive duration shorter
	// than one millisecond must not silently disable the cap.
	ms := duration.Milliseconds()
	if duration > 0 && ms == 0 {
		ms = 1
	}
	return apextest.Options{TimeoutMS: ms, TimeoutSource: source}
}
