package toolchaininstall_test

import (
	"os"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/gladehome"
	"github.com/glade-sh/glade/internal/testutil/toolchaininstall"
)

// A race-instrumented consumer may need more than 15 seconds to initialize
// after the first installation completes. It must still wait for the editor.
func TestPendingConsumerAllowsSlowStartup(t *testing.T) {
	toolchaininstall.PendingConsumers(t, []string{"slow"}, func(route string) error {
		if route == "install" {
			src, err := os.Getwd()
			if err != nil {
				return err
			}
			return gladehome.InstallFrom(src)
		}
		ready := make(chan error, 1)
		go func() {
			time.Sleep(16 * time.Second)
			_, err := gladehome.EnsureRoot()
			ready <- err
		}()
		return <-ready
	})
}
