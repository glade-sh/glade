package gladecli

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPlaygroundListenerRejectsHostileAuthority(t *testing.T) {
	reached := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	})
	handler := playgroundListenerHandler(next, "127.0.0.1:1789", false)

	cases := []struct {
		name    string
		host    string
		headers map[string]string
		want    int
	}{
		{"rebound host", "attacker.example:1789", map[string]string{"X-Glade-Client": "1"}, http.StatusForbidden},
		{"wrong port", "127.0.0.1:9999", map[string]string{"X-Glade-Client": "1"}, http.StatusForbidden},
		{"hostile origin", "127.0.0.1:1789", map[string]string{"Origin": "http://attacker.example"}, http.StatusForbidden},
		{"cross-site fetch", "127.0.0.1:1789", map[string]string{"Sec-Fetch-Site": "cross-site", "X-Glade-Client": "1"}, http.StatusForbidden},
		{"no origin metadata", "127.0.0.1:1789", nil, http.StatusForbidden},
		{"explicit client", "127.0.0.1:1789", map[string]string{"X-Glade-Client": "1"}, http.StatusOK},
		{"same-origin browser", "localhost:1789", map[string]string{"Origin": "http://localhost:1789", "Sec-Fetch-Site": "same-origin"}, http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reached = false
			req := httptest.NewRequest(http.MethodPost, "http://"+tc.host+"/playground/api/run", nil)
			req.Host = tc.host
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
			if reached != (tc.want == http.StatusOK) {
				t.Fatalf("handler reached = %v for status %d", reached, rec.Code)
			}
		})
	}
}

func TestPlaygroundPublicListenerKeepsOwnIngressChecks(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	handler := playgroundListenerHandler(next, "0.0.0.0:8080", true)
	req := httptest.NewRequest(http.MethodGet, "http://playground.example/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("public listener status = %d, want 200 (public mode keeps its own ingress checks)", rec.Code)
	}
}
