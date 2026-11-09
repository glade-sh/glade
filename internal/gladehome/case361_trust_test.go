//go:build darwin || linux

package gladehome

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCase361ToolchainDiscoveryRequiresAuthorityOnWritableAncestors(t *testing.T) {
	for _, tc := range []struct {
		name string
		mode os.FileMode
	}{
		{"group", 0770}, {"world", 0777}, {"sticky", 0777 | os.ModeSticky},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parent := t.TempDir()
			if err := os.Chmod(parent, tc.mode); err != nil {
				t.Fatal(err)
			}
			project := filepath.Join(parent, "project")
			for _, rel := range []string{"go.mod", "third_party/lwc/compile.mjs", "third_party/lwc/node_modules/@lwc/compiler/package.json", "lwcruntime/src/shims/wire-adapter.mjs", "lwcruntime/src/shims/lds-cache.mjs", "lwcruntime/src/shell/app.mjs", "lwcruntime/src/slds/slds-loader.mjs", "lwcruntime/src/lightning/button.mjs", "nested/.keep"} {
				writeMinimalToolchainFile(t, project, rel)
			}
			t.Setenv("XDG_DATA_HOME", t.TempDir())
			t.Setenv("GLADE_HOME", "")
			t.Setenv("GLADE_ROOT", "")
			t.Setenv("GLADE_ALLOW_PROJECT_TOOLCHAIN", "")
			t.Chdir(filepath.Join(project, "nested"))
			if root, err := Root(); err == nil {
				t.Fatalf("unauthorized discovery selected %q", root)
			}
			t.Setenv("GLADE_ALLOW_PROJECT_TOOLCHAIN", "1")
			if root, err := Root(); err != nil || root != project {
				t.Fatalf("opt-in root = %q, %v", root, err)
			}
			t.Setenv("GLADE_ALLOW_PROJECT_TOOLCHAIN", "")
			t.Setenv("GLADE_HOME", project)
			if root, err := Root(); err != nil || root != project {
				t.Fatalf("explicit root = %q, %v", root, err)
			}
		})
	}
}
