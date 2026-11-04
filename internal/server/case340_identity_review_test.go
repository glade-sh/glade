package server

import (
	"crypto/sha256"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCase340LoopbackConfiguredHostsRemainLocal(t *testing.T) {
	t.Setenv("GLADE_SERVER_CAPABILITY", "")
	t.Setenv("GLADE_SERVER_ALLOWED_HOSTS", "")
	for _, addr := range []string{"localhost:1789", "127.0.0.1:1789", "[::1]:1789"} {
		t.Run(addr, func(t *testing.T) {
			h := GuardHTTPAuthority(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }), addr, addr)
			for _, tc := range []struct{ method, path, header, value string }{
				{"POST", "/services/oauth2/token", "X-Glade-Client", "1"},
				{"GET", "/services/data/v62.0/query", "Authorization", "Bearer legacy-local-client"},
				{"POST", "/services/data/v67.0/sobjects/Account", "Authorization", "Bearer legacy-local-client"},
			} {
				req := httptest.NewRequest(tc.method, "http://"+addr+tc.path, nil)
				req.Header.Set(tc.header, tc.value)
				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, req)
				if rec.Code != http.StatusNoContent {
					t.Errorf("%s: got %d want 204", tc.path, rec.Code)
				}
			}
		})
	}
}

func TestCase340EvictedTokenCannotFallBackToDefaultUser(t *testing.T) {
	t.Setenv("GLADE_SERVER_CAPABILITY", "")
	t.Setenv("GLADE_SERVER_ALLOWED_HOSTS", "")
	token, err := issueToken("005000000000123")
	if err != nil {
		t.Fatal(err)
	}
	h := GuardHTTPAuthority(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }), "127.0.0.1:1789")
	request := func(path string) int {
		req := httptest.NewRequest("GET", "http://localhost:1789"+path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	if got := request("/services/oauth2/userinfo"); got != http.StatusNoContent {
		t.Fatalf("valid token got %d", got)
	}
	hash := sha256.Sum256([]byte(token))
	issuedTokens.Lock()
	el := issuedTokens.byHash[hash]
	delete(issuedTokens.byHash, hash)
	issuedTokens.order.Remove(el)
	issuedTokens.Unlock()
	for _, path := range []string{"/services/oauth2/userinfo", "/services/data/v62.0/query", "/services/data/v67.0/sobjects/Account"} {
		if got := request(path); got != http.StatusUnauthorized {
			t.Errorf("evicted token %s: got %d want 401", path, got)
		}
	}
}

func TestCase340IssuedIdentitySurvivesEvictionDuringRequest(t *testing.T) {
	t.Setenv("GLADE_SERVER_CAPABILITY", "")
	t.Setenv("GLADE_SERVER_ALLOWED_HOSTS", "")
	token, err := issueToken("005000000000123")
	if err != nil {
		t.Fatal(err)
	}
	h := GuardHTTPAuthority(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hash := sha256.Sum256([]byte(token))
		issuedTokens.Lock()
		el := issuedTokens.byHash[hash]
		delete(issuedTokens.byHash, hash)
		issuedTokens.order.Remove(el)
		issuedTokens.Unlock()
		_, _ = w.Write([]byte(selectedUserID(r, "")))
	}), "127.0.0.1:1789")
	req := httptest.NewRequest("GET", "http://localhost:1789/services/oauth2/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "005000000000123" {
		t.Fatalf("identity after eviction: status %d, identity matches = %v", rec.Code, rec.Body.String() == "005000000000123")
	}
}

func TestCase340DuplicateAuthorizationRejected(t *testing.T) {
	t.Setenv("GLADE_SERVER_CAPABILITY", "")
	h := GuardHTTPAuthority(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }), "127.0.0.1:1789")
	req := httptest.NewRequest("GET", "http://localhost:1789/services/oauth2/userinfo", nil)
	req.Header.Set("X-Glade-Client", "1")
	req.Header.Add("Authorization", "Bearer glade_expired")
	req.Header.Add("Authorization", "Bearer other")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("duplicate authorization status %d, want 401", rec.Code)
	}
}
