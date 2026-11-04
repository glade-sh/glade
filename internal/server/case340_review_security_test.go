package server

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestCase340SameSiteWithoutOriginRejected(t *testing.T) {
	for _, path := range []string{"/apex/SyntheticPage", "/services/apexrest/synthetic", "/services/data/v62.0/glade/reset"} {
		req := httptest.NewRequest(http.MethodGet, "http://localhost:1789"+path, nil)
		req.Header.Set("Sec-Fetch-Site", "same-site")
		org := storage.NewOrgState()
		rec := httptest.NewRecorder()
		New(&org).ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s: status %d, want 403", path, rec.Code)
		}
	}
	for _, site := range []string{"same-origin", "none", ""} {
		req := httptest.NewRequest(http.MethodGet, "http://localhost:1789/services/data/", nil)
		if site != "" {
			req.Header.Set("Sec-Fetch-Site", site)
		}
		org := storage.NewOrgState()
		rec := httptest.NewRecorder()
		New(&org).ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("site %q: status %d, want 200", site, rec.Code)
		}
	}
}

func TestCase340ConfiguredLoopbackAddressAccepted(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	for _, address := range []string{"127.0.0.2:1789", "127.0.1.5:1789", "[0:0:0:0:0:0:0:1]:1789"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "http://"+address+"/", nil)
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		GuardHTTPAuthority(next, address).ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Errorf("%s: status %d, want 204", address, rec.Code)
		}
	}
}

func TestCase340EmptyLightningCacheRevalidatedAndRemoved(t *testing.T) {
	previous := ensureLightningRoot
	ensureLightningRoot = func() (string, error) { return "", errors.New("toolchain rechecked") }
	t.Cleanup(func() { ensureLightningRoot = previous })
	for _, reset := range []bool{false, true} {
		root := filepath.Join(t.TempDir(), "cache")
		if err := os.Mkdir(root, 0700); err != nil {
			t.Fatal(err)
		}
		org := storage.NewOrgState()
		handler := New(&org)
		handler.lightning = lightningState{cacheRoot: root, cacheDir: filepath.Join(root, "lwc")}
		if err := handler.ensureLightningLocked(); err == nil || err.Error() != "toolchain rechecked" {
			t.Errorf("reset=%v: empty cache did not revalidate: %v", reset, err)
		}

		if reset {
			handler.ResetLightningCache()
		} else if err := handler.Close(); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(root); !os.IsNotExist(err) {
			t.Errorf("cache root survived cleanup: %v", err)
		}
	}
}
