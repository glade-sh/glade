package toolchaininstall_test

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/gladehome"
	"github.com/glade-sh/glade/internal/testutil/toolchaininstall"
)

func TestIndependentConsumerLoadedHeadroom(t *testing.T) {
	if os.Getenv("GLADE_TEST_HEADROOM") == "1" {
		toolchaininstall.PendingIndependentConsumer(t, "slow", func(route string) error {
			if route == "install" {
				src, err := os.Getwd()
				if err != nil {
					return err
				}
				return gladehome.InstallFrom(src)
			}
			time.Sleep(16 * time.Second)
			return nil
		})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestIndependentConsumerLoadedHeadroom$")
	cmd.Env = append(os.Environ(), "GLADE_TEST_HEADROOM=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("slow independent route: %v\n%s", err, out)
	}
}
