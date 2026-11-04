//go:build unix

package gladehome

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestFirstInstallReaderAcquiresLock(t *testing.T) {
	for _, route := range []string{"active", "ensured", "managed"} {
		for _, state := range []string{"available", "denied", "missing"} {
			name := route
			name += "-" + state
			t.Run(name, func(t *testing.T) {
				base := t.TempDir()
				t.Chdir(base)
				t.Setenv("XDG_DATA_HOME", base)
				t.Setenv("GLADE_HOME", "")
				t.Setenv("GLADE_ROOT", "")
				root := filepath.Join(base, "glade")
				attempts := 0
				open := func(path string, flags int, mode os.FileMode) (*os.File, error) {
					attempts++
					if attempts <= 2 {
						if attempts == 2 {
							for _, rel := range []string{"third_party/lwc/compile.mjs", "third_party/lwc/node_modules/@lwc/compiler/index.js", "lwcruntime/src/shims/wire-adapter.mjs", "lwcruntime/src/shims/lds-cache.mjs", "lwcruntime/src/shell/app.mjs", "lwcruntime/src/slds/slds-loader.mjs", "lwcruntime/src/lightning/button.mjs"} {
								writeMinimalToolchainFile(t, root, rel)
							}
						}
						return nil, &os.PathError{Op: "open", Path: path, Err: syscall.ENOENT}
					}
					if state == "denied" {
						return nil, &os.PathError{Op: "open", Path: path, Err: syscall.EACCES}
					}
					if state == "missing" {
						return nil, &os.PathError{Op: "open", Path: path, Err: syscall.ENOENT}
					}
					return os.OpenFile(path, flags, mode)
				}
				var release func()
				var err error
				if route == "managed" {
					release, err = leaseManagedRoot(root, open)
				} else {
					_, release, err = leaseActiveRootWith(route == "ensured", open, lockInstallFile)
				}
				if release != nil {
					defer release()
				}
				if state == "denied" {
					if !errors.Is(err, syscall.EACCES) {
						t.Fatalf("missing lock denial: %v", err)
					}
					return
				}
				if state == "missing" {
					if err == nil {
						t.Fatal("reader accepted an unavailable installation lock")
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(root, ".install.lock")
				if _, err := os.Stat(path); err != nil {
					t.Fatalf("first install reader returned without lock: %v", err)
				}
				if got := identityFence(t, path); got != "held" {
					t.Fatalf("first install writer fence = %s", got)
				}
			})
		}
	}
}
