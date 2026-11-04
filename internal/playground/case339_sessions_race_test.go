package playground

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestCase339ConcurrentPublicSessions(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	s := NewServer(ws, ServerOptions{Public: true})
	t.Cleanup(s.Close)
	const visitors = 4
	ids := make(chan string, visitors)
	var group sync.WaitGroup
	for i := 0; i < visitors; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			rec := httptest.NewRecorder()
			s.ServeHTTP(case339ResponseWriter{rec}, httptest.NewRequest(http.MethodGet, "/playground/api/workspace", nil))
			cookies := rec.Result().Cookies()
			if rec.Code != http.StatusOK || len(cookies) == 0 {
				ids <- ""
				return
			}
			ids <- cookies[0].Value
		}()
	}
	group.Wait()
	close(ids)
	seen := map[string]bool{}
	for id := range ids {
		if id == "" || seen[id] {
			t.Fatalf("missing or duplicate session identity %q", id)
		}
		seen[id] = true
	}
	t.Run("bootstrap_cookie_precedes_parallel_requests", func(t *testing.T) {
		boot := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/playground/", nil)
		req.RemoteAddr = "198.51.100.10:1"
		s.ServeHTTP(case339ResponseWriter{boot}, req)
		cookies := boot.Result().Cookies()
		if boot.Code != 200 || len(cookies) != 1 {
			t.Fatalf("bootstrap status=%d cookies=%d", boot.Code, len(cookies))
		}
		cookie := cookies[0]
		if !strings.Contains(boot.Header().Get("Cache-Control"), "no-store") {
			t.Fatalf("bootstrap response may be cached: %q", boot.Header().Get("Cache-Control"))
		}
		payload, _ := json.Marshal(FileSaveRequest{Path: "synthetic.json", Content: "saved value"})
		results := make(chan int, 2)
		for _, tc := range []struct {
			method, path string
			body         []byte
		}{{http.MethodGet, "/playground/api/workspace", nil}, {http.MethodPut, "/playground/api/files", payload}} {
			go func(tc struct {
				method, path string
				body         []byte
			}) {
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(tc.method, tc.path, bytes.NewReader(tc.body))
				req.RemoteAddr = "198.51.100.10:1"
				req.AddCookie(cookie)
				s.ServeHTTP(case339ResponseWriter{rec}, req)
				results <- rec.Code
			}(tc)
		}
		for i := 0; i < 2; i++ {
			if code := <-results; code != 200 {
				t.Fatalf("parallel request status %d", code)
			}
		}
		rec := httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, "/synthetic.json", nil)
		req.AddCookie(cookie)
		s.ServeHTTP(case339ResponseWriter{rec}, req)
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), "saved value") {
			t.Fatalf("saved file lost: %d %s", rec.Code, rec.Body.String())
		}
	})

}
