package gladehome

import (
	"context"
	"os"
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
