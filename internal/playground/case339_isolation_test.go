package playground

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCase339PublicVisitorsHaveIsolatedWorkspaces(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	s := NewServer(ws, ServerOptions{Public: true})
	t.Cleanup(s.Close)
	newVisitor := func() *http.Cookie {
		rec := httptest.NewRecorder()
		s.ServeHTTP(case339ResponseWriter{rec}, httptest.NewRequest(http.MethodGet, "/playground/api/workspace", nil))
		cookies := rec.Result().Cookies()
		if len(cookies) == 0 {
			t.Fatal("public visitor received no session cookie")
		}
		return cookies[0]
	}
	first, second := newVisitor(), newVisitor()
	if first.Value == second.Value {
		t.Fatal("visitors received the same session identity")
	}
	data, err := json.Marshal(FileSaveRequest{Path: "visitor.json", Content: "private visitor data"})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPut, "/playground/api/files", bytes.NewReader(data))
	req.AddCookie(first)
	rec := httptest.NewRecorder()
	s.ServeHTTP(case339ResponseWriter{rec}, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("save first visitor file: %d %s", rec.Code, rec.Body.String())
	}
	req = httptest.NewRequest(http.MethodGet, "/visitor.json", nil)
	req.AddCookie(second)
	rec = httptest.NewRecorder()
	s.ServeHTTP(case339ResponseWriter{rec}, req)
	if rec.Code == http.StatusOK || strings.Contains(rec.Body.String(), "private visitor data") {
		t.Fatalf("second visitor read first visitor file: %d %s", rec.Code, rec.Body.String())
	}
}

func TestCase339PublicSessionsIsolateDatabaseAndReset(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	s := NewServer(ws, ServerOptions{Public: true, RunTimeout: 30 * time.Second})
	t.Cleanup(s.Close)
	open := func() *http.Cookie {
		rec := httptest.NewRecorder()
		s.ServeHTTP(case339ResponseWriter{rec}, httptest.NewRequest(http.MethodGet, "/playground/api/workspace", nil))
		cookies := rec.Result().Cookies()
		if len(cookies) == 0 {
			t.Fatal("missing visitor session")
		}
		return cookies[0]
	}
	first, second := open(), open()
	request := func(method, path string, cookie *http.Cookie, body []byte) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, bytes.NewReader(body))
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		s.ServeHTTP(case339ResponseWriter{rec}, req)
		return rec
	}
	body, err := json.Marshal(RunRequest{AnonymousBody: "insert new Account(Name='VisitorPrivateMarker');"})
	if err != nil {
		t.Fatal(err)
	}
	rec := request(http.MethodPost, "/playground/api/run", first, body)
	var result RunResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil || result.Status != RunStatusPass {
		t.Fatalf("first visitor run: %d %s", rec.Code, rec.Body.String())
	}
	if rec := request(http.MethodGet, "/playground/api/database", first, nil); !strings.Contains(rec.Body.String(), "VisitorPrivateMarker") {
		t.Fatalf("first visitor lost its database: %s", rec.Body.String())
	}
	if rec := request(http.MethodGet, "/playground/api/database", second, nil); strings.Contains(rec.Body.String(), "VisitorPrivateMarker") {
		t.Fatalf("second visitor read private data: %s", rec.Body.String())
	}
	if rec := request(http.MethodPost, "/playground/api/reset", second, nil); rec.Code != http.StatusOK {
		t.Fatalf("second visitor reset: %d %s", rec.Code, rec.Body.String())
	}
	if rec := request(http.MethodGet, "/playground/api/database", first, nil); !strings.Contains(rec.Body.String(), "VisitorPrivateMarker") {
		t.Fatalf("second visitor reset first visitor database: %s", rec.Body.String())
	}
}

func TestCase339PublicSessionCookieSecurity(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	s := NewServer(ws, ServerOptions{Public: true})
	defer s.Close()
	rec := httptest.NewRecorder()
	s.ServeHTTP(case339ResponseWriter{rec}, httptest.NewRequest(http.MethodGet, "/playground/api/workspace", nil))
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == publicSessionCookie {
			if !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
				t.Fatalf("insecure session cookie: %#v", cookie)
			}
			return
		}
	}
	t.Fatal("missing session cookie")
}

func TestCase339PublicSessionsIsolateCacheAndExamples(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	oldCache := NewResultCache(filepath.Join(ws.DataRoot, "cache"))
	if err := oldCache.Store("synthetic", RunResult{Logs: []string{"OldPrivateMarker"}}); err != nil {
		t.Fatal(err)
	}
	s := NewServer(ws, ServerOptions{Public: true, ShowExamples: true})
	defer s.Close()
	request := func(method, path string, cookie *http.Cookie, body []byte) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, bytes.NewReader(body))
		if cookie != nil {
			req.AddCookie(cookie)
		}
		rec := httptest.NewRecorder()
		s.ServeHTTP(case339ResponseWriter{rec}, req)
		return rec
	}
	firstResponse := request(http.MethodGet, "/playground/api/workspace", nil, nil)
	secondResponse := request(http.MethodGet, "/playground/api/workspace", nil, nil)
	firstCookies, secondCookies := firstResponse.Result().Cookies(), secondResponse.Result().Cookies()
	if len(firstCookies) == 0 || len(secondCookies) == 0 {
		t.Fatal("missing visitor session")
	}
	first, second := firstCookies[0], secondCookies[0]
	for _, cookie := range []*http.Cookie{first, second} {
		if rec := request(http.MethodGet, "/playground/api/runs/latest", cookie, nil); strings.Contains(rec.Body.String(), "OldPrivateMarker") {
			t.Fatalf("public session exposed old cache: %s", rec.Body.String())
		}
	}
	if err := s.sessions.items[first.Value].server.runner.cache.Store("synthetic", RunResult{Logs: []string{"VisitorPrivateMarker"}}); err != nil {
		t.Fatal(err)
	}
	if rec := request(http.MethodGet, "/playground/api/runs/latest", second, nil); strings.Contains(rec.Body.String(), "VisitorPrivateMarker") {
		t.Fatalf("second session exposed first cache: %s", rec.Body.String())
	}
	examples := ListExampleProjects()
	if len(examples) == 0 {
		t.Fatal("missing builtin example")
	}
	body, err := json.Marshal(map[string]string{"id": examples[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	loaded := request(http.MethodPost, "/playground/api/examples/load", first, body)
	if loaded.Code != http.StatusOK {
		t.Fatalf("load example: %d %s", loaded.Code, loaded.Body.String())
	}
	var firstMeta, secondMeta WorkspaceMetadata
	if err := json.Unmarshal(request(http.MethodGet, "/playground/api/workspace", first, nil).Body.Bytes(), &firstMeta); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(request(http.MethodGet, "/playground/api/workspace", second, nil).Body.Bytes(), &secondMeta); err != nil {
		t.Fatal(err)
	}
	if firstMeta.ExampleID != examples[0].ID || secondMeta.ExampleID != "" {
		t.Fatalf("example isolation failed: first=%q second=%q", firstMeta.ExampleID, secondMeta.ExampleID)
	}
}
