package gladehome

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The writer lock is fail-closed: when the install lock cannot be created or
// opened, InstallFrom returns an error before touching any assets.
func TestInstallFromFailsClosedWhenWriterLockUnavailable(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(t *testing.T, base string)
		want  string
	}{
		{
			name: "share directory blocked by a file",
			setup: func(t *testing.T, base string) {
				if err := os.WriteFile(filepath.Join(base, "glade"), []byte("x"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
			want: "create toolchain install directory",
		},
		{
			name: "lock path is a directory",
			setup: func(t *testing.T, base string) {
				if err := os.MkdirAll(filepath.Join(base, "glade", ".install.lock"), 0o755); err != nil {
					t.Fatal(err)
				}
			},
			want: "open toolchain install lock",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := t.TempDir()
			t.Setenv("XDG_DATA_HOME", base)
			tc.setup(t, base)
			src := filepath.Join(t.TempDir(), "src")
			if err := InstallFrom(src); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("InstallFrom error = %v, want %q", err, tc.want)
			}
			entries, err := os.ReadDir(filepath.Join(base, "glade"))
			if err == nil {
				for _, entry := range entries {
					if entry.Name() != ".install.lock" {
						t.Fatalf("InstallFrom wrote %s without the install lock", entry.Name())
					}
				}
			}
		})
	}
}
