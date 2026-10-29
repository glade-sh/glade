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
		regexCPUSampleCache.Store(nil)
		calls := 0
		_, err := retryPrematureRegexTimeout(200*time.Millisecond, func() (*regexp2.Match, error) {
			calls++
			burnRegexTestCPU(150 * time.Millisecond)
			return nil, errPrematureTimeout
		})
		if err == nil || calls != 1 {
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

// burnRegexTestCPU spins for d of wall time, using CPU the way a genuine
// catastrophic search does.
func burnRegexTestCPU(d time.Duration) {
	for until := time.Now().Add(d); time.Now().Before(until); {
	}
}
