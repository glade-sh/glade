package playground

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCase339ReviewPublicRunDoesNotPersistResultCache(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	s := NewServer(ws, ServerOptions{Public: true, RunTimeout: 30 * time.Second})
	defer s.Close()
	body, _ := json.Marshal(RunRequest{AnonymousBody: "Integer value = 42;", UseCache: true})
	rec := httptest.NewRecorder()
	s.ServeHTTP(case339ResponseWriter{rec}, httptest.NewRequest(http.MethodPost, "/playground/api/run", bytes.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("run status %d: %s", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("missing session")
	}
	session := s.sessions.items[cookies[0].Value]
	_, found, err := session.server.runner.cache.Latest()
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Fatal("public run persisted latest result")
	}
}

func TestCase339ReviewPublicRoutesSetWriteDeadlines(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	s := NewServer(ws, ServerOptions{Public: true})
	defer s.Close()
	for _, path := range []string{"/playground/", "/playground/api/workspace", "/playground/api/database", "/playground/api/runs/latest"} {
		w := &case339DeadlineWriter{ResponseRecorder: httptest.NewRecorder()}
		s.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if !w.sawDeadline {
			t.Errorf("%s has no write deadline", path)
		}
	}
}

func TestCase339ReviewFullSessionTablePreservesInactive(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	s := NewServer(ws, ServerOptions{Public: true, RatePerMinute: 1000})
	defer s.Close()
	var oldest *http.Cookie
	for i := 0; i < publicSessionLimit; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/playground/api/workspace", nil)
		req.RemoteAddr = fmt.Sprintf("192.0.2.%d:1", i+1)
		s.ServeHTTP(case339ResponseWriter{rec}, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("session %d: %d", i, rec.Code)
		}
		cookie := rec.Result().Cookies()[0]
		if i == 0 {
			oldest = cookie
			s.sessions.items[cookie.Value].lastUsed = time.Now().Add(-time.Minute)
		}
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/playground/api/workspace", nil)
	req.RemoteAddr = "192.0.2.2:1"
	s.ServeHTTP(case339ResponseWriter{rec}, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("new visitor status: %d %s", rec.Code, rec.Body.String())
	}
	if _, ok := s.sessions.items[oldest.Value]; !ok {
		t.Fatal("unexpired inactive session was deleted")
	}
	for _, session := range s.sessions.items {
		session.active = 1
	}
	defer func() {
		for _, session := range s.sessions.items {
			session.active = 0
		}
	}()
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/playground/api/workspace", nil)
	req.RemoteAddr = "192.0.2.3:1"
	s.ServeHTTP(case339ResponseWriter{rec}, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("active sessions evicted: %d", rec.Code)
	}
}

func TestCase339ReviewPublicLatestCannotExposeOversizedCachedResult(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	s := NewServer(ws, ServerOptions{Public: true})
	defer s.Close()
	first := httptest.NewRecorder()
	s.ServeHTTP(case339ResponseWriter{first}, httptest.NewRequest(http.MethodGet, "/playground/api/workspace", nil))
	cookie := first.Result().Cookies()[0]
	child := s.sessions.items[cookie.Value].server
	if err := child.runner.cache.Store("synthetic", RunResult{Logs: []string{strings.Repeat("x", maxPublicResponseBytes+1)}}); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/playground/api/runs/latest", nil)
	req.AddCookie(cookie)
	s.ServeHTTP(case339ResponseWriter{rec}, req)
	if rec.Body.Len() > maxPublicResponseBytes || !strings.Contains(rec.Body.String(), `"found":false`) {
		t.Fatalf("public latest exposed cached output: status=%d bytes=%d", rec.Code, rec.Body.Len())
	}
}

func TestCase339ReviewPublicStatelessRequests(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	s := NewServer(ws, ServerOptions{Public: true})
	defer s.Close()
	for _, path := range []string{"/playground/assets/unknown.js", "/unknown-health-check", "/playground/api/health"} {
		rec := httptest.NewRecorder()
		s.ServeHTTP(case339ResponseWriter{rec}, httptest.NewRequest(http.MethodGet, path, nil))
		if len(rec.Result().Cookies()) != 0 || len(s.sessions.items) != 0 {
			t.Fatalf("%s created session", path)
		}
	}
}

func TestCase339ReviewSessionPeerQuotaAndRecovery(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	s := NewServer(ws, ServerOptions{Public: true, RatePerMinute: 1000})
	defer s.Close()
	create := func(ip string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/playground/api/workspace", nil)
		req.RemoteAddr = ip + ":1234"
		s.ServeHTTP(case339ResponseWriter{rec}, req)
		return rec
	}
	var first *http.Cookie
	for i := 0; i < 8; i++ {
		rec := create("192.0.2.10")
		if rec.Code != 200 {
			t.Fatalf("session %d: %d", i, rec.Code)
		}
		if i == 0 {
			first = rec.Result().Cookies()[0]
		}
	}
	for i := 0; i < 70; i++ {
		if rec := create("192.0.2.10"); rec.Code != 429 {
			t.Fatalf("over-quota session %d: %d", i, rec.Code)
		}
	}
	if rec := create("192.0.2.20"); rec.Code != 200 {
		t.Fatalf("other peer rejected: %d", rec.Code)
	}
	s.sessions.items[first.Value].lastUsed = time.Now().Add(-publicSessionIdle - time.Second)
	if rec := create("192.0.2.10"); rec.Code != 200 {
		t.Fatalf("expired slot not recovered: %d", rec.Code)
	}
}
