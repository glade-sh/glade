package gladehome

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestHomeReadOnlyFallbackRejectsIncompleteInstall(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod does not enforce Unix permissions on Windows")
	}
	required := []string{
		"third_party/lwc/compile.mjs",
		"third_party/lwc/node_modules/@lwc/compiler/index.js",
		"lwcruntime/src/shims/wire-adapter.mjs",
		"lwcruntime/src/shims/lds-cache.mjs",
		"lwcruntime/src/shell/app.mjs",
		"lwcruntime/src/slds/slds-loader.mjs",
		"lwcruntime/src/lightning/button.mjs",
	}
	for _, missing := range required {
		t.Run(missing, func(t *testing.T) {
			base := t.TempDir()
			t.Chdir(base)
			t.Setenv("XDG_DATA_HOME", base)
			t.Setenv("GLADE_HOME", "")
			t.Setenv("GLADE_ROOT", "")
			share := UserShareDir()
			for _, rel := range required {
				writeMinimalToolchainFile(t, share, rel)
			}
			path := filepath.Join(share, filepath.FromSlash(missing))
			if missing == required[1] {
				path = filepath.Dir(path)
			}
			if err := os.RemoveAll(path); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(share, ".install.lock"), nil, 0); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(share, 0o555); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(share, 0o755) })
			if _, err := os.Open(filepath.Join(share, ".install.lock")); !os.IsPermission(err) {
				t.Fatalf("fixture must deny access to install lock: %v", err)
			}
			for _, route := range []string{"Root", "EnsureRoot", "ToolchainStatus"} {
				t.Run(route, func(t *testing.T) {
					if err := runHomeReader(t, route+"/default"); err == nil {
						t.Fatal("read-only fallback accepted incomplete toolchain")
					}
				})
			}
		})
	}
}
