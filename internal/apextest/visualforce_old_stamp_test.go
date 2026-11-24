package apextest

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestOldTrustedVisualforceDigestRequiresOldMatchingStamp(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "input")
	writeFile(t, path, "first")
	oldNow := fileDigestClock.Load()
	setFileDigestClockForTesting(func() time.Time { return time.Now().Add(time.Minute) })
	t.Cleanup(func() { fileDigestClock.Store(oldNow); resetFileDigestTrust() })
	resetFileDigestTrust()
	want, err := currentFileDigest(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := oldTrustedVisualforceDigest(path); !ok || got != want {
		t.Fatalf("old input: digest=%x hit=%t", got, ok)
	}
	setFileDigestClockForTesting(func() time.Time { return time.Unix(0, 0) })
	if _, ok := oldTrustedVisualforceDigest(path); ok {
		t.Fatal("recent input used a timestamp-only digest")
	}
	setFileDigestClockForTesting(func() time.Time { return time.Now().Add(time.Minute) })
	replacement := filepath.Join(root, "replacement")
	writeFile(t, replacement, "other")
	if err := os.Rename(replacement, path); err != nil {
		t.Fatal(err)
	}
	if _, ok := oldTrustedVisualforceDigest(path); ok {
		t.Fatal("replacement used the old digest")
	}
}
