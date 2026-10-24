//go:build darwin || linux

package vm

import (
	"fmt"
	"os"
	"reflect"
	"syscall"
	"testing"
	"time"

	"github.com/dlclark/regexp2"
)

// Native ipv4_validation/literal_splits/org.tsv:2-5 and ORG-RUN.txt pin
// escaped-dot output. Exercise the full engine directly after host suspension;
// the existing String/Pattern route twins also cover this through public calls.
func TestRegexSplitEscapedDotFullEngineRouteAfterSuspension(t *testing.T) {
	const flag = "GLADE_FULL_SPLIT_CLOCK_CHILD"
	if os.Getenv(flag) != "1" {
		literalSplitSuspendedChild(t, "^TestRegexSplitEscapedDotFullEngineRouteAfterSuspension$", flag)
		return
	}
	clock := regexp2.MustCompile(`\.`, 0)
	clock.MatchTimeout = regexp2MatchTimeout
	for cycle := 0; cycle < 12; cycle++ {
		if _, err := clock.FindStringMatch("0.0.0.0"); err != nil {
			t.Fatal(err)
		}
		fmt.Println("LITERAL_SPLIT_PAUSE")
		if err := syscall.Kill(os.Getpid(), syscall.SIGSTOP); err != nil {
			t.Fatal(err)
		}
		until := time.Now().Add(150 * time.Millisecond)
		for time.Now().Before(until) {
			parts, err := splitRegexRegexp2("String.split", `\.`, "0.0.0.0", -1)
			if err != nil || !reflect.DeepEqual(parts, []string{"0", "0", "0", "0"}) {
				t.Fatalf("cycle=%d split=%q error=%v", cycle, parts, err)
			}
		}
	}
}
