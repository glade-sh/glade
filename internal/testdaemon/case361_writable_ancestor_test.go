//go:build darwin || linux

package testdaemon

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCase361DiscoveryWritableAncestorPolicy(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mode    os.FileMode
		trusted bool
	}{
		{"group", 0770, false}, {"world", 0777, false}, {"sticky", 0777 | os.ModeSticky, true}, {"private", 0700, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := t.TempDir()
			if err := os.Chmod(base, tc.mode); err != nil {
				t.Fatal(err)
			}
			socket := filepath.Join(base, "missing", "serve.sock")
			err := ValidateDiscoverySocket(socket)
			if tc.trusted {
				if !IsServerUnavailable(err) || !errors.Is(err, ErrDiscoveryDirectoryMissing) {
					t.Fatalf("trusted ancestor: %v", err)
				}
			} else if !errors.Is(err, ErrUntrustedDiscovery) {
				t.Fatalf("writable ancestor: %v", err)
			}
		})
	}
}
