package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestV035RevokeNativeMethods(t *testing.T) {
	org := testOrg()
	storage.EnsureDeterministicPlatformData(&org)
	s := New(&org)
	for _, method := range []string{"GET", "POST"} {
		for _, value := range []string{"", "invalid-v035"} {
			req := httptest.NewRequest(method, "/services/oauth2/revoke?"+url.Values{"token": {value}}.Encode(), nil)
			rec := httptest.NewRecorder()
			s.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("%s invalid/missing token got %d want 400", method, rec.Code)
			}
		}
	}
	token := case340OwnedToken(t, s)
	req := httptest.NewRequest("GET", "/services/oauth2/revoke?"+url.Values{"token": {token}}.Encode(), nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET valid token: %d %s", rec.Code, rec.Body.String())
	}
	if _, ok := issuedTokenUserFor(token, s); ok {
		t.Fatal("GET did not revoke token")
	}
	req = httptest.NewRequest("PUT", "/services/oauth2/revoke", strings.NewReader(""))
	rec = httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("PUT got %d", rec.Code)
	}
}
