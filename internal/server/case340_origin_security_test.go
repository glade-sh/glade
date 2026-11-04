package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestCase340CrossOriginCannotMutate(t *testing.T) {
	for _, tc := range []struct{ method, path, origin, site string }{
		{http.MethodPost, "/services/data/v67.0/glade/reset", "https://attacker.invalid", "cross-site"},
		{http.MethodGet, "/services/data/v67.0/tooling/executeAnonymous?anonymousBody=System.debug(1)", "", "cross-site"},
		{http.MethodPost, "/services/data/v62.0/glade/reset", "http://attacker.invalid", ""},
	} {
		t.Run(tc.method+tc.path+tc.origin+tc.site, func(t *testing.T) {
			org := storage.NewOrgState()
			handler := New(&org)
			req := httptest.NewRequest(tc.method, "http://localhost:1234"+tc.path, nil)
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			if tc.site != "" {
				req.Header.Set("Sec-Fetch-Site", tc.site)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403", rec.Code)
			}
		})
	}
}

func TestCase340SameOriginAndLocalClientsRemainAllowed(t *testing.T) {
	for _, origin := range []string{"", "http://localhost:1234"} {
		org := storage.NewOrgState()
		handler := New(&org)
		req := httptest.NewRequest(http.MethodGet, "http://localhost:1234/services/data/", nil)
		if origin != "" {
			req.Header.Set("Origin", origin)
			req.Header.Set("Sec-Fetch-Site", "same-origin")
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("origin %q: status = %d", origin, rec.Code)
		}
	}
}

func TestCase340BrowserHostOriginRoutes(t *testing.T) {
	for _, path := range []string{"/apex/SyntheticPage", "/lightning/glade.out.js"} {
		for _, tc := range []struct {
			origin    string
			forbidden bool
		}{{"https://attacker.invalid", true}, {"http://localhost:1234", false}} {
			org := storage.NewOrgState()
			handler := New(&org)
			req := httptest.NewRequest(http.MethodGet, "http://localhost:1234"+path, nil)
			req.Header.Set("Origin", tc.origin)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if (rec.Code == http.StatusForbidden) != tc.forbidden {
				t.Fatalf("%s origin %s: status = %d", path, tc.origin, rec.Code)
			}
		}
	}
}

func TestCase340OriginCanonicalDefaultPorts(t *testing.T) {
	for _, tc := range []struct {
		host, origin string
		allowed      bool
	}{
		{"localhost:80", "http://localhost", true},
		{"[::1]:80", "http://[::1]", true},
		{"localhost:1789", "http://localhost", false},
		{"localhost:80", "http://localhost/path", false},
		{"localhost:80", "http://user@localhost", false},
	} {
		req := httptest.NewRequest(http.MethodGet, "http://localhost/", nil)
		req.Host = tc.host
		req.Header.Set("Origin", tc.origin)
		if got := localRequestOriginAllowed(req); got != tc.allowed {
			t.Errorf("host %q origin %q: allowed=%v", tc.host, tc.origin, got)
		}
	}
}

func TestCase340OriginIPHostIdentity(t *testing.T) {
	for _, tc := range []struct {
		host, origin string
		allowed      bool
	}{
		{"127.0.0.1:80", "http://[::ffff:127.0.0.1]", false},
		{"[::1]:80", "http://[0:0:0:0:0:0:0:1]", true},
		{"127.0.0.1:80", "http://127.0.0.2", false},
	} {
		req := httptest.NewRequest(http.MethodGet, "http://localhost/", nil)
		req.Host = tc.host
		req.Header.Set("Origin", tc.origin)
		if got := localRequestOriginAllowed(req); got != tc.allowed {
			t.Errorf("host %q origin %q: allowed=%v", tc.host, tc.origin, got)
		}
	}
}
