package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestV035PublicProxyRequiresAuthority(t *testing.T) {
	t.Setenv("GLADE_SERVER_CAPABILITY", "secret")
	t.Setenv("GLADE_SERVER_ALLOWED_HOSTS", "")
	t.Setenv("GLADE_SERVER_PROXY_ORIGINS", "https://preview.example.test")
	h := GuardHTTPAuthority(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }), "127.0.0.1:1789")
	for _, tc := range []struct {
		name, host, origin, cap string
		want                    int
	}{
		{"proxy-unprivileged", "preview.example.test", "https://preview.example.test", "", http.StatusForbidden},
		{"proxy-authorized", "preview.example.test", "https://preview.example.test", "secret", http.StatusNoContent},
		{"direct-loopback", "localhost:1789", "http://localhost:1789", "", http.StatusNoContent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://localhost:1789/services/data/v67.0/query", nil)
			req.Host = tc.host
			req.Header.Set("Origin", tc.origin)
			req.Header.Set("Sec-Fetch-Site", "same-origin")
			req.Header.Set("X-Glade-Capability", tc.cap)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Errorf("got %d want %d", rec.Code, tc.want)
			}
		})
	}
}
