package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const case340AnonPath = "/services/data/v67.0/tooling/executeAnonymous?anonymousBody=System.debug(1)"

func case340BearerGuard(t *testing.T, capability string, addr string, hosts ...string) http.Handler {
	t.Helper()
	t.Setenv("GLADE_SERVER_CAPABILITY", capability)
	t.Setenv("GLADE_SERVER_ALLOWED_HOSTS", "")
	return GuardHTTPAuthority(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), addr, hosts...)
}

func case340Do(h http.Handler, method, host, path string, headers map[string]string) int {
	req := httptest.NewRequest(method, "http://"+host+path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code
}

func TestCase340ExecuteAnonymousFakeBearerRejected(t *testing.T) {
	h := case340BearerGuard(t, "", "127.0.0.1:1789")
	if got := case340Do(h, "GET", "localhost:1789", case340AnonPath, map[string]string{"Authorization": "Bearer fake"}); got != http.StatusForbidden {
		t.Fatalf("fake bearer status = %d, want 403", got)
	}
}

func TestCase340ExecuteAnonymousIssuedTokenAccepted(t *testing.T) {
	h := case340BearerGuard(t, "", "127.0.0.1:1789")
	token, err := issueToken("case340-user")
	if err != nil {
		t.Fatal(err)
	}
	if got := case340Do(h, "GET", "localhost:1789", case340AnonPath, map[string]string{"Authorization": "Bearer " + token}); got != http.StatusNoContent {
		t.Fatalf("issued token status = %d, want 204", got)
	}
}

func TestCase340ExecuteAnonymousCapabilityAccepted(t *testing.T) {
	h := case340BearerGuard(t, "case340-secret", "127.0.0.1:1789")
	if got := case340Do(h, "GET", "localhost:1789", case340AnonPath, map[string]string{"X-Glade-Capability": "case340-secret"}); got != http.StatusNoContent {
		t.Fatalf("capability status = %d, want 204", got)
	}
}

func TestCase340ExposedServerFakeBearerRejected(t *testing.T) {
	h := case340BearerGuard(t, "case340-secret", "0.0.0.0:1789", "example.test:1789")
	fake := map[string]string{"Authorization": "Bearer fake"}
	for _, tc := range []struct{ method, path string }{
		{"GET", case340AnonPath},
		{"POST", "/services/data/v67.0/sobjects/Account"},
		{"GET", "/services/data/v67.0/query?q=SELECT+Id+FROM+Account"},
		{"POST", "/services/oauth2/token"},
	} {
		if got := case340Do(h, tc.method, "example.test:1789", tc.path, fake); got != http.StatusForbidden {
			t.Errorf("%s %s fake bearer = %d, want 403", tc.method, tc.path, got)
		}
	}
	token, err := issueToken("case340-user")
	if err != nil {
		t.Fatal(err)
	}
	if got := case340Do(h, "GET", "example.test:1789", case340AnonPath, map[string]string{"Authorization": "Bearer " + token}); got != http.StatusNoContent {
		t.Errorf("issued token on exposed server = %d, want 204", got)
	}
	if got := case340Do(h, "GET", "example.test:1789", case340AnonPath, map[string]string{"X-Glade-Capability": "case340-secret"}); got != http.StatusNoContent {
		t.Errorf("capability on exposed server = %d, want 204", got)
	}
}

func case340Token(t *testing.T, h http.Handler, host string, headers map[string]string) (int, string) {
	t.Helper()
	req := httptest.NewRequest("POST", "http://"+host+"/services/oauth2/token", nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code, rec.Body.String()
}

func TestCase340ExposedTokenIssuanceRequiresCapability(t *testing.T) {
	var issued string
	t.Setenv("GLADE_SERVER_CAPABILITY", "case340-secret")
	t.Setenv("GLADE_SERVER_ALLOWED_HOSTS", "")
	h := GuardHTTPAuthority(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/services/oauth2/token" {
			tok, err := issueToken("case340-user")
			if err != nil {
				t.Error(err)
			}
			issued = tok
		}
		w.WriteHeader(http.StatusNoContent)
	}), "0.0.0.0:1789", "example.test:1789")
	if got, _ := case340Token(t, h, "example.test:1789", map[string]string{"X-Glade-Client": "1"}); got != http.StatusForbidden {
		t.Fatalf("client marker token status = %d, want 403", got)
	}
	if issued != "" {
		t.Fatal("token issued without capability")
	}
	if got, _ := case340Token(t, h, "example.test:1789", map[string]string{"X-Glade-Capability": "case340-secret"}); got != http.StatusNoContent {
		t.Fatalf("capability token status = %d, want 204", got)
	}
	if got := case340Do(h, "GET", "example.test:1789", case340AnonPath, map[string]string{"Authorization": "Bearer " + issued}); got != http.StatusNoContent {
		t.Fatalf("issued token on executeAnonymous = %d, want 204", got)
	}
	if got := case340Do(h, "GET", "example.test:1789", case340AnonPath, map[string]string{"Authorization": "Bearer local-case340-user"}); got != http.StatusForbidden {
		t.Fatalf("guessed token = %d, want 403", got)
	}
}

func TestCase340LoopbackTokenIssuanceKeepsClientMarker(t *testing.T) {
	h := case340BearerGuard(t, "", "127.0.0.1:1789")
	if got, _ := case340Token(t, h, "localhost:1789", map[string]string{"X-Glade-Client": "1"}); got != http.StatusNoContent {
		t.Fatalf("loopback token status = %d, want 204", got)
	}
}

func TestCase340TokensAreRandomAndBounded(t *testing.T) {
	a, err := issueToken("u1")
	if err != nil {
		t.Fatal(err)
	}
	b, err := issueToken("u1")
	if err != nil {
		t.Fatal(err)
	}
	if a == b || len(a) < 43 || strings.HasPrefix(a, "local-") {
		t.Fatalf("tokens not random: len=%d equal=%v", len(a), a == b)
	}
	if u, ok := issuedTokenUser(a); !ok || u != "u1" {
		t.Fatalf("lookup = %q %v", u, ok)
	}
	recent := ""
	for i := 0; i < maxIssuedTokens+10; i++ {
		recent, _ = issueToken("u2")
	}
	if _, ok := issuedTokenUser(recent); !ok {
		t.Fatal("recent token evicted")
	}
	if _, ok := issuedTokenUser(a); ok {
		t.Fatal("oldest token should have been evicted")
	}
	issuedTokens.Lock()
	n := len(issuedTokens.byHash)
	issuedTokens.Unlock()
	if n > maxIssuedTokens {
		t.Fatalf("store size %d exceeds bound", n)
	}
}
