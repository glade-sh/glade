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
		started := time.Now()
		calls := 0
		_, err := retryPrematureRegexTimeout(budget, func() (*regexp2.Match, error) {
			calls++
			return nil, errPrematureTimeout
		})
		if err != errPrematureTimeout || calls < 3 || time.Since(started) < budget {
			t.Fatalf("err=%v calls=%d elapsed=%s", err, calls, time.Since(started))
		}
	})
	t.Run("spent budget is not retried", func(t *testing.T) {
		clock := &regexRetryTestClock{available: true}
		calls := 0
		_, err := retryPrematureRegexTimeoutWithRuntime(200*time.Millisecond, func() (*regexp2.Match, error) {
			calls++
			clock.wall += 150 * time.Millisecond
			clock.cpu += 150 * time.Millisecond
			return nil, errPrematureTimeout
		}, clock.runtime())
		if err != errPrematureTimeout || calls != 1 {
			t.Fatalf("err=%v calls=%d", err, calls)
		}
	})
	t.Run("search suspended in flight retries", func(t *testing.T) {
		if _, ok := regexProcessCPUTime(); !ok {
			t.Skip("process CPU time unavailable on this platform")
		}
		regexCPUSampleCache.Store(nil)
		calls := 0
		got, err := retryPrematureRegexTimeout(suspendedBudget, func() (*regexp2.Match, error) {
			calls++
			if calls == 1 {
				time.Sleep(suspendedSleep)
				return nil, errPrematureTimeout
			}
			return want, nil
		})
		if err != nil || got != want || calls != 2 {
			t.Fatalf("got=%v err=%v calls=%d", got, err, calls)
		}
	})
	t.Run("stale clock after a suspended search retries", func(t *testing.T) {
		if _, ok := regexProcessCPUTime(); !ok {
			t.Skip("process CPU time unavailable on this platform")
		}
		regexCPUSampleCache.Store(nil)
		calls := 0
		got, err := retryPrematureRegexTimeout(suspendedBudget, func() (*regexp2.Match, error) {
			calls++
			switch calls {
			case 1:
				time.Sleep(suspendedSleep)
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
		regexCPUSampleCache.Store(nil)
		calls := 0
		_, err := retryPrematureRegexTimeout(suspendedBudget, func() (*regexp2.Match, error) {
			calls++
			time.Sleep(suspendedSleep)
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
