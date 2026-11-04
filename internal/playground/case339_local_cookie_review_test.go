package playground

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

// case339SafariCookieJar models a browser that does not retain Secure cookies
// over loopback HTTP; the standard Go jar permits them on localhost.
type case339SafariCookieJar struct{ http.CookieJar }

func (j case339SafariCookieJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	if u.Scheme == "http" {
		filtered := make([]*http.Cookie, 0, len(cookies))
		for _, cookie := range cookies {
			if !cookie.Secure {
				filtered = append(filtered, cookie)
			}
		}
		cookies = filtered
	}
	j.CookieJar.SetCookies(u, cookies)
}

type case339CookieTransport struct{ handler http.Handler }

func (t case339CookieTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	rec := httptest.NewRecorder()
	req.RemoteAddr = "127.0.0.1:1234"
	t.handler.ServeHTTP(case339ResponseWriter{rec}, req)
	return rec.Result(), nil
}

func TestCase339ReviewLocalHTTPCookieRetention(t *testing.T) {
	opts := ServerOptions{Public: true, RatePerMinute: 1000}
	field := reflect.ValueOf(&opts).Elem().FieldByName("InsecureLocalCookie")
	if field.IsValid() && field.Kind() == reflect.Bool {
		field.SetBool(true)
	}
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	s := NewServer(ws, opts)
	defer s.Close()
	host := "http://localhost:8080"
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: case339CookieTransport{s}, Jar: case339SafariCookieJar{jar}}
	var firstCookie *http.Cookie
	for i := 0; i < 12; i++ {
		resp, err := client.Get(host + "/playground/")
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("request %d: %s", i, resp.Status)
		}
		if i == 0 && len(resp.Cookies()) == 1 {
			firstCookie = resp.Cookies()[0]
		}
	}
	if firstCookie == nil || firstCookie.Secure || !firstCookie.HttpOnly || firstCookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("local cookie: %#v", firstCookie)
	}
	if len(s.sessions.items) != 1 {
		t.Fatalf("expected one retained session, got %d", len(s.sessions.items))
	}
	req, _ := http.NewRequest(http.MethodPut, host+"/playground/api/files", strings.NewReader(`{"path":"local.apex","content":"System.debug(1);"}`))
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	sb, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("save: %s %s", resp.Status, sb)
	}
	resp, err = client.Get(host + "/playground/api/workspace")
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil || !strings.Contains(string(data), `"path":"local.apex"`) {
		t.Fatalf("saved file missing from the retained session workspace: %q %v", data, err)
	}
}

func TestCase339ReviewDefaultHTTPSCookieRemainsSecure(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	s := NewServer(ws, ServerOptions{Public: true})
	defer s.Close()
	host := "https://playground.example"
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: case339CookieTransport{s}, Jar: jar}
	for i := 0; i < 10; i++ {
		resp, err := client.Get(host + "/playground/")
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("request %d: %s", i, resp.Status)
		}
		if i == 0 {
			cookies := resp.Cookies()
			if len(cookies) != 1 || !cookies[0].Secure || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteLaxMode {
				t.Fatalf("HTTPS cookie: %#v", cookies)
			}
		}
	}
	if len(s.sessions.items) != 1 {
		t.Fatalf("expected one HTTPS session, got %d", len(s.sessions.items))
	}
}

func TestCase339ReviewLocalCookieOptInDoesNotOverrideProxySecurity(t *testing.T) {
	for _, tc := range []struct {
		name         string
		proxy        string
		digitalOcean bool
	}{
		{"trusted proxy", "192.0.2.1", false},
		{"app platform", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := ServerOptions{Public: true, TrustedProxy: tc.proxy, DigitalOceanIngress: tc.digitalOcean}
			field := reflect.ValueOf(&opts).Elem().FieldByName("InsecureLocalCookie")
			if !field.IsValid() || field.Kind() != reflect.Bool {
				t.Fatal("explicit local HTTP cookie option missing")
			}
			field.SetBool(true)
			ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir()})
			if err != nil {
				t.Fatal(err)
			}
			s := NewServer(ws, opts)
			defer s.Close()
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/playground/", nil)
			s.ServeHTTP(case339ResponseWriter{rec}, req)
			cookies := rec.Result().Cookies()
			if len(cookies) != 1 || !cookies[0].Secure {
				t.Fatalf("proxy must retain Secure: %#v", cookies)
			}
		})
	}
}

func TestCase339ReviewLocalCookieHostBoundary(t *testing.T) {
	for _, tc := range []struct {
		host   string
		secure bool
	}{
		{"localhost:8080", false}, {"127.0.0.1:8080", false}, {"[::1]:8080", false},
		{"localhost.example:8080", true}, {"example.com:8080", true},
	} {
		t.Run(tc.host, func(t *testing.T) {
			opts := ServerOptions{Public: true}
			field := reflect.ValueOf(&opts).Elem().FieldByName("InsecureLocalCookie")
			if !field.IsValid() || field.Kind() != reflect.Bool {
				t.Fatal("explicit local HTTP cookie option missing")
			}
			field.SetBool(true)
			ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir()})
			if err != nil {
				t.Fatal(err)
			}
			s := NewServer(ws, opts)
			defer s.Close()
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/playground/", nil)
			req.Host = tc.host
			s.ServeHTTP(case339ResponseWriter{rec}, req)
			cookies := rec.Result().Cookies()
			if len(cookies) != 1 || cookies[0].Secure != tc.secure {
				t.Fatalf("host %s cookie: %#v", tc.host, cookies)
			}
		})
	}
}
