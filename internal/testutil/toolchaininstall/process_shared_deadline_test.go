package toolchaininstall_test

import (
	"os"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/gladehome"
	"github.com/glade-sh/glade/internal/testutil/toolchaininstall"
)

// A cold installer start must not consume the independent consumer's window:
// a 45-second startup followed by a 20-second consumer is within budget.
func TestIndependentConsumerDeadlineStartsAfterReadiness(t *testing.T) {
	if testing.Short() {
		t.Skip("uses real 45s and 20s waits")
	}
	toolchaininstall.PendingIndependentConsumer(t, "slow", func(route string) error {
		if route == "install" {
			time.Sleep(45 * time.Second)
			src, err := os.Getwd()
			if err != nil {
				return err
			}
			return gladehome.InstallFrom(src)
		}
		time.Sleep(20 * time.Second)
		return nil
	})
}
