package server

import (
	"net/http"
	"testing"
)

func TestV035TokenTTL(t *testing.T) {
	t.Setenv("GLADE_SERVER_TOKEN_TTL", "1ns")
	token, err := issueToken("user")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := issuedTokenUser(token); ok {
		t.Fatal("expired token accepted")
	}
	t.Setenv("GLADE_SERVER_TOKEN_TTL", "invalid")
	if _, err := issueToken("user"); err == nil {
		t.Fatal("invalid TTL accepted")
	}
}

func TestV035ExplicitPublicDemo(t *testing.T) {
	t.Setenv("GLADE_SERVER_CAPABILITY", "secret")
	h := GuardHTTPAuthorityWithOptions(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }), "0.0.0.0:1789", AuthorityOptions{AllowPublicDemo: true}, "example.test:1789")
	if got := case340Do(h, "GET", "example.test:1789", "/services/data/v67.0/query", map[string]string{"X-Glade-Client": "1"}); got != http.StatusNoContent {
		t.Fatalf("demo got %d", got)
	}
}
