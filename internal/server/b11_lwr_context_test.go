package server

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/lwcshell"
)

func TestB11LWRContextProductionRoute(t *testing.T) {
	for _, tc := range []struct {
		name, query string
		want        bool
	}{{"aura", "", false}, {"lwr", "?lwr=true", true}} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := lwcShellContextFromQuery(httptest.NewRequest("GET", "/lwc/preview/community/Example/Home"+tc.query, nil))
			ctx.Community.Site = "Example"
			merged := mergeCommunityRouteContext(lwcshell.PageContext{}, ctx)
			body, err := json.Marshal(merged.Community)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(body), `"lwr":true`) != tc.want {
				t.Fatalf("route context = %s", body)
			}
			html := renderLWCShellHTML(lwcbrowser.PageConfig{}, lwcshell.ShellPage{Context: merged})
			if strings.Contains(html, `"lwr":true`) != tc.want {
				t.Fatal("rendered host lost LWR context")
			}
		})
	}
}
