package gladehome

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestLeaseEnsuredRootSelectsManagedAndCustomRoots(t *testing.T) {
	for _, custom := range []bool{false, true} {
		name := "managed"
		if custom {
			name = "custom"
		}
		t.Run(name, func(t *testing.T) {
			base := t.TempDir()
			t.Setenv("XDG_DATA_HOME", base)
			t.Setenv("GLADE_HOME", "")
			t.Setenv("GLADE_ROOT", "")
			root := UserShareDir()
			if custom {
				root = filepath.Join(base, "custom")
				t.Setenv("GLADE_HOME", root)
			}
			for _, rel := range []string{"third_party/lwc/compile.mjs", "third_party/lwc/node_modules/@lwc/compiler/index.js", "lwcruntime/src/shims/wire-adapter.mjs", "lwcruntime/src/shims/lds-cache.mjs", "lwcruntime/src/shell/app.mjs", "lwcruntime/src/slds/slds-loader.mjs", "lwcruntime/src/lightning/button.mjs"} {
				writeMinimalToolchainFile(t, root, rel)
			}
			got, release, err := LeaseEnsuredRoot()
			if err != nil {
				t.Fatal(err)
			}
			defer release()
			if got != root {
				t.Fatalf("root = %q, want %q", got, root)
			}
		})
	}
}

func TestLeaseRootWorksWhenGlobalShareIsUnavailable(t *testing.T) {
	root := t.TempDir()
	for _, rel := range []string{"third_party/lwc/compile.mjs", "third_party/lwc/node_modules/@lwc/compiler/index.js", "lwcruntime/src/shims/wire-adapter.mjs", "lwcruntime/src/shims/lds-cache.mjs", "lwcruntime/src/shell/app.mjs", "lwcruntime/src/slds/slds-loader.mjs", "lwcruntime/src/lightning/button.mjs"} {
		writeMinimalToolchainFile(t, root, rel)
	}
	blocked := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocked, nil, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_DATA_HOME", blocked)
	t.Setenv("GLADE_HOME", root)
	t.Setenv("GLADE_ROOT", "")
	got, release, err := LeaseRoot()
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if got != root {
		t.Fatalf("root = %q, want %q", got, root)
	}
}

func TestLeaseRootRejectsFailedLockAcquisition(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "custom")
	for _, rel := range []string{"third_party/lwc/compile.mjs", "third_party/lwc/node_modules/@lwc/compiler/index.js", "lwcruntime/src/shims/wire-adapter.mjs", "lwcruntime/src/shims/lds-cache.mjs", "lwcruntime/src/shell/app.mjs", "lwcruntime/src/slds/slds-loader.mjs", "lwcruntime/src/lightning/button.mjs"} {
		writeMinimalToolchainFile(t, root, rel)
	}
	t.Setenv("XDG_DATA_HOME", base)
	t.Setenv("GLADE_HOME", root)
	t.Setenv("GLADE_ROOT", "")
	_, release, err := leaseActiveRootWith(false, os.OpenFile, func(*os.File, bool) error { return syscall.EACCES })
	if err == nil {
		if release != nil {
			release()
		}
		t.Fatal("lock acquisition failure was ignored")
	}
}
