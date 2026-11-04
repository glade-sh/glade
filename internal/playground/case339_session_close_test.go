package playground

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestCase339PublicSessionCloseRemovesWorkspaces(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	s := NewServer(ws, ServerOptions{Public: true})
	t.Cleanup(s.Close)
	s.ServeHTTP(case339ResponseWriter{httptest.NewRecorder()}, httptest.NewRequest(http.MethodGet, "/playground/api/workspace", nil))
	var root string
	for _, session := range s.sessions.items {
		root = session.root
	}
	if root == "" {
		t.Fatal("session workspace was not created")
	}
	s.Close()
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("session workspace still exists after close: %v", err)
	}
}
