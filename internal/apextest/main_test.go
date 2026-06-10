package apextest

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// Tests use unique t.TempDir() roots, so disk cache only adds gob I/O with
	// no hit rate. In-memory runtimeCache still applies.
	disableDiskCache.Store(true)
	os.Exit(m.Run())
}
