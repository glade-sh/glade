package vm

import (
	"reflect"
	"testing"
	"time"
)

// prematureRegexTimeouts models regexp2's approximate clock lagging for longer
// than one clock period after a host resume: the first n searches of input fail
// at once with a timeout, well inside the match budget. This reproduces the
// Linux CI failure without SIGSTOP. Native escaped-dot split results:
// ipv4_validation/literal_splits/org.tsv:2-5.
func prematureRegexTimeouts(t *testing.T, input string, n int) *int {
	t.Helper()
	calls := 0
	regexp2SearchFault = func(searched string) error {
		if searched != input {
			return nil
		}
		calls++
		if calls <= n {
			return errPrematureTimeout
		}
		return nil
	}
	t.Cleanup(func() { regexp2SearchFault = nil })
	return &calls
}

func TestPrematureRegexTimeoutSplitRoutesAreNotSwallowed(t *testing.T) {
	const input = "1.2.3.4"
	pattern, err := patternCompile([]Value{String(`\.`)})
	if err != nil {
		t.Fatal(err)
	}
	for _, route := range []string{"String.split", "Pattern.split/source", "Pattern.split/value"} {
		for _, limit := range []int64{-1, 0, 2} {
			calls := prematureRegexTimeouts(t, input, 2)
			var got []string
			switch route {
			case "String.split":
				got, err = stringRegexSplit(input, []Value{String(`\.`), Int(limit)})
			case "Pattern.split/source":
				got, err = patternSplit(`\.`, []Value{String(input), Int(limit)})
			case "Pattern.split/value":
				got, err = patternSplitValue(pattern, []Value{String(input), Int(limit)})
			}
			want := []string{"1", "2", "3", "4"}
			if limit == 2 {
				want = []string{"1", "2.3.4"}
			}
			if err != nil || !reflect.DeepEqual(got, want) || *calls <= 2 {
				t.Fatalf("route=%s limit=%d split=%q want=%q error=%v searches=%d", route, limit, got, want, err, *calls)
			}
		}
	}
}

func TestPrematureRegexTimeoutMatchAndReplaceRoutesRetry(t *testing.T) {
	const input = "1.2.3.4"
	t.Run("Pattern.matches", func(t *testing.T) {
		calls := prematureRegexTimeouts(t, input, 2)
		got, err := patternMatches([]Value{String(`[0-9.]+`), String(input)})
		if err != nil || got.Kind != ValueBool || !got.Bool || *calls <= 2 {
			t.Fatalf("matches=%v error=%v searches=%d", got, err, *calls)
		}
	})
	t.Run("String.replaceAll", func(t *testing.T) {
		calls := prematureRegexTimeouts(t, input, 2)
		got, err := stringRegexReplace("String.replaceAll", input, []Value{String(`\.`), String("-")}, true)
		if err != nil || got != "1-2-3-4" || *calls <= 2 {
			t.Fatalf("replaceAll=%q error=%v searches=%d", got, err, *calls)
		}
	})
}

// On Linux a process-directed SIGSTOP can land while a search is in flight
// (capture-host linux/amd64: a search began 111us after kill returned and ended 2.4s
// later with a timeout, regexp2's cached clock exact). The search spent its
// budget in wall time but not in CPU time, so it must be retried rather than
// kept as the base unsplit result. The sleeping fault models that suspension.
func TestRegexSplitSearchSuspendedInFlightIsRetried(t *testing.T) {
	if _, ok := regexProcessCPUTime(); !ok {
		t.Skip("process CPU time unavailable on this platform")
	}
	const input = "5.6.7.8"
	for _, limit := range []int64{-1, 0} {
		calls := 0
		regexp2SearchFault = func(searched string) error {
			if searched != input {
				return nil
			}
			calls++
			if calls == 1 {
				time.Sleep(regexp2MatchTimeout/2 + 100*time.Millisecond)
				return errPrematureTimeout
			}
			return nil
		}
		got, err := stringRegexSplit(input, []Value{String(`\.`), Int(limit)})
		regexp2SearchFault = nil
		if err != nil || !reflect.DeepEqual(got, []string{"5", "6", "7", "8"}) || calls < 2 {
			t.Fatalf("limit=%d split=%q error=%v searches=%d", limit, got, err, calls)
		}
	}
}
