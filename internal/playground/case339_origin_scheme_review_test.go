package playground

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCase339ReviewOriginSchemeAndIngress(t *testing.T) {
	for _, tc := range []struct {
		name, requestURL, origin, remote, trustedProxy string
		forwarded                                      []string
		digitalOcean, want                             bool
	}{
		{name: "https same origin", requestURL: "https://example.com/playground/api/reset", origin: "https://example.com", want: true},
		{name: "https downgrade", requestURL: "https://example.com/playground/api/reset", origin: "http://example.com", want: false},
		{name: "http same origin", requestURL: "http://example.com/playground/api/reset", origin: "http://example.com", want: true},
		{name: "http upgrade", requestURL: "http://example.com/playground/api/reset", origin: "https://example.com", want: false},
		{name: "http default port", requestURL: "http://example.com/playground/api/reset", origin: "http://example.com:80", want: true},
		{name: "https default port", requestURL: "https://example.com/playground/api/reset", origin: "https://example.com:443", want: true},
		{name: "different port", requestURL: "https://example.com/playground/api/reset", origin: "https://example.com:444", want: false},
		{name: "untrusted forwarding", requestURL: "http://example.com/playground/api/reset", origin: "https://example.com", forwarded: []string{"https"}, want: false},
		{name: "trusted proxy https", requestURL: "http://example.com/playground/api/reset", origin: "https://example.com", remote: "192.0.2.4:123", trustedProxy: "192.0.2.4", forwarded: []string{"https"}, want: true},
		{name: "trusted proxy downgrade", requestURL: "http://example.com/playground/api/reset", origin: "http://example.com", remote: "192.0.2.4:123", trustedProxy: "192.0.2.4", forwarded: []string{"https"}, want: false},
		{name: "trusted proxy duplicate", requestURL: "http://example.com/playground/api/reset", origin: "https://example.com", remote: "192.0.2.4:123", trustedProxy: "192.0.2.4", forwarded: []string{"https", "http"}, want: false},
		{name: "trusted proxy missing", requestURL: "http://example.com/playground/api/reset", origin: "http://example.com", remote: "192.0.2.4:123", trustedProxy: "192.0.2.4", want: false},
		{name: "platform external https", requestURL: "http://example.com/playground/api/reset", origin: "https://example.com", digitalOcean: true, want: true},
		{name: "platform downgrade", requestURL: "http://example.com/playground/api/reset", origin: "http://example.com", digitalOcean: true, want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir()})
			if err != nil {
				t.Fatal(err)
			}
			s := NewServer(ws, ServerOptions{Public: true, TrustedProxy: tc.trustedProxy, DigitalOceanIngress: tc.digitalOcean})
			defer s.Close()
			boot := httptest.NewRecorder()
			s.ServeHTTP(case339ResponseWriter{boot}, httptest.NewRequest(http.MethodGet, "/playground/", nil))
			cookies := boot.Result().Cookies()
			if len(cookies) != 1 {
				t.Fatal("session bootstrap failed")
			}
			req := httptest.NewRequest(http.MethodPost, tc.requestURL, nil)
			req.AddCookie(cookies[0])
			req.Header.Set("Origin", tc.origin)
			if tc.remote != "" {
				req.RemoteAddr = tc.remote
			}
			for _, v := range tc.forwarded {
				req.Header.Add("X-Forwarded-Proto", v)
			}
			rec := httptest.NewRecorder()
			s.ServeHTTP(case339ResponseWriter{rec}, req)
			if got := rec.Code == http.StatusOK; got != tc.want {
				t.Fatalf("status=%d body=%s, want accepted=%v", rec.Code, rec.Body.String(), tc.want)
			}
			if !tc.want && rec.Code != http.StatusForbidden {
				t.Fatalf("wanted forbidden, got %d", rec.Code)
			}
		})
	}
}

func TestCase339ReviewOriginSchemeProtectsAllMutations(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	s := NewServer(ws, ServerOptions{Public: true})
	defer s.Close()
	boot := httptest.NewRecorder()
	s.ServeHTTP(case339ResponseWriter{boot}, httptest.NewRequest(http.MethodGet, "/playground/", nil))
	cookie := boot.Result().Cookies()[0]
	for _, tc := range []struct{ method, path string }{{http.MethodPut, "/playground/api/files"}, {http.MethodDelete, "/playground/api/files"}, {http.MethodPost, "/playground/api/run"}, {http.MethodPost, "/playground/api/reset"}, {http.MethodPost, "/playground/api/examples/load"}} {
		req := httptest.NewRequest(tc.method, "https://example.com"+tc.path, nil)
		req.AddCookie(cookie)
		req.Header.Set("Origin", "http://example.com")
		rec := httptest.NewRecorder()
		s.ServeHTTP(case339ResponseWriter{rec}, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s %s: got %d", tc.method, tc.path, rec.Code)
		}
	}
}
