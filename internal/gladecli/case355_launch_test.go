package gladecli

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/server"
)

func TestCase355PlaygroundLaunchConfirmation(t *testing.T) {
	t.Setenv("GLADE_SERVER_CAPABILITY", "")
	t.Setenv("GLADE_SERVER_ALLOWED_HOSTS", "")
	t.Setenv("GLADE_SERVER_PROXY_ORIGINS", "")
	for _, example := range []string{"", "refinement-service"} {
		t.Run(example, func(t *testing.T) {
			called := false
			handler := server.GuardHTTPAuthority(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusNoContent)
			}), "127.0.0.1:1789")
			req := httptest.NewRequest(http.MethodGet, playgroundURL("127.0.0.1:1789", example), nil)
			req.Header.Set("Sec-Fetch-Site", "none")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if called || rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Open local preview") {
				t.Fatalf("launch confirmation: called=%v status=%d body=%q", called, rec.Code, rec.Body.String())
			}
			req.Header.Set("Sec-Fetch-Site", "same-origin")
			rec = httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if !called || rec.Code != http.StatusNoContent {
				t.Fatalf("confirmed launch: called=%v status=%d", called, rec.Code)
			}
		})
	}
}
