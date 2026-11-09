package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestV035RevokeUnknownAndOtherServerTokens(t *testing.T) {
	org := testOrg()
	storage.EnsureDeterministicPlatformData(&org)
	other := org.Clone()
	first, second := New(&org), New(&other)
	token := case340OwnedToken(t, first)
	for _, value := range []string{"synthetic-invalid-token", token} {
		req := httptest.NewRequest(http.MethodPost, "/services/oauth2/revoke", strings.NewReader(url.Values{"token": {value}}.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		second.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("invalid/foreign token status %d, want 400", rec.Code)
		}
	}
	if _, ok := issuedTokenUserFor(token, first); !ok {
		t.Fatal("other server revoked token")
	}
}
