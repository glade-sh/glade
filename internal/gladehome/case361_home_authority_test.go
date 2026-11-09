//go:build darwin || linux

package gladehome

import (
	"path/filepath"
	"testing"
)

func TestCase361MissingHomeExplicitAndXDGRoots(t *testing.T) {
	for _, tc := range []struct {
		name string
		xdg  bool
	}{{"explicit", false}, {"xdg", true}} {
		t.Run(tc.name, func(t *testing.T) {
			base := t.TempDir()
			root := filepath.Join(base, "project")
			if tc.xdg {
				root = filepath.Join(base, "glade")
			}
			for _, rel := range []string{"go.mod", "third_party/lwc/compile.mjs", "third_party/lwc/node_modules/@lwc/compiler/package.json", "lwcruntime/src/shims/wire-adapter.mjs", "lwcruntime/src/shims/lds-cache.mjs", "lwcruntime/src/shell/app.mjs", "lwcruntime/src/slds/slds-loader.mjs", "lwcruntime/src/lightning/button.mjs"} {
				writeMinimalToolchainFile(t, root, rel)
			}
			t.Setenv("HOME", "")
			t.Setenv("XDG_DATA_HOME", "")
			t.Setenv("GLADE_HOME", "")
			t.Setenv("GLADE_ROOT", "")
			t.Setenv("GLADE_ALLOW_PROJECT_TOOLCHAIN", "")
			if tc.xdg {
				t.Setenv("XDG_DATA_HOME", base)
			} else {
				t.Setenv("GLADE_HOME", root)
			}
			if got, err := Root(); err != nil || got != root {
				t.Fatalf("Root = %q, %v", got, err)
			}
		})
	}
}
