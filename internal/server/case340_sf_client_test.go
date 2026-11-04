package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCase340SalesforceCLIBearerRequests(t *testing.T) {
	t.Setenv("GLADE_SERVER_CAPABILITY", "")
	handler := GuardHTTPAuthority(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), "127.0.0.1:1789")
	for _, tc := range []struct {
		name, method, path, auth, origin, site, host string
		want                                         int
	}{
		{"login", "GET", "/services/oauth2/userinfo", "Bearer local-token-example", "", "", "", http.StatusNoContent},
		{"query", "GET", "/services/data/v62.0/query?q=SELECT+Id+FROM+Account", "Bearer local-token", "", "", "", http.StatusNoContent},
		{"create", "POST", "/services/data/v67.0/sobjects/Account", "Bearer local-token", "", "", "", http.StatusNoContent},
		{"soap", "POST", "/services/Soap/s/67.0/example", "Bearer local-token", "", "", "", http.StatusForbidden},
		{"same-origin-anonymous", "GET", "/services/data/v67.0/tooling/executeAnonymous", "Bearer local-token", "http://localhost:1789", "same-origin", "", http.StatusForbidden},
		{"anonymous", "GET", "/services/data/v67.0/tooling/executeAnonymous?anonymousBody=System.debug(1)", "Bearer local-token", "", "", "", http.StatusForbidden},
		{"missing", "GET", "/services/oauth2/userinfo", "", "", "", "", http.StatusForbidden},
		{"empty", "GET", "/services/oauth2/userinfo", "Bearer ", "", "", "", http.StatusForbidden},
		{"basic", "POST", "/services/data/v67.0/sobjects/Account", "Basic local-token", "", "", "", http.StatusForbidden},
		{"cross-origin", "POST", "/services/data/v67.0/sobjects/Account", "Bearer local-token", "http://evil.invalid", "cross-site", "", http.StatusForbidden},
		{"rebinding", "GET", "/services/oauth2/userinfo", "Bearer local-token", "", "", "evil.invalid:1789", http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "http://localhost:1789"+tc.path, nil)
			if tc.auth != "" {
				req.Header.Set("Authorization", tc.auth)
			}
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			if tc.site != "" {
				req.Header.Set("Sec-Fetch-Site", tc.site)
			}
			if tc.host != "" {
				req.Host = tc.host
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Errorf("status = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}

// The sf CLI flow on a loopback server: token login, then query and apex run
// with the issued token.
func TestCase340SalesforceCLILoginQueryApexRun(t *testing.T) {
	h := case340BearerGuard(t, "", "127.0.0.1:1789")
	if got, _ := case340Token(t, h, "localhost:1789", map[string]string{"X-Glade-Client": "1"}); got != http.StatusNoContent {
		t.Fatalf("login = %d", got)
	}
	token, err := issueToken("case340-user")
	if err != nil {
		t.Fatal(err)
	}
	auth := map[string]string{"Authorization": "Bearer " + token}
	if got := case340Do(h, "GET", "localhost:1789", "/services/data/v62.0/query?q=SELECT+Id+FROM+Account", auth); got != http.StatusNoContent {
		t.Errorf("query = %d", got)
	}
	if got := case340Do(h, "GET", "localhost:1789", case340AnonPath, auth); got != http.StatusNoContent {
		t.Errorf("apex run = %d", got)
	}
}
