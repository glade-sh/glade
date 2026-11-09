package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/lwcbrowser"
)

// Server routes must deliver the same implementations exercised by the native
// row comparisons in internal/lwcbrowser/testdata/b11_small_exports/org.tsv.
func TestB11ServerShimRoutes(t *testing.T) {
	org := testOrg()
	handler := New(&org)
	mobile, _ := lwcbrowser.SimulatedNativeAPIModuleJS("mobileCapabilities")
	cases := map[string]string{
		"navigation": lwcbrowser.NavigationModuleJS(), "refresh": lwcbrowser.RefreshModuleJS(), "mobileCapabilities": mobile,
		"clickToDial": lwcbrowser.LightningBaseComponentModuleJS("clickToDial"), "emptyState": lwcbrowser.LightningBaseComponentModuleJS("emptyState"),
		"richTextToolbarButton": lwcbrowser.LightningBaseComponentModuleJS("richTextToolbarButton"), "richTextToolbarButtonGroup": lwcbrowser.LightningBaseComponentModuleJS("richTextToolbarButtonGroup"),
	}
	for name, want := range cases {
		for _, suffix := range []string{"", ".js"} {
			t.Run(name+suffix, func(t *testing.T) {
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/lightning/shims/lightning/"+name+suffix, nil))
				if rec.Code != http.StatusOK || rec.Body.String() != want {
					t.Fatalf("status %d or wrong shim body", rec.Code)
				}
			})
		}
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/lightning/runtime/lightning/graphql.js", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "export function executeMutation(") {
		t.Fatalf("GraphQL runtime status %d or missing export", rec.Code)
	}
}
