package gladecli

import (
	"os"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
)

func TestMain(m *testing.M) {
	restoreDiskCache := apextest.DisableDiskCacheForTesting()
	code := m.Run()
	restoreDiskCache()
	os.Exit(code)
}
