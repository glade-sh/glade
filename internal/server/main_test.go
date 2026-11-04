package server

import (
	"os"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
)

func TestMain(m *testing.M) {
	cleanupPrivateTemp := preparePrivateTestTemp()
	// Server tests exercise the checked-out runtime assets. Authorize that
	// checkout explicitly when the caller has not selected a toolchain.
	if os.Getenv("GLADE_HOME") == "" && os.Getenv("GLADE_ROOT") == "" {
		if root, err := lightningTestRepoRoot(); err == nil {
			if err := os.Setenv("GLADE_ROOT", root); err != nil {
				panic(err)
			}
		}
	}
	restoreDiskCache := apextest.DisableDiskCacheForTesting()
	code := m.Run()
	restoreDiskCache()
	if err := cleanupPrivateTemp(); err != nil {
		_, _ = os.Stderr.WriteString("clean private test temp: " + err.Error() + "\n")
		code = 1
	}
	os.Exit(code)
}
