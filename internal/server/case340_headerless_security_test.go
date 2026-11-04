package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCase340HeaderlessMutationRequiresCapability(t *testing.T) {
	t.Setenv("GLADE_SERVER_CAPABILITY", "test-secret")
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	handler := GuardHTTPAuthority(next, "127.0.0.1:1789")
	for _, tc := range []struct{ method, path string }{
		{"GET", "/services/apexrest/Synthetic"},
		{"GET", "/apex/SyntheticPage"},
		{"POST", "/services/data/v62.0/glade/reset"},
		{"POST", "/services/data/v67.0/sobjects/Account"},
		{"GET", "/unknown-dynamic-handler"},
	} {
		for _, auth := range []struct {
			header, value string
			want          int
		}{
			{"", "", http.StatusForbidden},
			{"X-Glade-Capability", "bad", http.StatusForbidden},
			{"X-Glade-Capability", "test-secret", http.StatusNoContent},
			{"Origin", "http://localhost:1789", http.StatusNoContent},
			{"Sec-Fetch-Site", "same-origin", http.StatusNoContent},
			{"Sec-Fetch-Site", "none", http.StatusOK},
			{"Sec-Fetch-Site", "unexpected", http.StatusForbidden},
		} {
			req := httptest.NewRequest(tc.method, "http://localhost:1789"+tc.path, nil)
			if auth.header != "" {
				req.Header.Set(auth.header, auth.value)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			want := auth.want
			if tc.method != http.MethodGet && auth.header == "Sec-Fetch-Site" && auth.value == "none" {
				want = http.StatusForbidden
			}
			if rec.Code != want {
				t.Errorf("%s %s %s=%s: status %d, want %d", tc.method, tc.path, auth.header, auth.value, rec.Code, want)
			}
		}
	}
}
