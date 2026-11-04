package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCase340NavigationPromptsBeforeDynamicAction(t *testing.T) {
	t.Setenv("GLADE_SERVER_CAPABILITY", "")
	for _, path := range []string{"/services/apexrest/Synthetic?mutate=1", "/apex/SyntheticPage", "/db", "/lightning/preview"} {
		t.Run(path, func(t *testing.T) {
			calls := 0
			handler := GuardHTTPAuthority(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { calls++; w.WriteHeader(http.StatusNoContent) }), "127.0.0.1:1789")
			req := httptest.NewRequest(http.MethodGet, "http://localhost:1789"+path, nil)
			req.Header.Set("Sec-Fetch-Site", "none")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK || calls != 0 || !strings.Contains(rec.Body.String(), `href=""`) {
				t.Fatalf("initial navigation: status=%d calls=%d body=%q", rec.Code, calls, rec.Body.String())
			}
			req = httptest.NewRequest(http.MethodGet, "http://localhost:1789"+path, nil)
			req.Header.Set("Sec-Fetch-Site", "same-origin")
			rec = httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusNoContent || calls != 1 {
				t.Fatalf("confirmed navigation: status=%d calls=%d", rec.Code, calls)
			}
		})
	}
}
