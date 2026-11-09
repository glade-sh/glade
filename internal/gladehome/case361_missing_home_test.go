//go:build darwin || linux

package gladehome

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCase361MissingHomeDoesNotTrustSharedTemp(t *testing.T) {
	temp := t.TempDir()
	share := filepath.Join(temp, "glade-share")
	for _, rel := range []string{"third_party/lwc/compile.mjs", "third_party/lwc/node_modules/@lwc/compiler/package.json", "lwcruntime/src/shims/wire-adapter.mjs", "lwcruntime/src/shims/lds-cache.mjs", "lwcruntime/src/shell/app.mjs", "lwcruntime/src/slds/slds-loader.mjs", "lwcruntime/src/lightning/button.mjs"} {
		writeMinimalToolchainFile(t, share, rel)
	}
	t.Setenv("HOME", "")
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("GLADE_HOME", "")
	t.Setenv("GLADE_ROOT", "")
	t.Setenv("GLADE_ALLOW_PROJECT_TOOLCHAIN", "")
	t.Setenv("TMPDIR", temp)
	if root, err := Root(); err == nil || !strings.Contains(err.Error(), "HOME") {
		t.Fatalf("Root = %q, %v; want missing HOME diagnostic", root, err)
	}
	if root, err := EnsureRoot(); err == nil || !strings.Contains(err.Error(), "HOME") {
		t.Fatalf("EnsureRoot = %q, %v; want missing HOME diagnostic", root, err)
	}
	if _, err := os.Stat(filepath.Join(share, ".install.lock")); !os.IsNotExist(err) {
		t.Fatalf("shared temp lock touched: %v", err)
	}
}
