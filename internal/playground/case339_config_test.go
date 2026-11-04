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

	"github.com/glade-sh/glade/internal/project"
)

func TestCase339PublicRejectsWorkspaceConfiguration(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	server := NewServer(ws, ServerOptions{Public: true})
	t.Cleanup(server.Close)
	for _, path := range []string{"glade.yml", "glade.yaml", `nested\..\glade.yml`} {
		payload, err := json.Marshal(FileSaveRequest{Path: path, Content: "project:\n  schemaSnapshot: /dev/zero\n  schemaSnapshotSHA256: " + strings.Repeat("0", 64)})
		if err != nil {
			t.Fatal(err)
		}
		rec := httptest.NewRecorder()
		server.ServeHTTP(case339ResponseWriter{rec}, httptest.NewRequest(http.MethodPut, "/playground/api/files", bytes.NewReader(payload)))
		if rec.Code < 400 {
			t.Errorf("save %q: status %d, want rejection", path, rec.Code)
		}
	}
}

func TestCase339PublicCannotRemoveServerConfiguration(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	s := NewServer(ws, ServerOptions{Public: true})
	t.Cleanup(s.Close)
	rec := httptest.NewRecorder()
	s.ServeHTTP(case339ResponseWriter{rec}, httptest.NewRequest(http.MethodGet, "/playground/api/workspace", nil))
	var root string
	for _, session := range s.sessions.items {
		root = session.server.workspace.Root
	}
	if err := os.WriteFile(filepath.Join(root, "glade.yml"), []byte("# server configuration\n"), 0600); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodDelete, "/playground/api/files?path=glade.yml", nil)
	for _, cookie := range rec.Result().Cookies() {
		req.AddCookie(cookie)
	}
	rec = httptest.NewRecorder()
	s.ServeHTTP(case339ResponseWriter{rec}, req)
	if rec.Code < 400 {
		t.Fatalf("server configuration deleted: %d %s", rec.Code, rec.Body.String())
	}
}

func TestCase339PublicDoesNotInheritParentConfiguration(t *testing.T) {
	parent := t.TempDir()
	t.Setenv("TMPDIR", parent)
	if err := os.WriteFile(filepath.Join(parent, "glade.yml"), []byte("pdf:\n  command: /bin/false\n"), 0600); err != nil {
		t.Fatal(err)
	}
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	s := NewServer(ws, ServerOptions{Public: true})
	t.Cleanup(s.Close)
	s.ServeHTTP(case339ResponseWriter{httptest.NewRecorder()}, httptest.NewRequest(http.MethodGet, "/playground/api/workspace", nil))
	for _, session := range s.sessions.items {
		loaded, err := project.Load(session.server.workspace.Root)
		if err != nil {
			t.Fatal(err)
		}
		if loaded.PDF.Command != "" {
			t.Fatalf("public session inherited parent command %q", loaded.PDF.Command)
		}
		return
	}
	t.Fatal("session was not created")
}
