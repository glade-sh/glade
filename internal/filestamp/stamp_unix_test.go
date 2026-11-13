//go:build darwin || linux

package filestamp

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDeviceVerdictExpires(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(path, []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	trustedDevices.Clear()
	now := time.Unix(5000, 0)
	oldClock := verdictClock
	verdictClock = func() time.Time { return now }
	t.Cleanup(func() { verdictClock = oldClock })
	calls := 0
	verdict := func(v bool) func() bool { return func() bool { calls++; return v } }

	if _, ok := fromInfo(info, verdict(false)); ok {
		t.Fatal("untrusted device stamped")
	}
	if _, ok := fromInfo(info, verdict(true)); ok || calls != 1 {
		t.Fatalf("verdict not cached inside the window (ok=%v calls=%d)", ok, calls)
	}
	now = now.Add(deviceVerdictTTL)
	if _, ok := fromInfo(info, verdict(true)); !ok || calls != 2 {
		t.Fatalf("verdict not re-checked after the window (ok=%v calls=%d)", ok, calls)
	}
}
