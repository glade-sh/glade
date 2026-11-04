package playground

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCase339PublicRejectsExternalPackageRoots(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	s := NewServer(ws, ServerOptions{Public: true})
	t.Cleanup(s.Close)
	rec := httptest.NewRecorder()
	s.ServeHTTP(case339ResponseWriter{rec}, httptest.NewRequest(http.MethodGet, "/playground/api/workspace", nil))
	var meta WorkspaceMetadata
	if err := json.Unmarshal(rec.Body.Bytes(), &meta); err != nil {
		t.Fatal(err)
	}
	version := 0
	for _, file := range meta.Files {
		if file.Path == "sfdx-project.json" {
			version = file.Version
		}
	}
	if version == 0 {
		t.Fatal("missing project manifest")
	}
	body, err := json.Marshal(FileSaveRequest{Path: "sfdx-project.json", Version: version, Content: `{"packageDirectories":[{"path":"/"}],"sourceApiVersion":"67.0"}`})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPut, "/playground/api/files", bytes.NewReader(body))
	for _, cookie := range rec.Result().Cookies() {
		req.AddCookie(cookie)
	}
	rec = httptest.NewRecorder()
	s.ServeHTTP(case339ResponseWriter{rec}, req)
	if rec.Code < 400 {
		t.Fatalf("external package root accepted: %d %s", rec.Code, rec.Body.String())
	}
}

func TestCase339PublicReferenceRejectsExternalPackageRoots(t *testing.T) {
	projectRoot := t.TempDir()
	manifest := `{"packageDirectories":[{"path":"/"}],"sourceApiVersion":"67.0"}`
	if err := os.WriteFile(filepath.Join(projectRoot, "sfdx-project.json"), []byte(manifest), 0600); err != nil {
		t.Fatal(err)
	}
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	s := NewServer(ws, ServerOptions{Public: true, ProjectReferences: []ProjectReference{{Name: "Reference", Path: projectRoot}}})
	t.Cleanup(s.Close)
	body, err := json.Marshal(map[string]string{"id": s.projectRefs[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	s.ServeHTTP(case339ResponseWriter{rec}, httptest.NewRequest(http.MethodPost, "/playground/api/examples/load", bytes.NewReader(body)))
	if rec.Code >= 400 {
		return // Rejecting the unsafe reference is also safe.
	}
	req := httptest.NewRequest(http.MethodGet, "/sfdx-project.json", nil)
	for _, cookie := range rec.Result().Cookies() {
		req.AddCookie(cookie)
	}
	rec = httptest.NewRecorder()
	s.ServeHTTP(case339ResponseWriter{rec}, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("read manifest: %d %s", rec.Code, rec.Body.String())
	}
	var loaded struct {
		PackageDirectories []struct {
			Path string `json:"path"`
		} `json:"packageDirectories"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &loaded); err != nil {
		t.Fatal(err)
	}
	for _, dir := range loaded.PackageDirectories {
		if filepath.IsAbs(dir.Path) || dir.Path == ".." || strings.HasPrefix(filepath.Clean(dir.Path), ".."+string(filepath.Separator)) {
			t.Fatalf("external package directory retained: %q", dir.Path)
		}
	}
}

func TestCase339PublicAllowsConfinedProjectManifest(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	s := NewServer(ws, ServerOptions{Public: true})
	t.Cleanup(s.Close)
	rec := httptest.NewRecorder()
	s.ServeHTTP(case339ResponseWriter{rec}, httptest.NewRequest(http.MethodGet, "/playground/api/workspace", nil))
	var meta WorkspaceMetadata
	if err := json.Unmarshal(rec.Body.Bytes(), &meta); err != nil {
		t.Fatal(err)
	}
	var version int
	for _, file := range meta.Files {
		if file.Path == "sfdx-project.json" {
			version = file.Version
		}
	}
	body, err := json.Marshal(FileSaveRequest{Path: "sfdx-project.json", Version: version, Content: `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPut, "/playground/api/files", bytes.NewReader(body))
	for _, cookie := range rec.Result().Cookies() {
		req.AddCookie(cookie)
	}
	rec = httptest.NewRecorder()
	s.ServeHTTP(case339ResponseWriter{rec}, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("confined project manifest rejected: %d %s", rec.Code, rec.Body.String())
	}
}

func TestCase339PublicRunsAtAPIFloors(t *testing.T) {
	for _, version := range []string{"62.0", "67.0"} {
		t.Run(version, func(t *testing.T) {
			ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir()})
			if err != nil {
				t.Fatal(err)
			}
			s := NewServer(ws, ServerOptions{Public: true, RunTimeout: 30 * time.Second})
			defer s.Close()
			rec := httptest.NewRecorder()
			s.ServeHTTP(case339ResponseWriter{rec}, httptest.NewRequest(http.MethodGet, "/playground/api/workspace", nil))
			var meta WorkspaceMetadata
			if err := json.Unmarshal(rec.Body.Bytes(), &meta); err != nil {
				t.Fatal(err)
			}
			var manifestVersion int
			for _, file := range meta.Files {
				if file.Path == "sfdx-project.json" {
					manifestVersion = file.Version
				}
			}
			cookies := rec.Result().Cookies()
			if len(cookies) == 0 || manifestVersion == 0 {
				t.Fatal("session or manifest missing")
			}
			manifest := `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"` + version + `"}`
			body, _ := json.Marshal(FileSaveRequest{Path: "sfdx-project.json", Version: manifestVersion, Content: manifest})
			req := httptest.NewRequest(http.MethodPut, "/playground/api/files", bytes.NewReader(body))
			req.AddCookie(cookies[0])
			rec = httptest.NewRecorder()
			s.ServeHTTP(case339ResponseWriter{rec}, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("save manifest: %d %s", rec.Code, rec.Body.String())
			}
			body, _ = json.Marshal(RunRequest{AnonymousBody: "System.debug('ok');"})
			req = httptest.NewRequest(http.MethodPost, "/playground/api/run", bytes.NewReader(body))
			req.AddCookie(cookies[0])
			rec = httptest.NewRecorder()
			s.ServeHTTP(case339ResponseWriter{rec}, req)
			var result RunResult
			if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil || result.Status != RunStatusPass || len(result.Logs) != 1 || result.Logs[0] != "ok" {
				t.Fatalf("run API %s: %d %s", version, rec.Code, rec.Body.String())
			}
		})
	}
}
