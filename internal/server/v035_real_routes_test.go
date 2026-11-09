package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestV035ExposedServerRoutes(t *testing.T) {
	t.Setenv("GLADE_SERVER_CAPABILITY", "secret")
	t.Setenv("GLADE_SERVER_ALLOWED_HOSTS", "")
	org := testOrg()
	storage.EnsureDeterministicPlatformData(&org)
	s := New(&org)
	token := case340OwnedToken(t, s)
	h := GuardHTTPAuthority(s, "0.0.0.0:1789", "example.test:1789")
	for _, tc := range []struct{ method, path string }{
		{"GET", "/services/apexrest/Example"}, {"GET", "/apex/Example"},
		{"POST", "/lightning/apex/Example/run"}, {"POST", "/lightning/wire/apex"},
		{"POST", "/services/data/v62.0/sobjects/Account"}, {"POST", "/services/data/v67.0/composite"},
		{"GET", "/services/data/v67.0/tooling/query?q=SELECT+Id+FROM+ApexClass"},
		{"GET", "/services/data/v67.0/query?q=SELECT+Id+FROM+Account"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			request := func(bearer bool) int {
				req := httptest.NewRequest(tc.method, "http://example.test:1789"+tc.path, strings.NewReader("{}"))
				req.Header.Set("X-Glade-Client", "1")
				if bearer {
					req.Header.Set("Authorization", "Bearer "+token)
				}
				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, req)
				return rec.Code
			}
			if got := request(false); got != http.StatusForbidden {
				t.Errorf("unauthorized: got %d", got)
			}
			if got := request(true); got == http.StatusForbidden || got == http.StatusUnauthorized {
				t.Errorf("issued token rejected: %d", got)
			}
		})
	}
}
