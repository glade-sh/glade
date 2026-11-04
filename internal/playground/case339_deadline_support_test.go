package playground

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCase339PublicRejectsMissingBodyDeadlineSupport(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	s := NewServer(ws, ServerOptions{Public: true})
	t.Cleanup(s.Close)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/playground/api/reset", strings.NewReader(`{}`)))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want rejection when read deadlines are unsupported", rec.Code)
	}
}

func TestCase339PublicRejectsMissingWriteDeadlineSupport(t *testing.T) {
	rec := httptest.NewRecorder()
	writePublicRunResult(rec, http.StatusOK, RunResult{Status: RunStatusPass})
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want rejection when write deadlines are unsupported", rec.Code)
	}
}
