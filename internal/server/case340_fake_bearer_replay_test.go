package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCase340FakeBearerReplay(t *testing.T) {
	for _, tc := range []struct{ name, capability, addr, host, path string }{
		{"anonymous", "", "127.0.0.1:1789", "localhost:1789", "/services/data/v67.0/tooling/executeAnonymous?anonymousBody=System.debug(1)"},
		{"exposed-query", "secret", "0.0.0.0:1789", "example.test:1789", "/services/data/v67.0/query?q=SELECT+Id+FROM+Account"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GLADE_SERVER_CAPABILITY", tc.capability)
			t.Setenv("GLADE_SERVER_ALLOWED_HOSTS", "")
			h := GuardHTTPAuthority(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }), tc.addr, tc.host)
			req := httptest.NewRequest("GET", "http://"+tc.host+tc.path, nil)
			req.Header.Set("Authorization", "Bearer fake")
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusForbidden {
				t.Errorf("fake bearer got %d, want 403", rec.Code)
			}
		})
	}
}
