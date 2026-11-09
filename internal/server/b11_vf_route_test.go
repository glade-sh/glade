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

// The VF Lightning Out bootstrap shares the shim URLs covered by
// TestB11ServerShimRoutes; this checks their presence in its import map.
func TestB11VFHostShimRoutes(t *testing.T) {
	root, err := lightningTestRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	p, err := project.Load(filepath.Join(root, "testdata", "local-tests", "lightning-out-vf"))
	if err != nil {
		t.Fatal(err)
	}
	source, err := NewSourceMetadataFromProject(p)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	handler := newVisualforceHTMLTestServer(t, &org, source)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/apex/WidgetHost", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("VF status %d", rec.Code)
	}
	for _, name := range []string{"lightning/navigation", "lightning/graphql", "lightning/refresh", "lightning/mobileCapabilities", "lightning/clickToDial", "lightning/emptyState", "lightning/richTextToolbarButton", "lightning/richTextToolbarButtonGroup"} {
		if !strings.Contains(rec.Body.String(), name) {
			t.Errorf("VF import map lacks %s", name)
		}
	}
}
