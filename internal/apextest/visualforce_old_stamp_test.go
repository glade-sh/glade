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
	oldNow := fileDigestNow
	fileDigestNow = func() time.Time { return time.Now().Add(time.Minute) }
	t.Cleanup(func() { fileDigestNow = oldNow; resetFileDigestTrust() })
	resetFileDigestTrust()
	want, err := currentFileDigest(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := oldTrustedVisualforceDigest(path); !ok || got != want {
		t.Fatalf("old input: digest=%x hit=%t", got, ok)
	}
	fileDigestNow = time.Now
	if _, ok := oldTrustedVisualforceDigest(path); ok {
		t.Fatal("recent input used a timestamp-only digest")
	}
	fileDigestNow = func() time.Time { return time.Now().Add(time.Minute) }
	replacement := filepath.Join(root, "replacement")
	writeFile(t, replacement, "other")
	if err := os.Rename(replacement, path); err != nil {
		t.Fatal(err)
	}
	if _, ok := oldTrustedVisualforceDigest(path); ok {
		t.Fatal("replacement used the old digest")
	}
}
