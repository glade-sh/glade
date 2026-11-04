package gladehome

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProjectToolchainRequiresAuthorization(t *testing.T) {
	project := t.TempDir()
	for _, rel := range []string{"go.mod", "third_party/lwc/compile.mjs", "third_party/lwc/node_modules/@lwc/compiler/package.json", "third_party/lwc/scripts/prepare-lwc-shared.mjs", "lwcruntime/src/shims/wire-adapter.mjs", "lwcruntime/src/shims/lds-cache.mjs", "lwcruntime/src/shell/app.mjs", "lwcruntime/src/slds/slds-loader.mjs", "lwcruntime/src/lightning/button.mjs", "nested/.keep"} {
		writeMinimalToolchainFile(t, project, rel)
	}
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("GLADE_HOME", "")
	t.Setenv("GLADE_ROOT", "")
	t.Setenv("GLADE_ALLOW_PROJECT_TOOLCHAIN", "")
	t.Chdir(filepath.Join(project, "nested"))
	if got, err := Root(); err == nil {
		t.Fatalf("project authorized by Root: %s", got)
	}
	if got, ok, _ := ToolchainStatus(); ok {
		t.Fatalf("project authorized by status: %s", got)
	}
	// PATH contains a recording node: automatic installation must never invoke it.
	bin := t.TempDir()
	marker := filepath.Join(t.TempDir(), "called")
	if err := os.WriteFile(filepath.Join(bin, "node"), []byte("#!/bin/sh\necho called > \"$MARKER\"\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("MARKER", marker)
	if got, err := EnsureRoot(); err == nil {
		t.Fatalf("project installed automatically: %s", got)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("preparation attempted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(UserShareDir(), "third_party")); !os.IsNotExist(err) {
		t.Fatalf("project copied: %v", err)
	}
	t.Setenv("GLADE_ALLOW_PROJECT_TOOLCHAIN", "1")
	if got, err := Root(); err != nil || got != project {
		t.Fatalf("opt-in root = %q, %v", got, err)
	}
}
