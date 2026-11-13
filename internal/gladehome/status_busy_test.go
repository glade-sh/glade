package gladehome

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestToolchainStatusBoundedInProcessInstall(t *testing.T) {
	t.Setenv("GLADE_HOME", "")
	t.Setenv("GLADE_ROOT", "")
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	if err := os.MkdirAll(UserShareDir(), 0700); err != nil {
		t.Fatal(err)
	}
	installMu.Lock()
	defer installMu.Unlock()
	start := time.Now()
	_, ok, detail := ToolchainStatusBounded(context.Background())
	if ok || !strings.Contains(detail, "busy") {
		t.Fatalf("status = %v, %q; want busy", ok, detail)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("status waited too long for in-process installer")
	}
}

func TestToolchainStatusBoundedDiscoveredRoot(t *testing.T) {
 root := t.TempDir()
 for _, rel := range []string{"third_party/lwc/compile.mjs", "third_party/lwc/node_modules/@lwc/compiler/index.js", "lwcruntime/src/shims/wire-adapter.mjs", "lwcruntime/src/shims/lds-cache.mjs", "lwcruntime/src/shell/app.mjs", "lwcruntime/src/slds/slds-loader.mjs", "lwcruntime/src/lightning/button.mjs"} {
  writeMinimalToolchainFile(t, root, rel)
 }
 t.Setenv("GLADE_HOME", "")
 t.Setenv("GLADE_ROOT", "")
 t.Setenv("XDG_DATA_HOME", filepath.Join(t.TempDir(), "empty"))
 t.Setenv("GLADE_ALLOW_PROJECT_TOOLCHAIN", "1")
 t.Chdir(root)
 path, ok, detail := ToolchainStatusBounded(context.Background())
 if !ok || path != root || detail != "ok (discovered)" {
  t.Fatalf("status = %q, %t, %q; want discovered root %q", path, ok, detail, root)
 }
}
