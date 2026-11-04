package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCase340ExplicitLocalClientRequest(t *testing.T) {
	handler := GuardHTTPAuthority(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }), "127.0.0.1:1789")
	for _, tc := range []struct {
		method, path, origin, site string
		want                       int
	}{
		{"GET", "/services/oauth2/userinfo", "", "", http.StatusNoContent},
		{"POST", "/services/data/v62.0/glade/reset", "", "", http.StatusNoContent},
		{"POST", "/services/data/v67.0/sobjects/Account", "", "", http.StatusNoContent},
		{"POST", "/services/data/v67.0/glade/reset", "http://evil.invalid", "cross-site", http.StatusForbidden},
	} {
		req := httptest.NewRequest(tc.method, "http://localhost:1789"+tc.path, nil)
		req.Header.Set("X-Glade-Client", "1")
		if tc.origin != "" {
			req.Header.Set("Origin", tc.origin)
		}
		if tc.site != "" {
			req.Header.Set("Sec-Fetch-Site", tc.site)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Errorf("%s %s: got %d want %d", tc.method, tc.path, rec.Code, tc.want)
		}
	}
}
