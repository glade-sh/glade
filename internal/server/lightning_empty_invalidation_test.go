package server

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// An empty preparation must not hide a later source or toolchain change.
func TestEmptyLightningPreparationRevalidatesInputs(t *testing.T) {
	for _, change := range []string{"module added", "toolchain changed"} {
		for _, route := range []string{"bootstrap", "module"} {
			t.Run(change+"/"+route, func(t *testing.T) {
				root := t.TempDir()
				if change == "module added" {
					path := filepath.Join(root, "force-app", "main", "default", "lwc", "sample", "sample.js")
					if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, []byte("export default class Sample {}"), 0o600); err != nil {
						t.Fatal(err)
					}
				} else {
					t.Setenv("GLADE_HOME", filepath.Join(root, "changed-toolchain"))
				}
				previous := ensureLightningRoot
				calls := 0
				ensureLightningRoot = func() (string, error) {
					calls++
					return "", errors.New("revalidation sentinel")
				}
				t.Cleanup(func() { ensureLightningRoot = previous })
				s := &Server{lightning: lightningState{cacheDir: filepath.Join(root, "old-cache")}}
				if route == "bootstrap" {
					_, _, err := s.lightningBootstrapConfigLocked()
					if err == nil || !strings.Contains(err.Error(), "revalidation sentinel") {
						t.Fatalf("bootstrap error = %v, want revalidation error", err)
					}
				} else {
					w := httptest.NewRecorder()
					s.handleLightningModules(w, httptest.NewRequest(http.MethodGet, "/lightning/modules/c/sample.js", nil), []string{"c", "sample.js"})
					if w.Code != http.StatusNotImplemented || !strings.Contains(w.Body.String(), "revalidation sentinel") {
						t.Fatalf("module response = %d %s, want revalidation error", w.Code, w.Body.String())
					}
				}
				if calls != 1 {
					t.Fatalf("toolchain checks = %d, want 1", calls)
				}
			})
		}
	}
}
