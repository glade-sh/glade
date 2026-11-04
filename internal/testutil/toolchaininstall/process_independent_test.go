package toolchaininstall_test

import (
	"os"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/gladehome"
	"github.com/glade-sh/glade/internal/testutil/toolchaininstall"
)

func TestPendingIndependentConsumerAllowsSlowStartup(t *testing.T) {
	toolchaininstall.PendingIndependentConsumer(t, "slow", func(route string) error {
		if route == "install" {
			src, err := os.Getwd()
			if err != nil {
				return err
			}
			return gladehome.InstallFrom(src)
		}
		ready := make(chan struct{})
		go func() {
			time.Sleep(4 * time.Second)
			close(ready)
		}()
		<-ready
		return nil
	})
}
