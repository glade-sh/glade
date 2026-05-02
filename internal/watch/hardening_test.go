package watch

import (
	"path/filepath"
	"testing"
)

func TestNoPanicOnMissingAndMalformedWatchInputs(t *testing.T) {
	root := t.TempDir()
	assertNoPanic(t, func() {
		_, _ = CaptureSnapshot(filepath.Join(root, "missing"))
		_, _ = CapturePaths([]string{filepath.Join(root, "missing.cls"), ""})
		_ = DiffSnapshots(Snapshot{}, Snapshot{})
	})
}

func assertNoPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("unexpected panic: %v", recovered)
		}
	}()
	fn()
}
