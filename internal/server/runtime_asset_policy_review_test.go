package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestManagedRuntimeAssetKindPolicy(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "glade")
	for _, rel := range []string{
		"third_party/lwc/compile.mjs", "third_party/lwc/node_modules/@lwc/compiler/index.js",
		"lwcruntime/src/shims/wire-adapter.mjs", "lwcruntime/src/shims/lds-cache.mjs",
		"lwcruntime/src/shell/app.mjs", "lwcruntime/src/slds/slds-loader.mjs",
		"lwcruntime/src/lightning/button.mjs", "lwcruntime/src/experience/allowed.mjs",
		"lwcruntime/src/lightning/allowed.mjs", "lwcruntime/src/shell/allowed.mjs",
		"lwcruntime/src/shims/allowed.mjs", "lwcruntime/src/slds/allowed.mjs",
		"lwcruntime/src/private/secret.mjs", "lwcruntime/src/internal/secret.mjs",
	} {
		file := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(rel), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join(root, "lwcruntime/src/private"), filepath.Join(root, "lwcruntime/src/linked")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_DATA_HOME", base)
	t.Setenv("GLADE_HOME", "")
	t.Setenv("GLADE_ROOT", "")
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(base); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(old); err != nil {
			t.Error(err)
		}
	})
	for _, kind := range []string{"experience", "lightning", "shell", "shims", "slds", "private", "internal", "linked"} {
		t.Run(kind, func(t *testing.T) {
			name := "allowed.js"
			want := http.StatusOK
			if kind == "private" || kind == "internal" || kind == "linked" {
				name, want = "secret.js", http.StatusNotFound
			}
			rec := httptest.NewRecorder()
			(&Server{}).serveLightningShellRuntime(rec, httptest.NewRequest(http.MethodGet, "/", nil), []string{kind, name})
			if rec.Code != want {
				t.Fatalf("kind %s: status = %d, want %d, body %q", kind, rec.Code, want, rec.Body.String())
			}
		})
	}
}
