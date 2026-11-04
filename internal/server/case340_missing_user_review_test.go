package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestCase340IssuedTokenRejectsRemovedUser(t *testing.T) {
	t.Setenv("GLADE_SERVER_CAPABILITY", "")
	t.Setenv("GLADE_SERVER_ALLOWED_HOSTS", "")
	org := testOrg()
	storage.EnsureDeterministicPlatformData(&org)
	addUser(&org, "005000000000123", "user@example.test", "user.alias@example.test", "Test User")
	token, err := issueToken("005000000000123")
	if err != nil {
		t.Fatal(err)
	}
	h := GuardHTTPAuthority(New(&org), "127.0.0.1:1789")
	user := org.Objects["User"]
	delete(user.Records, "005000000000123")
	org.Objects["User"] = user
	for _, path := range []string{"/services/oauth2/userinfo", "/services/data/v67.0/query?q=SELECT+Id+FROM+Account", "/services/data/v67.0/sobjects/Account"} {
		req := httptest.NewRequest("GET", "http://localhost:1789"+path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("removed user %s: got %d want 401", path, rec.Code)
		}
	}
}

func TestCase340UserResetAndFixtureReplaceInvalidateTokens(t *testing.T) {
	for _, tc := range []struct{ path, body string }{
		{"/services/data/v67.0/glade/reset?scope=users", ""},
		{"/services/data/v67.0/glade/fixture?mode=replace", `{"version":"glade.storage.v1","objects":[]}`},
	} {
		t.Run(tc.path, func(t *testing.T) {
			t.Setenv("GLADE_SERVER_CAPABILITY", "")
			t.Setenv("GLADE_SERVER_ALLOWED_HOSTS", "")
			org := testOrg()
			storage.EnsureDeterministicPlatformData(&org)
			token, err := issueToken(string(defaultLocalUserID))
			if err != nil {
				t.Fatal(err)
			}
			h := GuardHTTPAuthority(New(&org), "127.0.0.1:1789")
			req := httptest.NewRequest("POST", "http://localhost:1789"+tc.path, strings.NewReader(tc.body))
			req.Header.Set("X-Glade-Client", "1")
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("reset status = %d body=%s", rec.Code, rec.Body.String())
			}
			get := httptest.NewRequest("GET", "http://localhost:1789/services/oauth2/userinfo", nil)
			get.Header.Set("Authorization", "Bearer "+token)
			after := httptest.NewRecorder()
			h.ServeHTTP(after, get)
			if after.Code != http.StatusUnauthorized {
				t.Fatalf("token after reset = %d, want 401", after.Code)
			}
		})
	}
}

func TestCase340ResetBetweenGuardAndDispatchRejectsToken(t *testing.T) {
	t.Setenv("GLADE_SERVER_CAPABILITY", "")
	t.Setenv("GLADE_SERVER_ALLOWED_HOSTS", "")
	org := testOrg()
	storage.EnsureDeterministicPlatformData(&org)
	token, err := issueToken(string(defaultLocalUserID))
	if err != nil {
		t.Fatal(err)
	}
	s := New(&org)
	h := GuardHTTPAuthority(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clearIssuedTokens()
		s.ServeHTTP(w, r)
	}), "127.0.0.1:1789")
	req := httptest.NewRequest("GET", "http://localhost:1789/services/oauth2/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("token reset before dispatch: got %d want 401", rec.Code)
	}
}

func TestCase340IssuedTokenRejectsSoftDeletedUser(t *testing.T) {
	t.Setenv("GLADE_SERVER_CAPABILITY", "")
	org := testOrg()
	storage.EnsureDeterministicPlatformData(&org)
	addUser(&org, "005000000000123", "user@example.test", "user.alias@example.test", "Test User")
	token, err := issueToken("005000000000123")
	if err != nil {
		t.Fatal(err)
	}
	users := org.Objects["User"]
	record := users.Records["005000000000123"]
	record.System.IsDeleted = true
	users.Records[record.ID] = record
	org.Objects["User"] = users
	h := GuardHTTPAuthority(New(&org), "127.0.0.1:1789")
	for _, path := range []string{"/services/oauth2/userinfo", "/services/data/v67.0/sobjects/Account"} {
		req := httptest.NewRequest("GET", "http://localhost:1789"+path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("soft-deleted user %s: got %d want 401", path, rec.Code)
		}
	}
}

func TestCase340FixtureMergeInvalidatesTokens(t *testing.T) {
	t.Setenv("GLADE_SERVER_CAPABILITY", "")
	org := testOrg()
	storage.EnsureDeterministicPlatformData(&org)
	token, err := issueToken(string(defaultLocalUserID))
	if err != nil {
		t.Fatal(err)
	}
	body := `{"version":"glade.storage.v1","objects":[{"name":"User","records":[{"id":"` + string(defaultLocalUserID) + `","fields":{"LastName":{"kind":"string","string":"Replacement"}}}]}]}`
	h := GuardHTTPAuthority(New(&org), "127.0.0.1:1789")
	req := httptest.NewRequest("POST", "http://localhost:1789/services/data/v67.0/glade/fixture?mode=merge", strings.NewReader(body))
	req.Header.Set("X-Glade-Client", "1")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("fixture merge status %d body=%s", rec.Code, rec.Body.String())
	}
	get := httptest.NewRequest("GET", "http://localhost:1789/services/oauth2/userinfo", nil)
	get.Header.Set("Authorization", "Bearer "+token)
	after := httptest.NewRecorder()
	h.ServeHTTP(after, get)
	if after.Code != http.StatusUnauthorized {
		t.Fatalf("token after fixture merge = %d, want 401", after.Code)
	}
}

func TestCase340UserDeletionAndRestoreInvalidatesTokens(t *testing.T) {
	t.Setenv("GLADE_SERVER_CAPABILITY", "")
	org := testOrg()
	storage.EnsureDeterministicPlatformData(&org)
	addUser(&org, "005000000000123", "user@example.test", "user.alias@example.test", "Test User")
	token, err := issueToken("005000000000123")
	if err != nil {
		t.Fatal(err)
	}
	s := New(&org)
	next := org.Clone()
	users := next.Objects["User"]
	record := users.Records["005000000000123"]
	record.System.IsDeleted = true
	users.Records[record.ID] = record
	next.Objects["User"] = users
	if err := s.commitOrg(next); err != nil {
		t.Fatal(err)
	}
	next = org.Clone()
	users = next.Objects["User"]
	record = users.Records["005000000000123"]
	record.System.IsDeleted = false
	users.Records[record.ID] = record
	next.Objects["User"] = users
	if err := s.commitOrg(next); err != nil {
		t.Fatal(err)
	}
	h := GuardHTTPAuthority(s, "127.0.0.1:1789")
	req := httptest.NewRequest("GET", "http://localhost:1789/services/oauth2/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("token after user deletion and restore = %d, want 401", rec.Code)
	}
}

func TestCase340NonUserCommitPreservesIssuedToken(t *testing.T) {
	org := testOrg()
	storage.EnsureDeterministicPlatformData(&org)
	token, err := issueToken(string(defaultLocalUserID))
	if err != nil {
		t.Fatal(err)
	}
	s := New(&org)
	next := org.Clone()
	addAccountForTest(&next, "001000000000123", "Synthetic")
	if err := s.commitOrg(next); err != nil {
		t.Fatal(err)
	}
	if _, ok := issuedTokenUser(token); !ok {
		t.Fatal("non-user update invalidated token")
	}
}

func TestCase340OtherUserChangePreservesToken(t *testing.T) {
	org := testOrg()
	storage.EnsureDeterministicPlatformData(&org)
	addUser(&org, "005000000000123", "one@example.test", "one.alias@example.test", "First User")
	addUser(&org, "005000000000124", "two@example.test", "two.alias@example.test", "Second User")
	first, err := issueToken("005000000000123")
	if err != nil {
		t.Fatal(err)
	}
	second, err := issueToken("005000000000124")
	if err != nil {
		t.Fatal(err)
	}
	s := New(&org)
	next := org.Clone()
	users := next.Objects["User"]
	record := users.Records["005000000000124"]
	record.System.IsDeleted = true
	users.Records[record.ID] = record
	next.Objects["User"] = users
	if err := s.commitOrg(next); err != nil {
		t.Fatal(err)
	}
	if _, ok := issuedTokenUser(first); !ok {
		t.Fatal("unrelated user's token was revoked")
	}
	if _, ok := issuedTokenUser(second); ok {
		t.Fatal("changed user's token was retained")
	}
}

func TestCase340NonUserFixtureMergePreservesToken(t *testing.T) {
	t.Setenv("GLADE_SERVER_CAPABILITY", "")
	org := testOrg()
	storage.EnsureDeterministicPlatformData(&org)
	token, err := issueToken(string(defaultLocalUserID))
	if err != nil {
		t.Fatal(err)
	}
	h := GuardHTTPAuthority(New(&org), "127.0.0.1:1789")
	body := `{"version":"glade.storage.v1","objects":[{"name":"Account","records":[{"fields":{"Name":{"kind":"string","string":"Synthetic"}}}]}]}`
	req := httptest.NewRequest("POST", "http://localhost:1789/services/data/v67.0/glade/fixture?mode=merge", strings.NewReader(body))
	req.Header.Set("X-Glade-Client", "1")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("fixture merge status %d", rec.Code)
	}
	if _, ok := issuedTokenUser(token); !ok {
		t.Fatal("non-user fixture merge revoked token")
	}
}

func TestCase340ApexRestUserMutationRevokesToken(t *testing.T) {
	t.Setenv("GLADE_SERVER_CAPABILITY", "")
	org := testOrg()
	storage.EnsureDeterministicPlatformData(&org)
	addUser(&org, "005000000000123", "user@example.test", "user.alias@example.test", "Test User")
	token, err := issueToken("005000000000123")
	if err != nil {
		t.Fatal(err)
	}
	s := New(&org)
	s.SetProjectIndex(writeApexRestProject(t, map[string]string{"SyntheticResource.cls": `@RestResource(urlMapping='/synthetic-user') global class SyntheticResource { @HttpPost global static void updateUser() { User u = [SELECT Id, LastName FROM User WHERE Id = '005000000000123']; u.LastName = 'Changed'; update u; } }`}))
	h := GuardHTTPAuthority(s, "127.0.0.1:1789")
	req := httptest.NewRequest("POST", "http://localhost:1789/services/apexrest/synthetic-user", nil)
	req.Header.Set("X-Glade-Client", "1")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK && rec.Code != http.StatusNoContent {
		t.Fatalf("Apex REST mutation status %d body=%s", rec.Code, rec.Body.String())
	}
	if _, ok := issuedTokenUser(token); ok {
		t.Fatal("Apex REST user change retained token")
	}
}

func TestCase340OtherServerUserChangePreservesToken(t *testing.T) {
	newServer := func() *Server { org := testOrg(); storage.EnsureDeterministicPlatformData(&org); return New(&org) }
	first, second := newServer(), newServer()
	issue := func(s *Server) string {
		req := httptest.NewRequest("POST", "/services/oauth2/token", strings.NewReader("grant_type=password"))
		rec := httptest.NewRecorder()
		s.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("token status %d", rec.Code)
		}
		var payload struct {
			AccessToken string `json:"access_token"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		return payload.AccessToken
	}
	firstToken, secondToken := issue(first), issue(second)
	next := first.Org.Clone()
	users := next.Objects["User"]
	record := users.Records[defaultLocalUserID]
	record.System.IsDeleted = true
	users.Records[record.ID] = record
	next.Objects["User"] = users
	if err := first.commitOrg(next); err != nil {
		t.Fatal(err)
	}
	if _, ok := issuedTokenUser(firstToken); ok {
		t.Error("changed server token was retained")
	}
	if _, ok := issuedTokenUser(secondToken); !ok {
		t.Error("other server token was revoked")
	}
}
