package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// Native native API59/67 converted resource URLs receive a tab document;
// stylesheets reject its HTML MIME type while scripts parse it on load.
func TestL17ShellResourceDocument(t *testing.T) {
	root, err := lightningTestRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	p, err := project.Load(filepath.Join(root, "testdata/local-tests/lwc-shell"))
	if err != nil {
		t.Fatal(err)
	}
	handler := newLightningTestServer(t, &storage.OrgState{}, SourceMetadata{Project: p})
	for _, row := range []struct {
		name, path, destination, policy string
		status                          int
	}{
		{"number_script", "/lwc/preview/tab/123", "script", "", http.StatusOK},
		{"number_style", "/lwc/preview/tab/123", "style", "nosniff", http.StatusOK},
		{"null_style", "/lwc/preview/tab/null", "style", "nosniff", http.StatusOK},
		{"undefined_style", "/lwc/preview/tab/undefined", "style", "nosniff", http.StatusOK},
		{"empty_style", "/lwc/preview/tab/Lwc_Probe", "style", "nosniff", http.StatusOK},
		{"other_missing_tab_resource", "/lwc/preview/tab/OtherMissing", "style", "nosniff", http.StatusOK},
		{"missing_tab_navigation", "/lwc/preview/tab/OtherMissing", "document", "", http.StatusNotFound},
		{"missing_tab_fetch", "/lwc/preview/tab/OtherMissing", "empty", "", http.StatusNotFound},
		{"missing_component_script", "/lwc/preview/component/c/OtherMissing", "script", "", http.StatusBadRequest},
	} {
		t.Run(row.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, row.path, nil)
			r.Header.Set("Sec-Fetch-Dest", row.destination)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if w.Code != row.status {
				t.Fatalf("HTTP %d want %d: %s", w.Code, row.status, w.Body.String())
			}
			if got := w.Header().Get("X-Content-Type-Options"); got != row.policy {
				t.Fatalf("MIME policy %q want %q", got, row.policy)
			}
			if row.status == http.StatusOK && (!strings.HasPrefix(w.Header().Get("Content-Type"), "text/html") || !strings.HasPrefix(w.Body.String(), "<!doctype html>")) {
				t.Fatal("resource response did not contain the production tab document")
			}
		})
	}
}
