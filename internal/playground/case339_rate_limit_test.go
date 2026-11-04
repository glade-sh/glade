package playground

import (
	"fmt"
	"net/http/httptest"
	"reflect"
	"testing"
)

// A direct peer controls forwarding headers; they must not select its quota.
func TestCase339ClientIPIgnoresUntrustedForwarding(t *testing.T) {
	for _, forwarded := range []string{"192.0.2.1", "198.51.100.7, 203.0.113.4", "not-an-ip"} {
		req := httptest.NewRequest("POST", "/playground/api/run", nil)
		req.RemoteAddr = "203.0.113.10:1234"
		req.Header.Set("X-Forwarded-For", forwarded)
		if got := clientIP(req); got != "203.0.113.10" {
			t.Errorf("forwarded %q: clientIP = %q, want peer", forwarded, got)
		}
	}
}

func TestCase339RotatingForwardedHeadersDoesNotEvadeRateLimit(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	s := NewServer(ws, ServerOptions{Public: true, RatePerMinute: 1})
	t.Cleanup(s.Close)
	for i, forwarded := range []string{"192.0.2.1", "198.51.100.2"} {
		req := httptest.NewRequest("POST", "/playground/api/reset", nil)
		req.RemoteAddr = "203.0.113.10:1234"
		req.Header.Set("X-Forwarded-For", forwarded)
		rec := httptest.NewRecorder()
		s.ServeHTTP(case339ResponseWriter{rec}, req)
		want := 200
		if i == 1 {
			want = 429
		}
		if rec.Code != want {
			t.Errorf("request %d: status %d, want %d", i, rec.Code, want)
		}
	}
}

func TestCase339PublicSessionCreationIsRateLimited(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	s := NewServer(ws, ServerOptions{Public: true, RatePerMinute: 1})
	defer s.Close()
	for i, want := range []int{200, 429} {
		req := httptest.NewRequest("GET", "/playground/api/workspace", nil)
		req.RemoteAddr = "203.0.113.10:1234"
		rec := httptest.NewRecorder()
		s.ServeHTTP(case339ResponseWriter{rec}, req)
		if rec.Code != want {
			t.Errorf("new session %d: status %d, want %d", i, rec.Code, want)
		}
	}
}

func TestCase339ReviewTrustedProxySessionIsolation(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	opts := ServerOptions{Public: true, RatePerMinute: 1000}
	field := reflect.ValueOf(&opts).Elem().FieldByName("TrustedProxy")
	if field.IsValid() && field.Kind() == reflect.String {
		field.SetString("203.0.113.10")
	}
	s := NewServer(ws, opts)
	defer s.Close()
	create := func(peer, forwarded string) int {
		req := httptest.NewRequest("GET", "/playground/api/workspace", nil)
		req.RemoteAddr = peer + ":1"
		req.Header.Set("X-Forwarded-For", forwarded)
		rec := httptest.NewRecorder()
		s.ServeHTTP(case339ResponseWriter{rec}, req)
		return rec.Code
	}
	for i := 0; i < 8; i++ {
		if code := create("203.0.113.10", "192.0.2.1"); code != 200 {
			t.Fatalf("visitor A %d: %d", i, code)
		}
	}
	if code := create("203.0.113.10", "192.0.2.1"); code != 429 {
		t.Fatalf("visitor A exceeded quota: %d", code)
	}
	if code := create("203.0.113.10", "192.0.2.2"); code != 200 {
		t.Fatalf("visitor B behind proxy rejected: %d", code)
	}
	for i := 0; i < 8; i++ {
		if code := create("203.0.113.11", fmt.Sprintf("192.0.2.%d", i+10)); code != 200 {
			t.Fatalf("untrusted peer %d: %d", i, code)
		}
	}
	if code := create("203.0.113.11", "198.51.100.20"); code != 429 {
		t.Fatalf("untrusted forwarding bypassed quota: %d", code)
	}
}

func TestCase339ReviewProxyRejectsAmbiguousForwarding(t *testing.T) {
	for _, values := range [][]string{{"192.0.2.1, 198.51.100.1"}, {"192.0.2.1", "198.51.100.1"}, {"malformed"}, {}} {
		req := httptest.NewRequest("GET", "/playground/", nil)
		req.RemoteAddr = "203.0.113.10:1"
		for _, value := range values {
			req.Header.Add("X-Forwarded-For", value)
		}
		if got := publicClientIP(req, "203.0.113.10"); got != "203.0.113.10" {
			t.Errorf("values %q selected %q", values, got)
		}
	}
}
