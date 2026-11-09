package server

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestCase340HostAuthorityGuard(t *testing.T) {
	t.Setenv("GLADE_SERVER_CAPABILITY", "host-test-secret")
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	for _, tc := range []struct {
		addr, host, forwarded string
		want                  int
	}{
		{"127.0.0.1:1789", "localhost:1789", "", http.StatusNoContent},
		{"127.0.0.1:1789", "127.0.0.1:1789", "", http.StatusNoContent},
		{"127.0.0.1:1789", "attacker.invalid:1789", "localhost:1789", http.StatusForbidden},
		{"127.0.0.1:1789", "localhost:1790", "", http.StatusForbidden},
		{"127.0.0.1:1789", "localhost:1789@attacker.invalid", "", http.StatusForbidden},
		{"preview.example:1789", "preview.example:1789", "", http.StatusNoContent},
		{"preview.example:1789", "other.example:1789", "preview.example:1789", http.StatusForbidden},
	} {
		t.Run(tc.addr+"/"+tc.host, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://localhost/", nil)
			req.Host = tc.host
			req.Header.Set("X-Forwarded-Host", tc.forwarded)
			req.Header.Set("X-Glade-Capability", "host-test-secret")
			req.Header.Set("Sec-Fetch-Site", "same-origin")
			rec := httptest.NewRecorder()
			GuardHTTPAuthority(next, tc.addr).ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}

func TestCase340HostGuardUsesActualEphemeralPort(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	req := httptest.NewRequest(http.MethodGet, "http://localhost:32123/", nil)
	req = req.WithContext(context.WithValue(req.Context(), http.LocalAddrContextKey, &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 32123}))
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	rec := httptest.NewRecorder()
	GuardHTTPAuthority(next, "127.0.0.1:0").ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestCase340AnonymousGETRequiresCapability(t *testing.T) {
	t.Setenv("GLADE_SERVER_CAPABILITY", "test-only-secret")
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	for _, tc := range []struct {
		capability string
		want       int
	}{
		{"", http.StatusForbidden}, {"wrong", http.StatusForbidden}, {"test-only-secret", http.StatusNoContent},
	} {
		req := httptest.NewRequest(http.MethodGet, "http://localhost:1789/services/data/v67.0/tooling/executeAnonymous?anonymousBody=System.debug(1)", nil)
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		req.Header.Set("X-Glade-Capability", tc.capability)
		rec := httptest.NewRecorder()
		GuardHTTPAuthority(next, "127.0.0.1:1789").ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Fatalf("capability %q: status = %d, want %d", tc.capability, rec.Code, tc.want)
		}
	}
}

func TestCase340ConfiguredHostnameWithResolvedListener(t *testing.T) {
	t.Setenv("GLADE_SERVER_CAPABILITY", "host-test-secret")
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	req := httptest.NewRequest(http.MethodGet, "http://preview.example:1789/", nil)
	req.Header.Set("X-Glade-Capability", "host-test-secret")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	rec := httptest.NewRecorder()
	GuardHTTPAuthority(next, "192.0.2.10:1789", "preview.example:1789").ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestCase340DefaultHTTPPortMayOmitPort(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	req := httptest.NewRequest(http.MethodGet, "http://localhost/", nil)
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	rec := httptest.NewRecorder()
	GuardHTTPAuthority(next, "127.0.0.1:80").ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestCase340AnonymousGETPathVariantsRequireCapability(t *testing.T) {
	t.Setenv("GLADE_SERVER_CAPABILITY", "test-only-secret")
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	for _, path := range []string{
		"/services/data/v62.0/tooling/executeAnonymous/",
		"/services//data/v67.0/tooling/%65xecuteAnonymous",
	} {
		req := httptest.NewRequest(http.MethodGet, "http://localhost:1789"+path, nil)
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		rec := httptest.NewRecorder()
		GuardHTTPAuthority(next, "127.0.0.1:1789").ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("%s: status = %d", path, rec.Code)
		}
	}
}

func TestCase340HostGuardCoversPreviewRoutes(t *testing.T) {
	for _, path := range []string{"/apex/SyntheticPage", "/lightning/glade.out.js", "/services/data/v67.0/"} {
		for _, tc := range []struct {
			host    string
			blocked bool
		}{{"attacker.invalid:1789", true}, {"localhost:1789", false}} {
			org := storage.NewOrgState()
			handler := GuardHTTPAuthority(New(&org), "127.0.0.1:1789")
			req := httptest.NewRequest(http.MethodGet, "http://localhost:1789"+path, nil)
			req.Host = tc.host
			req.Header.Set("Sec-Fetch-Site", "same-origin")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if (rec.Code == http.StatusForbidden) != tc.blocked {
				t.Fatalf("%s host %s: status = %d", path, tc.host, rec.Code)
			}
		}
	}
}
