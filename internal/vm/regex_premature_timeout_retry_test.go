package vm

import (
	"errors"
	"testing"
	"time"

	"github.com/dlclark/regexp2"
)

var errPrematureTimeout = errors.New("match timeout after 2s on input `0.0.0.0`")

// The retry guards the String.split/Pattern.split full-engine route against a
// stale regexp2 clock after suspension (ipv4_validation/literal_splits/org.tsv:2-5
// pin the unsplit-error-free output).
func TestRetryPrematureRegexTimeoutGates(t *testing.T) {
	want := &regexp2.Match{}
	// A suspension sleeps past half the budget. The budget leaves room for the
	// CPU other goroutines, GC and earlier subtests use in the sample window.
	const suspendedBudget, suspendedSleep = 400 * time.Millisecond, 250 * time.Millisecond
	clock := newVirtualRegexClock(t)
	t.Run("premature timeout retries", func(t *testing.T) {
		calls := 0
		got, err := retryPrematureRegexTimeout(2*time.Second, func() (*regexp2.Match, error) {
			calls++
			if calls == 1 {
				return nil, errPrematureTimeout
			}
			return want, nil
		})
		if err != nil || got != want || calls != 2 {
			t.Fatalf("got=%v err=%v calls=%d", got, err, calls)
		}
	})
	t.Run("clock lagging past one period retries again", func(t *testing.T) {
		calls := 0
		got, err := retryPrematureRegexTimeout(2*time.Second, func() (*regexp2.Match, error) {
			calls++
			if calls <= 2 {
				return nil, errPrematureTimeout
			}
			return want, nil
		})
		if err != nil || got != want || calls != 3 {
			t.Fatalf("got=%v err=%v calls=%d", got, err, calls)
		}
	})
	t.Run("timeouts until the budget is spent are returned", func(t *testing.T) {
		const budget = 300 * time.Millisecond
		started := clock.now()
		calls := 0
		_, err := retryPrematureRegexTimeout(budget, func() (*regexp2.Match, error) {
			calls++
			return nil, errPrematureTimeout
		})
		if elapsed := clock.now().Sub(started); err != errPrematureTimeout || calls < 3 || elapsed < budget {
			t.Fatalf("err=%v calls=%d elapsed=%s", err, calls, elapsed)
		}
	})
	t.Run("spent budget is not retried", func(t *testing.T) {
		clock.reset()
		calls := 0
		_, err := retryPrematureRegexTimeout(200*time.Millisecond, func() (*regexp2.Match, error) {
			calls++
			clock.burn(150 * time.Millisecond)
			return nil, errPrematureTimeout
		})
		if err != errPrematureTimeout || calls != 1 {
			t.Fatalf("err=%v calls=%d", err, calls)
		}
	})
	t.Run("search suspended in flight retries", func(t *testing.T) {
		clock.reset()
		calls := 0
		got, err := retryPrematureRegexTimeout(suspendedBudget, func() (*regexp2.Match, error) {
			calls++
			if calls == 1 {
				clock.sleep(suspendedSleep)
				return nil, errPrematureTimeout
			}
			return want, nil
		})
		if err != nil || got != want || calls != 2 {
			t.Fatalf("got=%v err=%v calls=%d", got, err, calls)
		}
	})
	t.Run("stale clock after a suspended search retries", func(t *testing.T) {
		clock.reset()
		calls := 0
		got, err := retryPrematureRegexTimeout(suspendedBudget, func() (*regexp2.Match, error) {
			calls++
			switch calls {
			case 1:
				clock.sleep(suspendedSleep)
				return nil, errPrematureTimeout
			case 2:
				return nil, errPrematureTimeout
			}
			return want, nil
		})
		if err != nil || got != want || calls != 3 {
			t.Fatalf("got=%v err=%v calls=%d", got, err, calls)
		}
	})
	t.Run("a second suspended search is returned", func(t *testing.T) {
		clock.reset()
		calls := 0
		_, err := retryPrematureRegexTimeout(suspendedBudget, func() (*regexp2.Match, error) {
			calls++
			clock.sleep(suspendedSleep)
			return nil, errPrematureTimeout
		})
		if err != errPrematureTimeout || calls > 2 {
			t.Fatalf("err=%v calls=%d", err, calls)
		}
	})
	t.Run("other errors are not retried", func(t *testing.T) {
		calls := 0
		_, err := retryPrematureRegexTimeout(2*time.Second, func() (*regexp2.Match, error) {
			calls++
			return nil, errors.New("boom")
		})
		if err == nil || calls != 1 {
			t.Fatalf("err=%v calls=%d", err, calls)
		}
	})
}

// virtualRegexClock replaces the wall, sleep and process CPU clocks behind the
// premature-timeout retry for one test. Time moves only when the test says so:
// sleep advances wall time alone (a suspended process), burn advances wall and
// CPU time (a genuine search). The product budgets and comparisons are unchanged.
type virtualRegexClock struct {
	wall time.Time
	cpu  time.Duration
}

type regexRetryTestClock struct {
	wall, cpu time.Duration
	available bool
}

func (clock *regexRetryTestClock) runtime() regexRetryRuntime {
	return regexRetryRuntime{
		now:   func() time.Time { return time.Unix(0, int64(clock.wall)) },
		sleep: func(d time.Duration) { clock.wall += d },
		sample: func(now time.Time) *regexCPUSample {
			if !clock.available {
				return nil
			}
			return &regexCPUSample{wall: now, cpu: clock.cpu}
		},
		cpu: func() (time.Duration, bool) { return clock.cpu, clock.available },
	}
}

func TestRegexRetryCPUClassificationBoundary(t *testing.T) {
	for _, tc := range []struct {
		name      string
		cpu       time.Duration
		available bool
		wantCalls int
	}{
		{"below half", 100*time.Millisecond - time.Nanosecond, true, 2},
		{"at half", 100 * time.Millisecond, true, 1},
		{"above half", 150 * time.Millisecond, true, 1},
		{"unavailable", 0, false, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clock := &regexRetryTestClock{available: tc.available}
			calls := 0
			_, err := retryPrematureRegexTimeoutWithRuntime(200*time.Millisecond, func() (*regexp2.Match, error) {
				calls++
				clock.wall += 150 * time.Millisecond
				clock.cpu += tc.cpu
				return nil, errPrematureTimeout
			}, clock.runtime())
			if err != errPrematureTimeout || calls != tc.wantCalls {
				t.Fatalf("err=%v calls=%d, want original error and %d calls", err, calls, tc.wantCalls)
			}
		})
	}
}

// Verify the production sampler and retry entry point with measured process CPU.
func TestRegexRetryRealCPUBudget(t *testing.T) {
	_, ok := regexProcessCPUTime()
	if !ok {
		t.Skip("process CPU time unavailable")
	}
	regexCPUSampleCache.Store(nil)
	calls := 0
	watchdog := time.Now().Add(20 * time.Second)
	_, err := retryPrematureRegexTimeout(200*time.Millisecond, func() (*regexp2.Match, error) {
		calls++
		start, available := regexProcessCPUTime()
		if !available {
			t.Fatal("process CPU time became unavailable")
		}
		for {
			cpu, available := regexProcessCPUTime()
			if !available {
				t.Fatal("process CPU time became unavailable")
			}
			if cpu-start >= 150*time.Millisecond {
				break
			}
			if time.Now().After(watchdog) {
				t.Fatal("CPU consumption watchdog expired")
			}
		}
		return nil, errPrematureTimeout
	})
	if err != errPrematureTimeout || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

func newVirtualRegexClock(t *testing.T) *virtualRegexClock {
	t.Helper()
	clock := &virtualRegexClock{wall: time.Unix(1_700_000_000, 0)}
	saved := regexClock
	regexClock = regexTimeSource{
		now:     clock.now,
		sleep:   clock.sleep,
		cpuTime: func() (time.Duration, bool) { return clock.cpu, true },
	}
	regexCPUSampleCache.Store(nil)
	t.Cleanup(func() {
		regexClock = saved
		regexCPUSampleCache.Store(nil)
	})
	return clock
}

func (c *virtualRegexClock) now() time.Time { return c.wall }

func (c *virtualRegexClock) sleep(d time.Duration) { c.wall = c.wall.Add(d) }

func (c *virtualRegexClock) burn(d time.Duration) {
	c.wall = c.wall.Add(d)
	c.cpu += d
}

// reset drops the cached CPU sample so the next search takes a fresh one.
func (c *virtualRegexClock) reset() { regexCPUSampleCache.Store(nil) }
