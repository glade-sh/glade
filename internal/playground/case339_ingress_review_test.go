package playground

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestCase339ReviewDigitalOceanIngressClientQuotas(t *testing.T) {
	config, err := os.ReadFile("../../.do/app.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(config), "GLADE_DO_APP_PLATFORM") {
		t.Error("deployment does not enable App Platform ingress identity")
	}
	opts := ServerOptions{Public: true, RatePerMinute: 1000}
	f := reflect.ValueOf(&opts).Elem().FieldByName("DigitalOceanIngress")
	if f.IsValid() && f.Kind() == reflect.Bool {
		f.SetBool(true)
	}
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	s := NewServer(ws, opts)
	defer s.Close()
	for i := 1; i <= 12; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/playground/", nil)
		req.RemoteAddr = "10.0.0.5:1234"
		req.Header.Set("Do-Connecting-Ip", fmt.Sprintf("192.0.2.%d", i))
		s.ServeHTTP(case339ResponseWriter{rec}, req)
		if rec.Code != 200 || len(rec.Result().Cookies()) != 1 {
			t.Fatalf("proxied visitor %d: %d", i, rec.Code)
		}
	}
}

func TestCase339ReviewHealthAndReadinessContract(t *testing.T) {
	for _, public := range []bool{false, true} {
		t.Run(fmt.Sprint(public), func(t *testing.T) {
			ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir()})
			if err != nil {
				t.Fatal(err)
			}
			s := NewServer(ws, ServerOptions{Public: public, Version: "test-version"})
			defer s.Close()
			for _, tc := range []struct{ path, status string }{{"/healthz", "ok"}, {"/readyz", "ready"}} {
				rec := httptest.NewRecorder()
				s.ServeHTTP(case339ResponseWriter{rec}, httptest.NewRequest(http.MethodGet, tc.path, nil))
				if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"status":"`+tc.status+`"`) || !strings.Contains(rec.Body.String(), `"version":"test-version"`) || len(rec.Result().Cookies()) != 0 {
					t.Fatalf("%s: %d %s", tc.path, rec.Code, rec.Body.String())
				}
			}
			s.runner.initErr = errors.New("secret database detail")
			rec := httptest.NewRecorder()
			s.ServeHTTP(case339ResponseWriter{rec}, httptest.NewRequest(http.MethodGet, "/readyz", nil))
			if rec.Code != 503 || !strings.Contains(rec.Body.String(), `"status":"unavailable"`) || strings.Contains(rec.Body.String(), "secret database detail") {
				t.Fatalf("not-ready: %d %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestCase339ReviewCrossSiteNavigationPreservesSession(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	s := NewServer(ws, ServerOptions{Public: true})
	defer s.Close()
	boot := httptest.NewRecorder()
	s.ServeHTTP(case339ResponseWriter{boot}, httptest.NewRequest(http.MethodGet, "/playground/", nil))
	cookies := boot.Result().Cookies()
	if len(cookies) != 1 || cookies[0].SameSite != http.SameSiteLaxMode || !cookies[0].Secure || !cookies[0].HttpOnly {
		t.Fatalf("cookie does not permit secure top-level return: %#v", cookies)
	}
	cookie := cookies[0]
	save, _ := json.Marshal(FileSaveRequest{Path: "visitor.json", Content: "preserved"})
	req := httptest.NewRequest(http.MethodPut, "/playground/api/files", bytes.NewReader(save))
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	s.ServeHTTP(case339ResponseWriter{rec}, req)
	if rec.Code != 200 {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	req = httptest.NewRequest(http.MethodGet, "/playground/", nil)
	req.AddCookie(cookie)
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	rec = httptest.NewRecorder()
	s.ServeHTTP(case339ResponseWriter{rec}, req)
	if rec.Code != 200 || len(rec.Result().Cookies()) != 0 {
		t.Fatalf("navigation replaced session: %d", rec.Code)
	}
	req = httptest.NewRequest(http.MethodGet, "/visitor.json", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	s.ServeHTTP(case339ResponseWriter{rec}, req)
	if rec.Code != 200 || rec.Body.String() != "preserved" {
		t.Fatalf("return lost file: %d %s", rec.Code, rec.Body.String())
	}
	req = httptest.NewRequest(http.MethodPost, "/playground/api/reset", nil)
	req.AddCookie(cookie)
	req.Header.Set("Origin", "https://attacker.example")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	rec = httptest.NewRecorder()
	s.ServeHTTP(case339ResponseWriter{rec}, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("cross-site mutation: %d", rec.Code)
	}
}

func TestCase339ReviewIngressRejectsSpoofedOrAmbiguousHeaders(t *testing.T) {
	for _, tc := range []struct {
		enabled bool
		values  []string
		want    string
	}{
		{false, []string{"192.0.2.1"}, "203.0.113.10"},
		{true, []string{"192.0.2.1", "192.0.2.2"}, "203.0.113.10"},
		{true, []string{"192.0.2.1, 192.0.2.2"}, "203.0.113.10"},
		{true, []string{"bad"}, "203.0.113.10"},
		{true, []string{"192.0.2.1"}, "192.0.2.1"},
	} {
		req := httptest.NewRequest(http.MethodGet, "/playground/", nil)
		req.RemoteAddr = "203.0.113.10:1"
		for _, value := range tc.values {
			req.Header.Add("Do-Connecting-Ip", value)
		}
		if got := publicIngressClientIP(req, "", tc.enabled); got != tc.want {
			t.Errorf("enabled=%v values=%q: %q want %q", tc.enabled, tc.values, got, tc.want)
		}
	}
}

func TestCase339ReviewProbeEmptyVersionIsOmitted(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	s := NewServer(ws, ServerOptions{Public: true})
	defer s.Close()
	for _, path := range []string{"/healthz", "/readyz"} {
		rec := httptest.NewRecorder()
		s.ServeHTTP(case339ResponseWriter{rec}, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != 200 || strings.Contains(rec.Body.String(), `"version"`) {
			t.Errorf("%s: %d %s", path, rec.Code, rec.Body.String())
		}
	}
}

func TestCase339ReviewMutationOriginPolicy(t *testing.T) {
	for _, tc := range []struct {
		name    string
		origins []string
		site    string
		want    bool
	}{
		{"same-origin", []string{"https://example.com"}, "same-origin", true},
		{"no-origin-cli", nil, "", true},
		{"cross-site", []string{"https://example.com"}, "cross-site", false},
		{"other-origin", []string{"https://other.example"}, "same-site", false},
		{"multiple-origin", []string{"https://example.com", "https://other.example"}, "", false},
		{"null-origin", []string{"null"}, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "https://example.com/playground/api/reset", nil)
			for _, v := range tc.origins {
				req.Header.Add("Origin", v)
			}
			req.Header.Set("Sec-Fetch-Site", tc.site)
			if got := publicSameOrigin(req); got != tc.want {
				t.Fatalf("origin policy=%v want %v", got, tc.want)
			}
		})
	}
}
