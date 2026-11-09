package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestV035ExposedRoutesRequireAuthority(t *testing.T) {
	h := case340BearerGuard(t, "secret", "0.0.0.0:1789", "example.test:1789")
	token, err := issueToken("user")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ method, path string }{
		{"GET", "/services/apexrest/example"},
		{"GET", "/apex/Example"},
		{"POST", "/services/data/v67.0/actions/custom/apex/Example"},
		{"POST", "/services/data/v67.0/sobjects/Account"},
		{"POST", "/services/data/v67.0/composite"},
		{"GET", "/services/data/v67.0/tooling/query"},
		{"POST", "/services/Soap/s/67.0/example"},
		{"POST", "/lightning/action"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			if got := case340Do(h, tc.method, "example.test:1789", tc.path, map[string]string{"X-Glade-Client": "1"}); got != http.StatusForbidden {
				t.Errorf("without authority: got %d, want 403", got)
			}
			if got := case340Do(h, tc.method, "example.test:1789", tc.path, map[string]string{"Authorization": "Bearer " + token}); got != http.StatusNoContent {
				t.Errorf("issued token: got %d, want 204", got)
			}
			if got := case340Do(h, tc.method, "example.test:1789", tc.path, map[string]string{"X-Glade-Capability": "secret"}); got != http.StatusNoContent {
				t.Errorf("capability: got %d, want 204", got)
			}
		})
	}
}

func TestV035RevokeIssuedToken(t *testing.T) {
	org := testOrg()
	storage.EnsureDeterministicPlatformData(&org)
	s := New(&org)
	token := case340OwnedToken(t, s)
	req := httptest.NewRequest(http.MethodPost, "/services/oauth2/revoke", strings.NewReader("token="+url.QueryEscape(token)))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("revoke status = %d: %s", rec.Code, rec.Body.String())
	}
	if _, ok := issuedTokenUserFor(token, s); ok {
		t.Fatal("revoked token remains valid")
	}
}

func TestV035OtherServerTokenError(t *testing.T) {
	t.Setenv("GLADE_SERVER_CAPABILITY", "")
	org := testOrg()
	storage.EnsureDeterministicPlatformData(&org)
	other := org.Clone()
	token := case340OwnedToken(t, New(&org))
	h := GuardHTTPAuthority(New(&other), "127.0.0.1:1789")
	req := httptest.NewRequest(http.MethodGet, "http://localhost:1789/services/oauth2/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), "invalid or expired local token") {
		t.Fatalf("other server token: %d %s", rec.Code, rec.Body.String())
	}
}
