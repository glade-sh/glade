package playground

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestCase339ReviewPublicResponsesAreNotCacheable(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	s := NewServer(ws, ServerOptions{Public: true, RatePerMinute: 1000})
	defer s.Close()
	request := func(method, path string, body []byte, cookie *http.Cookie, conditional bool) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, bytes.NewReader(body))
		req.RemoteAddr = "192.0.2.7:1"
		if cookie != nil {
			req.AddCookie(cookie)
		}
		if conditional {
			req.Header.Set("If-Modified-Since", time.Now().Add(time.Hour).UTC().Format(http.TimeFormat))
		}
		s.ServeHTTP(case339ResponseWriter{rec}, req)
		if !strings.Contains(rec.Header().Get("Cache-Control"), "no-store") {
			t.Errorf("%s %s: cache-control %q", method, path, rec.Header().Get("Cache-Control"))
		}
		return rec
	}
	first := request(http.MethodGet, "/playground/api/workspace", nil, nil, false)
	if first.Code != 200 || len(first.Result().Cookies()) != 1 {
		t.Fatalf("session bootstrap: %d", first.Code)
	}
	cookie := first.Result().Cookies()[0]
	for _, path := range []string{"/playground/api/workspace", "/playground/api/database", "/playground/api/runs/latest", "/playground/api/examples", "/playground/assets/unknown.js"} {
		request(http.MethodGet, path, nil, cookie, false)
	}
	payload, _ := json.Marshal(FileSaveRequest{Path: "private.json", Content: "visitor-only"})
	if rec := request(http.MethodPut, "/playground/api/files", payload, cookie, false); rec.Code != 200 {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	rec := request(http.MethodGet, "/private.json", nil, cookie, true)
	if rec.Code != 200 || rec.Body.String() != "visitor-only" || rec.Header().Get("Last-Modified") != "" {
		t.Fatalf("conditional private file: %d last-modified=%q body=%q", rec.Code, rec.Header().Get("Last-Modified"), rec.Body.String())
	}
	rangeRec := httptest.NewRecorder()
	rangeReq := httptest.NewRequest(http.MethodGet, "/private.json", nil)
	rangeReq.AddCookie(cookie)
	rangeReq.Header.Set("Range", "bytes=999999-")
	s.ServeHTTP(case339ResponseWriter{rangeRec}, rangeReq)
	if rangeRec.Code != http.StatusRequestedRangeNotSatisfiable || !strings.Contains(rangeRec.Header().Get("Cache-Control"), "no-store") {
		t.Fatalf("private file range error: %d cache-control=%q", rangeRec.Code, rangeRec.Header().Get("Cache-Control"))
	}
}

func TestCase339ReviewHealthProbeDoesNotCreateSessions(t *testing.T) {
	config, err := os.ReadFile("../../.do/app.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(config), "http_path: /healthz") {
		t.Error("deployment health probe does not use /healthz")
	}
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	s := NewServer(ws, ServerOptions{Public: true, RatePerMinute: 1000})
	defer s.Close()
	for i := 0; i < 100; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		req.RemoteAddr = "192.0.2.7:1"
		s.ServeHTTP(case339ResponseWriter{rec}, req)
		if rec.Code != 200 || len(rec.Result().Cookies()) != 0 || len(s.sessions.items) != 0 {
			t.Fatalf("probe %d: status=%d cookies=%d sessions=%d", i, rec.Code, len(rec.Result().Cookies()), len(s.sessions.items))
		}
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/playground/", nil)
	req.RemoteAddr = "192.0.2.7:1"
	s.ServeHTTP(case339ResponseWriter{rec}, req)
	if rec.Code != 200 || len(rec.Result().Cookies()) != 1 {
		t.Fatalf("visitor after health probes: %d", rec.Code)
	}
}
