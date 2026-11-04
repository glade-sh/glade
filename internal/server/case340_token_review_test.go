package server

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestCase340ReviewExposedTokenRequiresCapability(t *testing.T) {
	t.Setenv("GLADE_SERVER_CAPABILITY", "test-secret")
	t.Setenv("GLADE_SERVER_ALLOWED_HOSTS", "")
	h := GuardHTTPAuthority(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }), "0.0.0.0:1789", "example.test:1789")
	for _, tc := range []struct {
		path, header, value string
		want                int
	}{
		{"/services/oauth2/token", "X-Glade-Client", "1", http.StatusForbidden},
		{"/services//oauth2/token/", "X-Glade-Client", "1", http.StatusForbidden},
		{"/services/oauth2/token", "X-Glade-Capability", "wrong", http.StatusForbidden},
		{"/services/oauth2/token", "X-Glade-Capability", "test-secret", http.StatusNoContent},
	} {
		req := httptest.NewRequest(http.MethodPost, "http://example.test:1789"+tc.path, nil)
		req.Header.Set(tc.header, tc.value)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Errorf("%s %s: got %d want %d", tc.path, tc.header, rec.Code, tc.want)
		}
	}
}

func TestCase340ReviewTokenEntropyAndBinding(t *testing.T) {
	org := testOrg()
	storage.EnsureDeterministicPlatformData(&org)
	addUser(&org, "005000000000123", "user@example.test", "user.alias@example.test", "Test User")
	h := New(&org)
	var tokens []string
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/services/oauth2/token", strings.NewReader("grant_type=password"))
		req.Header.Set("X-GLADE-User-Id", "005000000000123")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("token endpoint status %d", rec.Code)
		}
		var payload struct {
			AccessToken string `json:"access_token"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(payload.AccessToken, "glade_"))
		if err != nil || len(raw) < 16 {
			t.Fatalf("token lacks required random bytes: decoded length %d, error %v", len(raw), err)
		}
		if got := bearerUserID("Bearer " + payload.AccessToken); got != "005000000000123" {
			t.Fatalf("issued bearer user = %q", got)
		}
		tokens = append(tokens, payload.AccessToken)
	}
	if tokens[0] == tokens[1] {
		t.Fatal("two issuances returned the same token")
	}
}
