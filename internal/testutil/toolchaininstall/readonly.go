package toolchaininstall

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// ReadOnlyCompleted exercises the same consumers as PendingConsumers against a
// completed, non-writable home. No lock, a readable lock owned by another user,
// and an inaccessible lock must all allow verified assets to be read.
func ReadOnlyCompleted(t *testing.T, consumers []string, run func(*testing.T, string) error) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("chmod does not enforce directory write permissions on Windows")
	}
	for _, lockMode := range []string{"absent", "read-only", "inaccessible"} {
		t.Run(lockMode, func(t *testing.T) {
			base := t.TempDir()
			t.Chdir(base)
			t.Setenv("XDG_DATA_HOME", base)
			// Match an installed distribution: the source checkout supplies the
			// compiler script, while the managed share has no source go.mod.
			sourceFixture(t, base)
			share := filepath.Join(base, "glade")
			sourceFixture(t, share)
			if err := os.Remove(filepath.Join(share, "go.mod")); err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(share, "editor/vscode-glade.vsix"), "completed editor", 0o644)
			lockPath := filepath.Join(share, ".install.lock")
			if lockMode != "absent" {
				writeFile(t, lockPath, "", 0o644)
			}
			var dirs []string
			if err := filepath.WalkDir(share, func(path string, entry fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if entry.IsDir() {
					dirs = append(dirs, path)
					return nil
				}
				return os.Chmod(path, 0o444)
			}); err != nil {
				t.Fatal(err)
			}
			// Restore permissions before TempDir's cleanup, including after failures.
			t.Cleanup(func() {
				for _, dir := range dirs {
					_ = os.Chmod(dir, 0o755)
				}
			})
			for _, dir := range dirs {
				if err := os.Chmod(dir, 0o555); err != nil {
					t.Fatal(err)
				}
			}
			if lockMode == "inaccessible" {
				if err := os.Chmod(lockPath, 0); err != nil {
					t.Fatal(err)
				}
			}
			file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
			if err == nil {
				file.Close()
				t.Fatal("fixture permits a write lock; read-only permissions are not enforced")
			}
			for _, consumer := range consumers {
				for _, selector := range []string{"default", "GLADE_HOME", "GLADE_ROOT", "GLADE_HOME_relative", "GLADE_ROOT_relative", "GLADE_HOME_parent", "GLADE_ROOT_parent", "GLADE_HOME_symlink", "GLADE_ROOT_symlink", "GLADE_HOME_case", "GLADE_ROOT_case"} {
					t.Run(consumer+"/"+selector, func(t *testing.T) {
						if selector == "GLADE_HOME_case" || selector == "GLADE_ROOT_case" {
							info, err := os.Stat(share)
							if err != nil {
								t.Fatal(err)
							}
							variant, err := os.Stat(filepath.Join(base, "GLADE"))
							if os.IsNotExist(err) || err == nil && !os.SameFile(info, variant) {
								t.Skip("case alias requires a case-insensitive filesystem")
							}
							if err != nil {
								t.Fatal(err)
							}
						}
						t.Setenv("GLADE_HOME", "")
						t.Setenv("GLADE_ROOT", "")
						if err := run(t, consumer+"/"+selector); err != nil {
							t.Fatal(err)
						}
					})
				}
			}
			if lockMode == "absent" {
				if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
					t.Fatalf("reader created a lock in a completed home: %v", err)
				}
			}
		})
	}
}
