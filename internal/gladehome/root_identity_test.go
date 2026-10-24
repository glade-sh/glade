package gladehome

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManagedShareLockDirectoryIdentity(t *testing.T) {
	root := t.TempDir()
	share := filepath.Join(root, "managed")
	other := filepath.Join(root, "other")
	for _, dir := range []string{share, other} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(root)
	for _, alias := range []string{share, "managed", filepath.Join("other", "..", "managed")} {
		if !sameDirectory(alias, share) || !sameDirectory(share, alias) {
			t.Errorf("managed-share lock did not recognize directory alias %q", alias)
		}
	}
	for _, path := range []string{other, filepath.Join(root, "missing"), "", " "} {
		if sameDirectory(path, share) || sameDirectory(share, path) {
			t.Errorf("managed-share lock treated %q as the share", path)
		}
	}
	file := filepath.Join(root, "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if sameDirectory(file, file) {
		t.Fatal("managed-share lock accepted a regular file as a directory")
	}
}
