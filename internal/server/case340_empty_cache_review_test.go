package server

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestCase340EmptyLightningCacheRechecksToolchain(t *testing.T) {
	sentinel := errors.New("toolchain rechecked")
	previous := ensureLightningRoot
	calls := 0
	ensureLightningRoot = func() (string, error) { calls++; return "", sentinel }
	t.Cleanup(func() { ensureLightningRoot = previous })
	for _, route := range []string{"module", "bootstrap"} {
		t.Run(route, func(t *testing.T) {
			root := t.TempDir()
			org := storage.NewOrgState()
			handler := New(&org)
			handler.lightning = lightningState{cacheRoot: root, cacheDir: filepath.Join(root, "lwc")}
			before := calls
			if route == "bootstrap" {
				_, _, err := handler.lightningBootstrapConfigLocked()
				if !errors.Is(err, sentinel) {
					t.Errorf("bootstrap did not report toolchain error: %v", err)
				}
			} else {
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/lightning/modules/c/synthetic/synthetic.js", nil))
				if rec.Code == http.StatusOK {
					t.Errorf("module accepted unavailable toolchain")
				}
			}
			if calls != before+1 {
				t.Errorf("empty %s cache skipped toolchain check", route)
			}
		})
	}
}
