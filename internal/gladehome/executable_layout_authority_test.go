package gladehome

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Run a copy of the test executable in each legacy installation layout. Marker
// files alone must not authorize adjacent code, even when the binary is genuine.
func TestExecutableAdjacentToolchainsNeedAuthorization(t *testing.T) {
	if os.Getenv("GLADE_TEST_ADJACENT_CHILD") == "1" {
		if got, err := Root(); err == nil {
			t.Fatalf("adjacent root authorized: %s", got)
		}
		if got, err := EnsureRoot(); err == nil {
			t.Fatalf("adjacent source installed: %s", got)
		}
		if _, err := os.Stat(os.Getenv("GLADE_TEST_COMMAND_LOG")); !os.IsNotExist(err) {
			t.Fatalf("preparation attempted: %v", err)
		}
		t.Setenv("GLADE_ALLOW_PROJECT_TOOLCHAIN", "1")
		if got, err := Root(); err != nil || !sameDirectory(got, os.Getenv("GLADE_TEST_ASSET")) {
			t.Fatalf("explicit opt-in root = %q, %v", got, err)
		}
		return
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	for _, layout := range []string{"alongside", "share", "parent-share", "ancestor"} {
		t.Run(layout, func(t *testing.T) {
			root := t.TempDir()
			binDir := filepath.Join(root, "bin")
			asset := binDir
			switch layout {
			case "share":
				asset = filepath.Join(binDir, "share", "glade")
			case "parent-share":
				asset = filepath.Join(root, "share", "glade")
			case "ancestor":
				asset = root
			}
			for _, rel := range []string{"go.mod", "third_party/lwc/compile.mjs", "third_party/lwc/node_modules/@lwc/compiler/package.json", "third_party/lwc/scripts/prepare-lwc-shared.mjs", "lwcruntime/src/shims/wire-adapter.mjs", "lwcruntime/src/shims/lds-cache.mjs", "lwcruntime/src/shell/app.mjs", "lwcruntime/src/slds/slds-loader.mjs", "lwcruntime/src/lightning/button.mjs"} {
				writeMinimalToolchainFile(t, asset, rel)
			}
			if err := os.MkdirAll(binDir, 0755); err != nil {
				t.Fatal(err)
			}
			copyPath := filepath.Join(binDir, "glade-test")
			if err := os.WriteFile(copyPath, contents, 0755); err != nil {
				t.Fatal(err)
			}
			nodeDir := t.TempDir()
			marker := filepath.Join(t.TempDir(), "node-ran")
			if err := os.WriteFile(filepath.Join(nodeDir, "node"), []byte("#!/bin/sh\necho called > \"$GLADE_TEST_COMMAND_LOG\"\n"), 0755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("GLADE_TEST_ADJACENT_CHILD", "1")
			t.Setenv("GLADE_TEST_COMMAND_LOG", marker)
			t.Setenv("GLADE_TEST_ASSET", asset)
			t.Setenv("GLADE_HOME", "")
			t.Setenv("GLADE_ROOT", "")
			t.Setenv("GLADE_ALLOW_PROJECT_TOOLCHAIN", "")
			t.Setenv("XDG_DATA_HOME", t.TempDir())
			t.Setenv("PATH", nodeDir)
			cmd := exec.Command(copyPath, "-test.run=^TestExecutableAdjacentToolchainsNeedAuthorization$")
			cmd.Dir = t.TempDir()
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("child: %v\n%s", err, output)
			}
		})
	}
}
