package inputview

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestUnreadableFileObservationAgreesWithStat(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses file read permissions")
	}
	path := filepath.Join(t.TempDir(), "unreadable")
	if err := os.WriteFile(path, []byte("content"), 0000); err != nil {
		t.Fatal(err)
	}
	v := New()
	if _, err := v.ReadFile(path); !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("read error = %v, want permission error", err)
	}
	if _, err := v.Stat(path); err != nil {
		t.Fatalf("stat: %v", err)
	}
	if err := v.Recheck(); err != nil {
		t.Fatalf("unchanged permission error: %v", err)
	}
}
