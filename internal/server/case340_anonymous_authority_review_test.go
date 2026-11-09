package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestCase340AnonymousRoutesRequireCapabilityOrIssuedBearer(t *testing.T) {
	for _, bind := range []struct{ name, addr, host string }{{"exposed", "0.0.0.0:1789", "example.test:1789"}, {"loopback", "127.0.0.1:1789", "localhost:1789"}} {
		t.Run(bind.name, func(t *testing.T) {
			t.Setenv("GLADE_SERVER_CAPABILITY", "secret")
			t.Setenv("GLADE_SERVER_ALLOWED_HOSTS", "")
			token, err := issueToken("005000000000123")
			if err != nil {
				t.Fatal(err)
			}
			h := GuardHTTPAuthority(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }), bind.addr, bind.host)
			for _, api := range []string{"62.0", "67.0"} {
				for _, route := range []struct{ name, method, path string }{
					{"get", "GET", "/services/data/v" + api + "/tooling/executeAnonymous?anonymousBody=System.debug(1)"},
					{"post", "POST", "/services/data/v" + api + "/tooling/executeAnonymous"},
					{"soap", "POST", "/services/Soap/s/" + api + "/example"},
				} {
					for _, auth := range []struct {
						name, header, value string
						want                int
					}{
						{"client", "X-Glade-Client", "1", http.StatusForbidden},
						{"fake", "Authorization", "Bearer fake", http.StatusForbidden},
						{"capability", "X-Glade-Capability", "secret", http.StatusNoContent},
						{"issued", "Authorization", "Bearer " + token, http.StatusNoContent},
					} {
						t.Run(api+"/"+route.name+"/"+auth.name, func(t *testing.T) {
							req := httptest.NewRequest(route.method, "http://"+bind.host+route.path, strings.NewReader(`{"anonymousBody":"System.debug(1)"}`))
							req.Header.Set(auth.header, auth.value)
							rec := httptest.NewRecorder()
							h.ServeHTTP(rec, req)
							if rec.Code != auth.want {
								t.Errorf("status=%d want=%d", rec.Code, auth.want)
							}
						})
					}
				}
			}
			req := httptest.NewRequest("POST", "http://"+bind.host+"/services/Soap/u/67.0/example", nil)
			req.Header.Set("X-Glade-Client", "1")
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			want := http.StatusNoContent
			if bind.name == "exposed" {
				want = http.StatusForbidden
			}
			if rec.Code != want {
				t.Errorf("partner SOAP status=%d want=%d", rec.Code, want)
			}
			req = httptest.NewRequest("POST", "http://"+bind.host+"/services/Soap/u/67.0/example", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			rec = httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusNoContent {
				t.Errorf("authorized partner SOAP status=%d", rec.Code)
			}
		})
	}
}

func TestCase340CompositeCannotOverrideIssuedUser(t *testing.T) {
	for _, route := range []string{"composite", "composite/graph", "composite/batch"} {
		t.Run(route, func(t *testing.T) {
			t.Setenv("GLADE_SERVER_CAPABILITY", "")
			t.Setenv("GLADE_SERVER_ALLOWED_HOSTS", "")
			org := testOrg()
			storage.EnsureDeterministicPlatformData(&org)
			addUser(&org, "005000000000123", "one@example.test", "one.alias@example.test", "First User")
			addUser(&org, "005000000000124", "two@example.test", "two.alias@example.test", "Second User")
			s := New(&org)
			token := case340OwnedTokenFor(t, s, "005000000000123")
			sub := `{"method":"POST","url":"/services/data/v67.0/sobjects/Account","referenceId":"create","httpHeaders":{"X-GLADE-User-Id":"005000000000124"},"body":{"Name":"Synthetic"}}`
			payload := `{"compositeRequest":[` + sub + `]}`
			if route == "composite/graph" {
				payload = `{"graphs":[{"graphId":"one","compositeRequest":[` + sub + `]}]}`
			}
			if route == "composite/batch" {
				payload = `{"batchRequests":[` + sub + `]}`
			}
			req := httptest.NewRequest("POST", "http://localhost:1789/services/data/v67.0/"+route, strings.NewReader(payload))
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			GuardHTTPAuthority(s, "127.0.0.1:1789").ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			var payloadJSON any
			if err := json.Unmarshal(rec.Body.Bytes(), &payloadJSON); err != nil {
				t.Fatal(err)
			}
			accounts := org.Objects["Account"].Records
			if len(accounts) != 1 {
				t.Fatalf("accounts=%d body=%s", len(accounts), rec.Body.String())
			}
			for _, account := range accounts {
				if got := account.System.CreatedByID; string(got) != "005000000000123" {
					t.Errorf("created by %s want user A; response %s", got, fmt.Sprint(payloadJSON))
				}
			}
		})
	}
}

func TestCase340SOAPKeepsIssuedUser(t *testing.T) {
	t.Setenv("GLADE_SERVER_CAPABILITY", "")
	org := testOrg()
	storage.EnsureDeterministicPlatformData(&org)
	addUser(&org, "005000000000123", "one@example.test", "one.alias@example.test", "First User")
	addUser(&org, "005000000000124", "two@example.test", "two.alias@example.test", "Second User")
	s := New(&org)
	token := case340OwnedTokenFor(t, s, "005000000000123")
	body := soapEnvelope(`<apex:executeAnonymous><apex:String>System.assert(String.valueOf(UserInfo.getUserId()).startsWith('005000000000123'));</apex:String></apex:executeAnonymous>`)
	req := httptest.NewRequest("POST", "http://localhost:1789/services/Soap/s/67.0/example", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-GLADE-User-Id", "005000000000124")
	rec := httptest.NewRecorder()
	GuardHTTPAuthority(s, "127.0.0.1:1789").ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "<success>true</success>") {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCase340AnonymousExecutionRealServerAuthority(t *testing.T) {
	t.Setenv("GLADE_SERVER_CAPABILITY", "secret")
	t.Setenv("GLADE_SERVER_ALLOWED_HOSTS", "")
	org := testOrg()
	storage.EnsureDeterministicPlatformData(&org)
	s := New(&org)
	token := case340OwnedToken(t, s)
	h := GuardHTTPAuthority(s, "0.0.0.0:1789", "example.test:1789")
	for _, route := range []struct{ name, path, body, contentType string }{
		{"rest", "/services/data/v67.0/tooling/executeAnonymous", `{"anonymousBody":"System.debug(1);"}`, "application/json"},
		{"soap", "/services/Soap/s/67.0/example", soapEnvelope(`<apex:executeAnonymous><apex:String>System.debug(1);</apex:String></apex:executeAnonymous>`), "text/xml"},
	} {
		for _, auth := range []struct {
			name, header, value string
			want                int
		}{
			{"marker", "X-Glade-Client", "1", http.StatusForbidden},
			{"capability", "X-Glade-Capability", "secret", http.StatusOK},
			{"issued", "Authorization", "Bearer " + token, http.StatusOK},
		} {
			t.Run(route.name+"/"+auth.name, func(t *testing.T) {
				req := httptest.NewRequest("POST", "http://example.test:1789"+route.path, strings.NewReader(route.body))
				req.Header.Set("Content-Type", route.contentType)
				req.Header.Set(auth.header, auth.value)
				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, req)
				if rec.Code != auth.want {
					t.Errorf("status=%d want=%d body=%s", rec.Code, auth.want, rec.Body.String())
				}
			})
		}
	}
}

func TestCase340SameOriginIssuedBearerAuthorizesAnonymous(t *testing.T) {
	t.Setenv("GLADE_SERVER_CAPABILITY", "secret")
	t.Setenv("GLADE_SERVER_ALLOWED_HOSTS", "")
	token, err := issueToken("005000000000123")
	if err != nil {
		t.Fatal(err)
	}
	h := GuardHTTPAuthority(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }), "0.0.0.0:1789", "example.test:1789")
	for _, route := range []struct{ name, method, path string }{
		{"get", "GET", "/services/data/v67.0/tooling/executeAnonymous"},
		{"post", "POST", "/services/data/v67.0/tooling/executeAnonymous"},
		{"soap", "POST", "/services/Soap/s/67.0/example"},
	} {
		for _, tc := range []struct {
			name, origin, site, bearer string
			want                       int
		}{
			{"same-origin", "http://example.test:1789", "same-origin", token, http.StatusNoContent},
			{"same-origin-origin-only", "http://example.test:1789", "", token, http.StatusNoContent},
			{"cross-origin", "http://evil.invalid", "cross-site", token, http.StatusForbidden},
			{"forged-same-origin", "http://example.test:1789", "same-origin", "fake", http.StatusForbidden},
		} {
			t.Run(route.name+"/"+tc.name, func(t *testing.T) {
				req := httptest.NewRequest(route.method, "http://example.test:1789"+route.path, nil)
				req.Header.Set("Authorization", "Bearer "+tc.bearer)
				req.Header.Set("Origin", tc.origin)
				if tc.site != "" {
					req.Header.Set("Sec-Fetch-Site", tc.site)
				}
				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, req)
				if rec.Code != tc.want {
					t.Errorf("status=%d want=%d", rec.Code, tc.want)
				}
			})
		}
	}
}
