//go:build darwin || linux

package testdaemon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCase340MissingSocketRequiresTrustedAncestry(t *testing.T) {
	root := t.TempDir()
	if !secureDiscoveryPath(root) {
		t.Fatal("safe control root is not trusted")
	}
	for _, name := range []string{"unsafe-parent", "unsafe-ancestor"} {
		t.Run(name, func(t *testing.T) {
			parent := filepath.Join(root, name)
			if err := os.MkdirAll(filepath.Join(parent, "nested"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(parent, 0777); err != nil {
				t.Fatal(err)
			}
			dir := parent
			if name == "unsafe-ancestor" {
				dir = filepath.Join(parent, "nested")
			}
			err := ValidateDiscoverySocket(filepath.Join(dir, "missing.sock"))
			if err == nil || !strings.Contains(err.Error(), "insecure") {
				t.Fatalf("missing socket in unsafe ancestry: %v", err)
			}
			err = ValidateDiscoverySocket(filepath.Join(dir, "missing", "nested", "socket"))
			if err == nil || !strings.Contains(err.Error(), "insecure") {
				t.Fatalf("missing directory in unsafe ancestry: %v", err)
			}
		})
	}
	err := ValidateDiscoverySocket(filepath.Join(root, "missing", "nested", "socket"))
	if !IsServerUnavailable(err) {
		t.Fatalf("trusted missing directory should remain unavailable: %v", err)
	}
}
