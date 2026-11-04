package testdaemon

import (
	"os"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
)

func TestMain(m *testing.M) {
	cleanupPrivateTemp := preparePrivateTestTemp()
	restoreDiskCache := apextest.DisableDiskCacheForTesting()
	code := m.Run()
	restoreDiskCache()
	if err := cleanupPrivateTemp(); err != nil {
		_, _ = os.Stderr.WriteString("clean private test temp: " + err.Error() + "\n")
		code = 1
	}
	os.Exit(code)
}
