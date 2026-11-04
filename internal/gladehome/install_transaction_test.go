package gladehome

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallFromPreservesExistingToolchainOnCopyFailure(t *testing.T) {
	src := t.TempDir()
	for _, rel := range []string{
		"third_party/lwc/compile.mjs",
		"third_party/lwc/node_modules/@lwc/compiler/index.js",
		"lwcruntime/src/shims/wire-adapter.mjs",
		"lwcruntime/src/shims/lds-cache.mjs",
		"lwcruntime/src/shell/app.mjs",
		"lwcruntime/src/slds/slds-loader.mjs",
		"lwcruntime/src/lightning/button.mjs",
	} {
		writeMinimalToolchainFile(t, src, rel)
	}
	// The optional validation marker for experience is absent, so the second
	// directory copy fails after the first directory would have been replaced.
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
