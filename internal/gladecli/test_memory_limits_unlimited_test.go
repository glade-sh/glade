package gladecli

import (
	"fmt"
	"math"
	"runtime/debug"
	"testing"
)

type testMemoryLimitPolicy struct {
	name  string
	env   string
	limit int64
	gc    int
}

var testMemoryLimitPolicies = []testMemoryLimitPolicy{
	{"finite", "2GiB", 2 << 30, -1},
	{"unlimited", "off", math.MaxInt64, 50},
}

func TestApplyTestMemoryLimitsEffectiveLimitRoutes(t *testing.T) {
	for _, policy := range []testMemoryLimitPolicy{
		{"default_from_unlimited", "", math.MaxInt64, -1},
		{"finite", "2GiB", 2 << 30, -1},
		{"finite_below_maximum", "9223372036854775806", math.MaxInt64 - 1, -1},
		{"unlimited_off", "off", math.MaxInt64, 50},
		{"unlimited_numeric", "9223372036854775807", math.MaxInt64, 50},
		{"runtime_unlimited_env_finite", "2GiB", math.MaxInt64, 50},
		{"runtime_finite_env_off", "off", 2 << 30, -1},
	} {
		for _, aggressive := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/aggressive_%t", policy.name, aggressive), func(t *testing.T) {
				t.Setenv("GOMEMLIMIT", policy.env)
				// Model the startup environment and subsequent runtime API overrides.
				oldLimit := debug.SetMemoryLimit(policy.limit)
				oldGC := debug.SetGCPercent(73)
				t.Cleanup(func() {
					debug.SetGCPercent(oldGC)
					debug.SetMemoryLimit(oldLimit)
				})
				applyTestMemoryLimits(aggressive)
				wantGC := 73
				if aggressive {
					wantGC = policy.gc
				}
				if got := debug.SetGCPercent(wantGC); got != wantGC {
					t.Errorf("GC percent = %d, want %d", got, wantGC)
				}
				wantLimit := policy.limit
				if policy.env == "" {
					wantLimit = 4 << 30
				}
				if got := debug.SetMemoryLimit(-1); got != wantLimit {
					t.Errorf("memory limit = %d, want %d", got, wantLimit)
				}
			})
		}
	}
}
