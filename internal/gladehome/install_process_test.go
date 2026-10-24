package gladehome

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/testutil/toolchaininstall"
)

func TestToolchainInstallTwoProcessesColdShare(t *testing.T) {
	toolchaininstall.ColdShare(t, []string{"EnsureRoot", "InstallFrom", "InstallFromCWD"}, func(route string) error {
		switch route {
		case "EnsureRoot":
			root, err := EnsureRoot()
			if err == nil && root != UserShareDir() {
				return fmt.Errorf("EnsureRoot = %q, want %q", root, UserShareDir())
			}
			return err
		case "InstallFrom":
			src, err := os.Getwd()
			if err != nil {
				return err
			}
			return InstallFrom(src)
		case "InstallFromCWD":
			return InstallFromCWD()
		default:
			return fmt.Errorf("unknown route %q", route)
		}
	})
}

func TestEnsureRootTwoProcessesPendingEditor(t *testing.T) {
	toolchaininstall.PendingEditor(t, func(string) error {
		_, err := EnsureRoot()
		return err
	})
}

func TestHomeReadersTwoProcessesPendingInstall(t *testing.T) {
	var routes []string
	for _, reader := range []string{"EnsureRoot", "Root", "LWCToolchainDir", "ShimsDir", "RuntimeAssetDir", "ToolchainStatus"} {
		for _, selector := range []string{"default", "GLADE_HOME", "GLADE_ROOT", "GLADE_HOME_relative", "GLADE_ROOT_relative", "GLADE_HOME_parent", "GLADE_ROOT_parent", "GLADE_HOME_symlink", "GLADE_ROOT_symlink", "GLADE_HOME_case", "GLADE_ROOT_case"} {
			routes = append(routes, reader+"/"+selector)
		}
	}
	toolchaininstall.PendingConsumers(t, routes, func(route string) error { return runHomeReader(t, route) })
}

func runHomeReader(t *testing.T, route string) error {
	if route == "install" {
		src, err := os.Getwd()
		if err != nil {
			return err
		}
		return InstallFrom(src)
	}
	reader, selector, _ := strings.Cut(route, "/")
	toolchaininstall.SelectShare(t, selector)
	var err error
	switch reader {
	case "EnsureRoot":
		_, err = EnsureRoot()
	case "Root":
		_, err = Root()
	case "LWCToolchainDir":
		_, err = LWCToolchainDir()
	case "ShimsDir":
		_, err = ShimsDir()
	case "RuntimeAssetDir":
		for _, kind := range []string{"experience", "lightning", "shell", "shims", "slds"} {
			if _, err = RuntimeAssetDir(kind); err != nil {
				return err
			}
		}
	case "ToolchainStatus":
		_, ok, detail := ToolchainStatus()
		if !ok {
			return fmt.Errorf("toolchain not ready: %s", detail)
		}
	default:
		return fmt.Errorf("unknown reader %q", reader)
	}
	return err
}

func TestHomeReadersExternalRootDoesNotRequireWritableUserShare(t *testing.T) {
	for _, env := range []string{"GLADE_HOME", "GLADE_ROOT", "discovered"} {
		t.Run(env, func(t *testing.T) {
			root := t.TempDir()
			for _, rel := range []string{"third_party/lwc/compile.mjs", "third_party/lwc/node_modules/@lwc/compiler/index.js", "lwcruntime/src/shims/wire-adapter.mjs", "lwcruntime/src/shims/lds-cache.mjs", "lwcruntime/src/shell/app.mjs", "lwcruntime/src/slds/slds-loader.mjs", "lwcruntime/src/lightning/button.mjs"} {
				writeMinimalToolchainFile(t, root, rel)
			}
			blocked := filepath.Join(t.TempDir(), "not-a-directory")
			if err := os.WriteFile(blocked, []byte("blocked"), 0o600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("XDG_DATA_HOME", blocked)
			t.Setenv("GLADE_HOME", "")
			t.Setenv("GLADE_ROOT", "")
			resolvers := []func() (string, error){Root, EnsureRoot}
			wantDetail := "ok (explicit)"
			if env == "discovered" {
				t.Chdir(root)
				// Discovery is read-only; EnsureRoot intentionally installs to the share.
				resolvers = []func() (string, error){Root}
				wantDetail = "ok (discovered)"
			} else {
				t.Setenv(env, root)
			}
			for _, resolve := range resolvers {
				got, err := resolve()
				if err != nil || got != root {
					t.Fatalf("explicit root = %q, %v; want %q", got, err, root)
				}
			}
			got, ok, detail := ToolchainStatus()
			if !ok || got != root || detail != wantDetail {
				t.Fatalf("status = %q, %t, %q", got, ok, detail)
			}
		})
	}
}

func TestHomeReadersCompletedReadOnlyHome(t *testing.T) {
	toolchaininstall.ReadOnlyCompleted(t, []string{"EnsureRoot", "Root", "LWCToolchainDir", "ShimsDir", "RuntimeAssetDir", "ToolchainStatus"}, runHomeReader)
}
