package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestRuntimeServingRequiresSourceAuthority(t *testing.T) {
	base := t.TempDir()
	project, managed := filepath.Join(base, "project"), filepath.Join(base, "xdg", "glade")
	write := func(root, rel, content string) {
		t.Helper()
		name := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write(project, "go.mod", "module example.test/untrusted\n")
	for _, rel := range []string{"lwcruntime/src/shims/wire-adapter.mjs", "lwcruntime/src/shims/lwc-engine.mjs", "lwcruntime/src/shell/app.mjs", "lwcruntime/src/slds/slds-loader.mjs", "lwcruntime/src/slds/design-system/assets/icons/utility-sprite/svg/symbols.svg"} {
		write(project, rel, "project-asset")
		write(managed, rel, "managed-asset")
	}
	for _, rel := range []string{"third_party/lwc/compile.mjs", "third_party/lwc/node_modules/@lwc/compiler/index.js", "lwcruntime/src/shims/lds-cache.mjs", "lwcruntime/src/lightning/button.mjs"} {
		write(managed, rel, "fixture")
	}
	t.Chdir(project)
	t.Setenv("XDG_DATA_HOME", filepath.Join(base, "xdg"))
	t.Setenv("GLADE_HOME", "")
	t.Setenv("GLADE_ROOT", "")
	t.Setenv("GLADE_ALLOW_PROJECT_TOOLCHAIN", "")
	handler := New(&storage.OrgState{})
	for _, scenario := range []struct{ name, key, value, want string }{
		{"untrusted-project", "GLADE_ROOT", "", "managed-asset"},
		{"explicit-root", "GLADE_ROOT", project, "project-asset"},
		{"explicit-home", "GLADE_HOME", project, "project-asset"},
		{"project-opt-in", "GLADE_ALLOW_PROJECT_TOOLCHAIN", "1", "project-asset"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Setenv(scenario.key, scenario.value)
			for _, route := range []string{"/lightning/shims/core/wire-adapter.js", "/lightning/vendor/lwc.js", "/lightning/runtime/shell/app.js", "/lightning/runtime/slds/slds-loader.js", "/assets/icons/utility-sprite/svg/symbols.svg"} {
				t.Run(route, func(t *testing.T) {
					rec := httptest.NewRecorder()
					handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, route, nil))
					if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != scenario.want {
						t.Fatalf("%s: HTTP %d body %q, want %q", route, rec.Code, rec.Body.String(), scenario.want)
					}
				})
			}
		})
	}
}
