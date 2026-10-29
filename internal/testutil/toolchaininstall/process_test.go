package toolchaininstall_test

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/gladehome"
	"github.com/glade-sh/glade/internal/testutil/toolchaininstall"
)

func TestPendingIndependentConsumerDeadline(t *testing.T) {
	if scenario := os.Getenv("GLADE_TEST_INDEPENDENCE_SCENARIO"); scenario != "" {
		toolchaininstall.PendingIndependentConsumer(t, scenario, func(route string) error {
			if route == "install" || route == "blocked" {
				src, err := os.Getwd()
				if err != nil {
					return err
				}
				return gladehome.InstallFrom(src)
			}
			// Longer than the former 3-second bound, well inside the deadline.
			time.Sleep(4 * time.Second)
			return nil
		})
		return
	}

	for _, scenario := range []string{"slow", "blocked"} {
		t.Run(scenario, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestPendingIndependentConsumerDeadline$")
			cmd.Env = append(os.Environ(), "GLADE_TEST_INDEPENDENCE_SCENARIO="+scenario)
			out, err := cmd.CombinedOutput()
			if ctx.Err() != nil {
				t.Fatalf("scenario timed out: %v\n%s", ctx.Err(), out)
			}
			if scenario == "blocked" {
				if err == nil || !strings.Contains(string(out), "independent route blocked blocked on the toolchain install") {
					t.Fatalf("expected blocked-route failure, got %v\n%s", err, out)
				}
			} else if err != nil {
				t.Fatalf("independent route failed: %v\n%s", err, out)
			}
		})
	}
}
