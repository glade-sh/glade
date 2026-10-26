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
	t.Run("premature timeout retries once", func(t *testing.T) {
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
	t.Run("second timeout is returned", func(t *testing.T) {
		calls := 0
		_, err := retryPrematureRegexTimeout(2*time.Second, func() (*regexp2.Match, error) {
			calls++
			return nil, errPrematureTimeout
		})
		if err == nil || calls != 2 {
			t.Fatalf("err=%v calls=%d", err, calls)
		}
	})
	t.Run("spent budget is not retried", func(t *testing.T) {
		calls := 0
		_, err := retryPrematureRegexTimeout(20*time.Millisecond, func() (*regexp2.Match, error) {
			calls++
			time.Sleep(15 * time.Millisecond)
			return nil, errPrematureTimeout
		})
		if err == nil || calls != 1 {
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
