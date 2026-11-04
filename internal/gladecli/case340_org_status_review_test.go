package gladecli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/glade-sh/glade/internal/server"
)

type case340TransportFunc func(*http.Request) (*http.Response, error)

func (f case340TransportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestCase340OrgStatusThroughGuard(t *testing.T) {
	previous := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = previous })
	handler := server.GuardHTTPAuthority(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }), "127.0.0.1:1789")
	http.DefaultTransport = case340TransportFunc(func(r *http.Request) (*http.Response, error) {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, r)
		return rec.Result(), nil
	})
	status := checkOrgStatus(context.Background(), orgConfig{InstanceURL: "http://127.0.0.1:1789"})
	if status.Status != "running" {
		t.Fatalf("guarded org status=%q error=%q", status.Status, status.Error)
	}
}
