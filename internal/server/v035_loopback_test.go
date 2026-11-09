package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestV035LoopbackWithAdditionalHostsKeepsPolicy(t *testing.T) {
	t.Setenv("GLADE_SERVER_CAPABILITY", "")
	t.Setenv("GLADE_SERVER_ALLOWED_HOSTS", "preview.example")
	h := GuardHTTPAuthority(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }), "127.0.0.1:1789")
	req := httptest.NewRequest(http.MethodGet, "http://localhost:1789/services/data/v67.0/query", nil)
	req.Header.Set("X-Glade-Client", "1")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("loopback request got %d, want 204", rec.Code)
	}
}
