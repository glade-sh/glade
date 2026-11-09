package lwcbrowser

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/gladehome"
)

// Compare the native desktop Aura observations with browser-rendered local shims.
func TestB11ComponentBrowserNative(t *testing.T) {
	names := map[string]string{"dial": "clickToDial", "empty": "emptyState", "button": "richTextToolbarButton", "group": "richTextToolbarButtonGroup"}
	shims := map[string]string{}
	for id, name := range names {
		if !IsLightningBaseComponentModule(name) || SupportedLightningBaseComponentSpecifiers()["lightning/"+name] == "" {
			t.Errorf("%s: missing supported base component %s", id, name)
			continue
		}
		shims[name] = LightningBaseComponentModuleJS(name)
	}
	if t.Failed() {
		return
	}
	f, err := os.Open("testdata/b11_components/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	extraFile, err := os.Open("testdata/b11_component_extra/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer extraFile.Close()
	extraReader := csv.NewReader(extraFile)
	extraReader.Comma = '\t'
	extraReader.LazyQuotes = true
	extraRows, err := extraReader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	clickoutFile, err := os.Open("testdata/b11_component_clickout/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer clickoutFile.Close()
	clickoutReader := csv.NewReader(clickoutFile)
	clickoutReader.Comma = '\t'
	clickoutReader.LazyQuotes = true
	clickoutRows, err := clickoutReader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	root, err := gladehome.Root()
	if err != nil {
		t.Fatal(err)
	}
	module := os.Getenv("GLADE_LWC_PLAYWRIGHT_MODULE")
	if module == "" {
		module = filepath.Join("..", "..", "lwcruntime", "node_modules", "playwright")
	}
	module, err = filepath.Abs(module)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/":
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, `<!doctype html><script>window.process={env:{NODE_ENV:"production"}}</script><script type="importmap">{"imports":{"lwc":"/lwc.js","@glade/shell/diagnostics":"/diagnostics.js"}}</script><script type="module" src="/entry.js"></script>`)
		case r.URL.Path == "/lwc.js":
			http.ServeFile(w, r, filepath.Join(root, "third_party/lwc/node_modules/@lwc/engine-dom/dist/index.js"))
		case r.URL.Path == "/diagnostics.js":
			w.Header().Set("Content-Type", "text/javascript")
			fmt.Fprint(w, `export function reportDiagnostic() {}`)
		case r.URL.Path == "/entry.js":
			w.Header().Set("Content-Type", "text/javascript")
			http.ServeFile(w, r, "testdata/b11_components/entry.mjs")
		case strings.HasPrefix(r.URL.Path, "/shim/"):
			w.Header().Set("Content-Type", "text/javascript")
			name := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/shim/"), ".js")
			fmt.Fprint(w, shims[name])
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	input, err := json.Marshal(map[string]any{"url": server.URL, "module": module, "executable": os.Getenv("GLADE_LWC_BROWSER_EXECUTABLE"), "rows": rows[1:], "extraRows": extraRows[1:], "clickoutRows": clickoutRows[1:]})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", "testdata/b11_components/browser.mjs")
	cmd.Stdin = bytes.NewReader(input)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("component browser conformance: %v\n%s", err, output)
	}
}
