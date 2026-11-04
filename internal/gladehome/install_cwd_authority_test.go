package gladehome

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallFromCWDAuthorizesOnlyCurrentAncestors(t *testing.T) {
	if os.Getenv("GLADE_TEST_CWD_INSTALL_CHILD") == "1" {
		err := InstallFromCWD()
		data, readErr := os.ReadFile(os.Getenv("GLADE_TEST_INSTALL_LOG"))
		if os.Getenv("GLADE_TEST_EXPECT_SOURCE") == "" {
			if err == nil || !os.IsNotExist(readErr) {
				t.Fatalf("adjacent source was attempted: err=%v log=%q read=%v", err, data, readErr)
			}
			return
		}
		if err == nil || readErr != nil {
			t.Fatalf("expected recording checker to fail: err=%v read=%v", err, readErr)
		}
		got := strings.TrimSpace(string(data))
		want := filepath.Join(os.Getenv("GLADE_TEST_EXPECT_SOURCE"), "third_party/lwc/scripts/prepare-lwc-shared.mjs")
		gotInfo, ge := os.Stat(got)
		wantInfo, we := os.Stat(want)
		if ge != nil || we != nil || !os.SameFile(gotInfo, wantInfo) {
			t.Fatalf("preparation script = %q, want %q", got, want)
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
	for _, layout := range []string{"alongside", "share", "parent-share"} {
		for _, validCWD := range []bool{false, true} {
			name := layout + "/missing"
			if validCWD {
				name = layout + "/checkout"
			}
			t.Run(name, func(t *testing.T) {
				root := t.TempDir()
				bin := filepath.Join(root, "bin")
				asset := bin
				switch layout {
				case "share":
					asset = filepath.Join(bin, "share", "glade")
				case "parent-share":
					asset = filepath.Join(root, "share", "glade")
				}
				files := []string{"go.mod", "third_party/lwc/compile.mjs", "third_party/lwc/node_modules/@lwc/compiler/package.json", "third_party/lwc/scripts/prepare-lwc-shared.mjs", "lwcruntime/src/shims/wire-adapter.mjs", "lwcruntime/src/shims/lds-cache.mjs", "lwcruntime/src/shell/app.mjs", "lwcruntime/src/slds/slds-loader.mjs", "lwcruntime/src/lightning/button.mjs"}
				for _, rel := range files {
					writeMinimalToolchainFile(t, asset, rel)
				}
				if err := os.MkdirAll(bin, 0755); err != nil {
					t.Fatal(err)
				}
				copied := filepath.Join(bin, "glade-test")
				if err := os.WriteFile(copied, contents, 0755); err != nil {
					t.Fatal(err)
				}
				cwd := t.TempDir()
				expected := ""
				if validCWD {
					expected = cwd
					for _, rel := range files {
						writeMinimalToolchainFile(t, cwd, rel)
					}
					cwd = filepath.Join(cwd, "nested")
					if err := os.MkdirAll(cwd, 0755); err != nil {
						t.Fatal(err)
					}
				}
				node := t.TempDir()
				if err := os.WriteFile(filepath.Join(node, "node"), []byte("#!/bin/sh\nprintf '%s\\n' \"$1\" > \"$GLADE_TEST_INSTALL_LOG\"\nexit 1\n"), 0755); err != nil {
					t.Fatal(err)
				}
				t.Setenv("GLADE_TEST_CWD_INSTALL_CHILD", "1")
				t.Setenv("GLADE_TEST_EXPECT_SOURCE", expected)
				t.Setenv("GLADE_TEST_INSTALL_LOG", filepath.Join(t.TempDir(), "called"))
				t.Setenv("GLADE_HOME", "")
				t.Setenv("GLADE_ROOT", "")
				t.Setenv("GLADE_ALLOW_PROJECT_TOOLCHAIN", "")
				t.Setenv("XDG_DATA_HOME", t.TempDir())
				t.Setenv("PATH", node)
				cmd := exec.Command(copied, "-test.run=^TestInstallFromCWDAuthorizesOnlyCurrentAncestors$")
				cmd.Dir = cwd
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("child: %v\n%s", err, out)
				}
			})
		}
	}
}
