package playground

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCase339PublicRejectsHiddenPaths(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	server := NewServer(ws, ServerOptions{Public: true})
	t.Cleanup(server.Close)
	for _, path := range []string{".uploads/a.json", ".hidden.json", "ordinary/.hidden/b.json"} {
		body, err := json.Marshal(FileSaveRequest{Path: path, Content: "{}"})
		if err != nil {
			t.Fatal(err)
		}
		rec := httptest.NewRecorder()
		server.ServeHTTP(case339ResponseWriter{rec}, httptest.NewRequest(http.MethodPut, "/playground/api/files", bytes.NewReader(body)))
		if rec.Code < 400 {
			t.Errorf("save %q: status %d, want rejection", path, rec.Code)
		}
	}
}
