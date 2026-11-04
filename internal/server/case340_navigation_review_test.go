package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCase340NavigationRequiresCapabilityForDynamicRoutes(t *testing.T) {
	t.Setenv("GLADE_SERVER_CAPABILITY", "test-secret")
	handler := GuardHTTPAuthority(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }), "127.0.0.1:1789")
	for _, path := range []string{"/services/apexrest/Synthetic", "/apex/SyntheticPage", "/db", "/unknown-dynamic"} {
		for _, capability := range []string{"", "bad", "test-secret"} {
			req := httptest.NewRequest(http.MethodGet, "http://localhost:1789"+path, nil)
			req.Header.Set("Sec-Fetch-Site", "none")
			req.Header.Set("X-Glade-Capability", capability)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			want := http.StatusOK
			if capability == "test-secret" {
				want = http.StatusNoContent
			}
			if rec.Code != want {
				t.Errorf("%s capability=%q got=%d want=%d", path, capability, rec.Code, want)
			}
		}
	}
}
