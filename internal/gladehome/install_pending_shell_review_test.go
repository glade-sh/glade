package gladehome

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSourceInstallRejectsUnmarkedShellTransaction(t *testing.T) {
	for _, state := range []string{"unstarted", "started", "committed"} {
		for _, route := range []string{"install", "ensure"} {
			t.Run(state+"/"+route, func(t *testing.T) {
				base := t.TempDir()
				t.Setenv("XDG_DATA_HOME", base)
				t.Setenv("GLADE_HOME", "")
				t.Setenv("GLADE_ROOT", "")
				share := UserShareDir()
				txn := filepath.Join(share, ".install-txn.fixture")
				if err := os.MkdirAll(txn, 0755); err != nil {
					t.Fatal(err)
				}
				if state != "unstarted" {
					if err := os.WriteFile(filepath.Join(txn, "started"), nil, 0600); err != nil {
						t.Fatal(err)
					}
				}
				if state == "committed" {
					if err := os.WriteFile(filepath.Join(txn, "committed"), nil, 0600); err != nil {
						t.Fatal(err)
					}
				}
				asset := filepath.Join(share, "third_party", "keep")
				if err := os.MkdirAll(filepath.Dir(asset), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(asset, []byte("preserve"), 0644); err != nil {
					t.Fatal(err)
				}
				source := filepath.Join(base, "source")
				for _, rel := range []string{"go.mod", "third_party/lwc/compile.mjs", "third_party/lwc/node_modules/@lwc/compiler/index.js", "lwcruntime/src/experience/fixture.mjs", "lwcruntime/src/lightning/button.mjs", "lwcruntime/src/shell/app.mjs", "lwcruntime/src/shims/wire-adapter.mjs", "lwcruntime/src/shims/lds-cache.mjs", "lwcruntime/src/slds/slds-loader.mjs"} {
					writeMinimalToolchainFile(t, source, rel)
				}
				old, err := os.Getwd()
				if err != nil {
					t.Fatal(err)
				}
				if err := os.Chdir(source); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := os.Chdir(old); err != nil {
						t.Error(err)
					}
				})
				if route == "install" {
					err = InstallFrom(source)
				} else {
					_, err = EnsureRoot()
				}
				if err == nil || !strings.Contains(err.Error(), "interrupted installer transaction") {
					t.Fatalf("%s with %s shell journal: error = %v", route, state, err)
				}
				got, err := os.ReadFile(asset)
				if err != nil || string(got) != "preserve" {
					t.Fatalf("asset = %q, %v", got, err)
				}
			})
		}
	}
}
