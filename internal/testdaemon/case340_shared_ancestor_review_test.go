//go:build darwin || linux

package testdaemon

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestCase340MissingDiscoveryAllowsTrustedSharedAncestors(t *testing.T) {
	for _, base := range []string{"/", "/tmp"} {
		t.Run(base, func(t *testing.T) {
			if !secureDiscoveryAncestry(base) {
				t.Fatalf("shared control is not trusted: %s", base)
			}
			missing := filepath.Join(base, fmt.Sprintf(".r340-missing-%d", os.Getpid()), "nested", "serve.sock")
			if _, err := os.Lstat(filepath.Dir(missing)); !os.IsNotExist(err) {
				t.Fatalf("missing control unexpectedly exists: %v", err)
			}
			if err := ValidateDiscoverySocket(missing); !IsServerUnavailable(err) || !errors.Is(err, ErrDiscoveryDirectoryMissing) {
				t.Fatalf("trusted shared ancestor must permit fallback without PID access: %v", err)
			}
		})
	}
}
