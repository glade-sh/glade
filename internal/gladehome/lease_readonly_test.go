package gladehome

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestLeaseManagedRootReadOnlyFilesystem(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_DATA_HOME", base)
	root := filepath.Join(base, "glade")
	if err := os.Mkdir(root, 0755); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{"read-only filesystem", syscall.EROFS, false},
		{"permission denied", syscall.EACCES, false},
		{"I/O error", syscall.EIO, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			release, err := leaseManagedRoot(root, func(path string, flag int, mode os.FileMode) (*os.File, error) {
				return nil, &os.PathError{Op: "open", Path: path, Err: tc.err}
			})
			if release != nil {
				defer release()
			}
			if tc.want {
				if err != nil || release == nil {
					t.Fatalf("read-only lease = %v, %v", release != nil, err)
				}
			} else if !errors.Is(err, tc.err) {
				t.Fatalf("lease error = %v, want %v", err, tc.err)
			}
		})
	}
}
