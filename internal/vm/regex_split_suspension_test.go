//go:build darwin || linux

package vm

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/dlclark/regexp2"
)

// A paused host must not make a fresh literal search inherit an expired clock.
// The child isolates regexp2's process-global clock from other package tests.
func TestLiteralSplitStringAndPatternRoutesAfterSuspension(t *testing.T) {
	t.Logf("GOOS=%s GOARCH=%s", runtime.GOOS, runtime.GOARCH)
	const childFlag = "GLADE_LITERAL_SPLIT_CLOCK_CHILD"
	if os.Getenv(childFlag) != "1" {
		literalSplitSuspendedChild(t, "^TestLiteralSplitStringAndPatternRoutesAfterSuspension$", childFlag)
		return
	}
	routes := []string{"String.split", "Pattern.split/source", "Pattern.split/value"}
	pattern, err := patternCompile([]Value{String(`\.`)})
	if err != nil {
		t.Fatal(err)
	}
	clock := regexp2.MustCompile(`\.`, 0)
	clock.MatchTimeout = regexp2MatchTimeout
	for cycle := 0; cycle < 12; cycle++ {
		// Start regexp2's clock. This direct call meets the same stale clock
		// right after a resume, so it takes the runtime's premature retry.
		if _, err := retryPrematureRegexTimeout(clock.MatchTimeout, func() (*regexp2.Match, error) {
			return clock.FindStringMatch("0.0.0.0")
		}); err != nil {
			t.Fatal(err)
		}
		fmt.Println("LITERAL_SPLIT_PAUSE")
		if err := syscall.Kill(os.Getpid(), syscall.SIGSTOP); err != nil {
			t.Fatal(err)
		}
		until := time.Now().Add(150 * time.Millisecond)
		for time.Now().Before(until) {
			for _, route := range routes {
				for _, limit := range []int64{-1, 0, 1, 2, 5} {
					var got []string
					var err error
					switch route {
					case "String.split":
						got, err = stringRegexSplit("0.0.0.0", []Value{String(`\.`), Int(limit)})
					case "Pattern.split/source":
						got, err = patternSplit(`\.`, []Value{String("0.0.0.0"), Int(limit)})
					case "Pattern.split/value":
						got, err = patternSplitValue(pattern, []Value{String("0.0.0.0"), Int(limit)})
					}
					want := []string{"0", "0", "0", "0"}
					if limit == 1 {
						want = []string{"0.0.0.0"}
					} else if limit == 2 {
						want = []string{"0", "0.0.0"}
					}
					if err != nil || !reflect.DeepEqual(got, want) {
						t.Fatalf("cycle=%d route=%s limit=%d split=%q want=%q error=%v", cycle, route, limit, got, want, err)
					}
				}
			}
		}
	}
}

func literalSplitSuspendedChild(t *testing.T, selection, flag string) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "-test.run="+selection, "-test.v", "-test.timeout=100s")
	cmd.Env = append(os.Environ(), flag+"=1", "GOMAXPROCS=2")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()
	scanner := bufio.NewScanner(stdout)
	var output strings.Builder
	for scanner.Scan() {
		line := scanner.Text()
		output.WriteString(line + "\n")
		if line == "LITERAL_SPLIT_PAUSE" {
			time.Sleep(2400 * time.Millisecond)
			if err := cmd.Process.Signal(syscall.SIGCONT); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("suspended split child: %v\n%s", err, output.String())
	}
}
