package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func case340OwnedToken(t *testing.T, s *Server) string {
	return case340OwnedTokenFor(t, s, "")
}

func case340OwnedTokenFor(t *testing.T, s *Server, userID string) string {
	t.Helper()
	req := httptest.NewRequest("POST", "/services/oauth2/token", strings.NewReader("grant_type=password"))
	if userID != "" {
		req.Header.Set("X-GLADE-User-Id", userID)
	}
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("token status %d: %s", rec.Code, rec.Body.String())
	}
	var p struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	return p.AccessToken
}

func TestCase340OwnedTokenRejectsOtherServer(t *testing.T) {
	t.Setenv("GLADE_SERVER_CAPABILITY", "")
	a := testOrg()
	storage.EnsureDeterministicPlatformData(&a)
	b := a.Clone()
	first, second := New(&a), New(&b)
	token := case340OwnedToken(t, first)
	h := GuardHTTPAuthority(second, "127.0.0.1:1789")
	req := httptest.NewRequest("GET", "http://localhost:1789/services/oauth2/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("other server accepted token: %d", rec.Code)
	}
}

func TestCase340UnchangedUserChildrenPreserveOwnedToken(t *testing.T) {
	for _, route := range []string{"commit", "http"} {
		t.Run(route, func(t *testing.T) {
			t.Setenv("GLADE_SERVER_CAPABILITY", "")
			org := testOrg()
			storage.EnsureDeterministicPlatformData(&org)
			users := org.Objects["User"]
			u := users.Records[defaultLocalUserID]
			u.Children = map[string][]storage.Record{"Contacts": nil}
			users.Records[u.ID] = u
			org.Objects["User"] = users
			s := New(&org)
			token := case340OwnedToken(t, s)
			if route == "commit" {
				next := org.Clone()
				addAccountForTest(&next, "001000000000123", "Synthetic")
				if err := s.commitOrg(next); err != nil {
					t.Fatal(err)
				}
			} else {
				h := GuardHTTPAuthority(s, "127.0.0.1:1789")
				req := httptest.NewRequest("POST", "http://localhost:1789/services/data/v67.0/glade/fixture?mode=merge", strings.NewReader(`{"version":"glade.storage.v1","objects":[]}`))
				req.Header.Set("X-Glade-Client", "1")
				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, req)
				if rec.Code != http.StatusOK {
					t.Fatalf("fixture status %d: %s", rec.Code, rec.Body.String())
				}
			}
			if _, ok := issuedTokenUserFor(token, s); !ok {
				t.Fatal("unchanged user token revoked")
			}
		})
	}
}
