package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestV035ExposedDataRoutes(t *testing.T) {
	t.Setenv("GLADE_SERVER_CAPABILITY", "secret")
	t.Setenv("GLADE_SERVER_ALLOWED_HOSTS", "")
	org := testOrg()
	storage.EnsureDeterministicPlatformData(&org)
	s := New(&org)
	token := case340OwnedToken(t, s)
	h := GuardHTTPAuthority(s, "0.0.0.0:1789", "example.test:1789")
	for _, route := range []struct {
		name, method, path, body string
		want                     int
	}{
		{"query", "GET", "/services/data/v67.0/query?q=SELECT+Id+FROM+Account", "", http.StatusOK},
		{"dml", "POST", "/services/data/v67.0/sobjects/Account", `{"Name":"Synthetic"}`, http.StatusCreated},
		{"composite", "POST", "/services/data/v67.0/composite", `{"compositeRequest":[{"method":"GET","url":"/services/data/v67.0/limits","referenceId":"limits"}]}`, http.StatusOK},
		{"tooling", "GET", "/services/data/v67.0/tooling/executeAnonymous?anonymousBody=System.debug(1)", "", http.StatusOK},
	} {
		t.Run(route.name, func(t *testing.T) {
			for _, authorized := range []bool{false, true} {
				req := httptest.NewRequest(route.method, "http://example.test:1789"+route.path, strings.NewReader(route.body))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("X-Glade-Client", "1")
				if authorized {
					req.Header.Set("Authorization", "Bearer "+token)
				}
				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, req)
				if !authorized && rec.Code != http.StatusForbidden {
					t.Fatalf("unauthorized %d: %s", rec.Code, rec.Body.String())
				}
				if authorized && rec.Code != route.want {
					t.Fatalf("authorized %d want %d: %s", rec.Code, route.want, rec.Body.String())
				}
			}
		})
	}
}
