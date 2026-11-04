package gladehome

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRootParentRelativeHomeFromChild(t *testing.T) {
	base := t.TempDir()
	home := filepath.Join(base, "home")
	for _, path := range []string{
		"third_party/lwc/compile.mjs",
		"third_party/lwc/node_modules/@lwc/compiler/package.json",
		"lwcruntime/src/shims/wire-adapter.mjs",
		"lwcruntime/src/shims/lds-cache.mjs",
		"lwcruntime/src/shell/app.mjs",
		"lwcruntime/src/slds/slds-loader.mjs",
		"lwcruntime/src/lightning/button.mjs",
	} {
		writeMinimalToolchainFile(t, home, path)
	}
	child := filepath.Join(base, "child")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(child)
	for _, key := range []string{"GLADE_HOME", "GLADE_ROOT"} {
		t.Run(key, func(t *testing.T) {
			t.Setenv("GLADE_HOME", "")
			t.Setenv("GLADE_ROOT", "")
			t.Setenv("XDG_DATA_HOME", t.TempDir())
			t.Setenv(key, filepath.Join("..", "home"))
			for _, resolve := range []struct {
				name string
				fn   func() (string, error)
			}{{"Root", Root}, {"EnsureRoot", EnsureRoot}} {
				got, err := resolve.fn()
				if err != nil || got != home {
					t.Errorf("%s = %q, %v; want %q", resolve.name, got, err, home)
				}
			}
		})
	}
}
