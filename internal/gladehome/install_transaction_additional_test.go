package gladehome

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallFromPreservesExistingToolchainOnUnreadableAsset(t *testing.T) {
	src := t.TempDir()
	for _, rel := range []string{
		"third_party/lwc/compile.mjs", "third_party/lwc/node_modules/@lwc/compiler/index.js",
		"lwcruntime/src/experience/cmsDeliveryApi.mjs", "lwcruntime/src/shims/wire-adapter.mjs",
		"lwcruntime/src/shims/lds-cache.mjs", "lwcruntime/src/shell/app.mjs",
		"lwcruntime/src/slds/slds-loader.mjs", "lwcruntime/src/lightning/button.mjs",
		"lwcruntime/src/shims/unreadable.mjs",
	} {
		writeMinimalToolchainFile(t, src, rel)
	}
	unreadable := filepath.Join(src, "lwcruntime", "src", "shims", "unreadable.mjs")
	if err := os.Chmod(unreadable, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(unreadable, 0600) })
	if file, err := os.Open(unreadable); err == nil {
		file.Close()
		t.Skip("filesystem permits reading mode-000 files")
	}
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	old := filepath.Join(UserShareDir(), "third_party", "lwc", "old.js")
	if err := os.MkdirAll(filepath.Dir(old), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(old, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := InstallFrom(src); err == nil {
		t.Fatal("expected a copy failure")
	}
	got, err := os.ReadFile(old)
	if err != nil || string(got) != "old" {
		t.Fatalf("previous toolchain was damaged: %q, %v", got, err)
	}
}
