//go:build unix

package gladehome

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestToolchainStatusReportsUnavailableInstallLock(t *testing.T) {
	base := t.TempDir()
	t.Chdir(base)
	t.Setenv("XDG_DATA_HOME", base)
	t.Setenv("GLADE_HOME", "")
	t.Setenv("GLADE_ROOT", "")
	share := UserShareDir()
	for _, rel := range []string{"third_party/lwc/compile.mjs", "third_party/lwc/node_modules/@lwc/compiler/index.js", "lwcruntime/src/shims/wire-adapter.mjs", "lwcruntime/src/shims/lds-cache.mjs", "lwcruntime/src/shell/app.mjs", "lwcruntime/src/slds/slds-loader.mjs", "lwcruntime/src/lightning/button.mjs", ".install.lock"} {
		writeMinimalToolchainFile(t, share, rel)
	}
	lock := filepath.Join(share, ".install.lock")
	if err := os.Chmod(lock, 0); err != nil {
		t.Fatal(err)
	}
	if file, err := os.Open(lock); err == nil {
		file.Close()
		t.Skip("permissions are not enforced")
	}
	_, ok, detail := ToolchainStatus()
	if ok || !strings.Contains(detail, "install lock") || !strings.Contains(detail, "permission denied") {
		t.Fatalf("status ok=%v detail=%q; want a lock permission error", ok, detail)
	}
}
