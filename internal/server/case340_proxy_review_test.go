package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestCase340ProxyExplicitOrigins(t *testing.T) {
	t.Setenv("GLADE_SERVER_PROXY_ORIGINS", "https://preview.example.test,https://preview.example.test:8443,http://dev.example.test:9000")
	for _, tc := range []struct {
		name, host, origin, site, forwarded string
		allowed                             bool
	}{
		{"https default", "preview.example.test", "https://preview.example.test", "same-origin", "", true},
		{"https explicit default", "preview.example.test:443", "https://preview.example.test", "same-origin", "http", true},
		{"https alternate", "preview.example.test:8443", "https://preview.example.test:8443", "same-origin", "https", true},
		{"http alternate", "dev.example.test:9000", "http://dev.example.test:9000", "same-origin", "https", true},
		{"foreign origin", "preview.example.test", "https://evil.example.test", "cross-site", "https", false},
		{"different origin port", "preview.example.test", "https://preview.example.test:8443", "same-origin", "https", false},
		{"unconfigured authority", "evil.example.test", "https://evil.example.test", "same-origin", "https", false},
		{"cross-site no origin", "preview.example.test", "", "cross-site", "https", false},
		{"scheme forgery", "preview.example.test", "http://preview.example.test", "same-origin", "https", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, path := range []string{"/services/data/", "/lightning/glade.out.js", "/apex/SyntheticPage"} {
				org := storage.NewOrgState()
				handler := GuardHTTPAuthority(New(&org), "127.0.0.1:1789")
				req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:1789"+path, nil)
				req.Host = tc.host
				if tc.origin != "" {
					req.Header.Set("Origin", tc.origin)
				}
				if tc.site != "" {
					req.Header.Set("Sec-Fetch-Site", tc.site)
				}
				if tc.forwarded != "" {
					req.Header.Set("X-Forwarded-Proto", tc.forwarded)
					req.Header.Set("X-Forwarded-Host", "evil.example.test")
				}
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)
				if (rec.Code != http.StatusForbidden) != tc.allowed {
					t.Errorf("%s: status=%d allowed=%v", path, rec.Code, tc.allowed)
				}
			}
		})
	}
}

func TestCase340ProxyForgedHeadersDoNotAuthorize(t *testing.T) {
	t.Setenv("GLADE_SERVER_PROXY_ORIGINS", "https://trusted.example.test")
	for _, host := range []string{"evil.example.test", "trusted.example.test:8443", "127.0.0.1:1789"} {
		org := storage.NewOrgState()
		handler := GuardHTTPAuthority(New(&org), "127.0.0.1:1789")
		req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:1789/services/data/", nil)
		req.Host = host
		req.Header.Set("Origin", "https://trusted.example.test")
		req.Header.Set("X-Forwarded-Proto", "https")
		req.Header.Set("X-Forwarded-Host", "trusted.example.test")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("Host %q: status=%d, want 403", host, rec.Code)
		}
	}
}

func TestCase340ProxyGuardEnforcesOriginForAnyHandler(t *testing.T) {
	t.Setenv("GLADE_SERVER_PROXY_ORIGINS", "https://preview.example.test")
	handler := GuardHTTPAuthority(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }), "127.0.0.1:1789")
	for _, tc := range []struct {
		host, origin, site string
		want               int
	}{
		{"preview.example.test", "https://preview.example.test", "same-origin", http.StatusNoContent},
		{"preview.example.test", "https://evil.example.test", "cross-site", http.StatusForbidden},
		{"preview.example.test", "http://preview.example.test", "same-origin", http.StatusForbidden},
		{"127.0.0.1:1789", "http://evil.example.test", "cross-site", http.StatusForbidden},
		{"127.0.0.1:1789", "http://127.0.0.1:1789", "same-origin", http.StatusNoContent},
	} {
		req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:1789/", nil)
		req.Host = tc.host
		req.Header.Set("Origin", tc.origin)
		req.Header.Set("Sec-Fetch-Site", tc.site)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Errorf("host %q origin %q: status=%d want=%d", tc.host, tc.origin, rec.Code, tc.want)
		}
	}
}

func TestCase340ProxyInvalidOriginConfigurationFailsClosed(t *testing.T) {
	for _, configured := range []string{"https://user@preview.example.test", "https://preview.example.test/path", "https://preview.example.test:99999", "https://preview.example.test?x=1", "https://preview.example.test#fragment"} {
		t.Run(configured, func(t *testing.T) {
			t.Setenv("GLADE_SERVER_PROXY_ORIGINS", configured)
			org := storage.NewOrgState()
			handler := GuardHTTPAuthority(New(&org), "127.0.0.1:1789")
			req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:1789/services/data/", nil)
			req.Host = "preview.example.test"
			req.Header.Set("Origin", "https://preview.example.test")
			req.Header.Set("X-Forwarded-Proto", "https")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusForbidden {
				t.Errorf("configuration %q: status=%d, want 403", configured, rec.Code)
			}
		})
	}
}
