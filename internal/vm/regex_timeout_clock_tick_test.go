package vm

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/dlclark/regexp2"
)

// The search callback models an approximate clock whose updater is still
// pending after a premature timeout. Fake time makes the ordering independent
// of host scheduling and avoids suspending the test process.
// Native split results: ipv4_validation/literal_splits/org.tsv:2-5.
func TestRegexTimeoutRetryAllowsPendingClockTick(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tick := time.NewTimer(regexp2.DefaultClockPeriod)
		defer tick.Stop()
		re := regexp2.MustCompile(`\.`, regexp2.None)
		calls := 0
		match, err := retryPrematureRegexTimeout(regexp2MatchTimeout, func() (*regexp2.Match, error) {
			calls++
			select {
			case <-tick.C:
				return re.FindStringMatch("0.0.0.0")
			default:
				return nil, errPrematureTimeout
			}
		})
		if err != nil || match == nil || match.Index != 1 || match.String() != "." || calls != 2 {
			t.Fatalf("fresh search before clock update: match=%v err=%v calls=%d", match, err, calls)
		}
	})
}

func TestRegexTimeoutRetrySuccessfulSearchDoesNotWait(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		started := time.Now()
		want := &regexp2.Match{}
		calls := 0
		match, err := retryPrematureRegexTimeout(regexp2MatchTimeout, func() (*regexp2.Match, error) {
			calls++
			return want, nil
		})
		if err != nil || match != want || calls != 1 || time.Since(started) != 0 {
			t.Fatalf("successful search changed: match=%v err=%v calls=%d elapsed=%s", match, err, calls, time.Since(started))
		}
	})
}
