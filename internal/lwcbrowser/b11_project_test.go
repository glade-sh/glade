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
	"github.com/glade-sh/glade/internal/lwc/compile"
	"github.com/glade-sh/glade/internal/project"
)

// Additional route coverage for the already captured export behaviors.
func TestB11SmallExportsProjectNative(t *testing.T) {
	if home := os.Getenv("GLADE_HOME"); home != "" && !filepath.IsAbs(home) {
		absolute, err := filepath.Abs(home)
		if err != nil {
			t.Fatal(err)
		}
		t.Setenv("GLADE_HOME", absolute)
	}
	p, err := project.Load("testdata/b11_small_exports/project")
	if err != nil {
		t.Fatal(err)
	}
	cache := t.TempDir()
	cfg, manifest, err := PreparePageConfig(p, cache)
	if err != nil {
		t.Fatal(err)
	}
	warmCfg, warmManifest, err := PreparePageConfig(p, cache)
	if err != nil {
		t.Fatalf("warm project: %v", err)
	}
	if warmManifest.OutDir != manifest.OutDir || importMapJSON(warmCfg) != importMapJSON(cfg) {
		t.Fatal("warm project changed its compiled route")
	}
	batch, err := compile.CompileBatch(p, compile.Options{OutDir: filepath.Join(t.TempDir(), "batch"), LightningModules: SalesforceImportMap()})
	if err != nil {
		t.Fatal(err)
	}
	for name, result := range batch {
		if result.Err != nil {
			t.Errorf("batch %s: %v", name, result.Err)
		}
	}
	if t.Failed() {
		return
	}
	f, err := os.Open("testdata/b11_small_exports/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	all, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	var rows [][]string
	for _, r := range all[1:] {
		if r[0] == "nav" || r[0] == "graph" || r[0] == "refresh" || r[0] == "mobile" || r[0] == "dial" || r[0] == "empty" || r[0] == "button" || r[0] == "group" {
			rows = append(rows, r)
		}
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
		w.Header().Set("Content-Type", "application/javascript")
		switch {
		case r.URL.Path == "/":
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, `<!doctype html><script>window.process={env:{NODE_ENV:"production"}}</script><script type="importmap">%s</script><script type="module">import "@lwc/synthetic-shadow"; import {createElement} from 'lwc';`, importMapJSON(cfg))
			for _, kind := range []string{"nav", "graph", "refresh", "mobile", "dial", "empty", "button", "group", "proxy"} {
				for _, api := range []string{"59", "62", "67"} {
					name := "b11" + kind + api
					fmt.Fprintf(w, "import %s from 'c/%s'; document.body.append(createElement('c-%s',{is:%s}));", name, name, name, name)
				}
			}
			fmt.Fprint(w, "</script>")
		case r.URL.Path == "/lightning/vendor/lwc.js":
			http.ServeFile(w, r, filepath.Join(root, "third_party/lwc/node_modules/@lwc/engine-dom/dist/index.js"))
		case r.URL.Path == "/lightning/vendor/synthetic-shadow.js":
			http.ServeFile(w, r, filepath.Join(root, "third_party/lwc/node_modules/@lwc/synthetic-shadow/dist/index.js"))
		case r.URL.Path == "/lightning/shims/lightning/navigation.js":
			fmt.Fprint(w, NavigationModuleJS())
		case r.URL.Path == "/lightning/shims/lightning/refresh.js":
			fmt.Fprint(w, RefreshModuleJS())
		case r.URL.Path == "/lightning/shims/lightning/mobileCapabilities.js":
			source, _ := SimulatedNativeAPIModuleJS("mobileCapabilities")
			fmt.Fprint(w, source)
		case strings.HasPrefix(r.URL.Path, "/lightning/shims/lightning/"):
			name := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/lightning/shims/lightning/"), ".js")
			if IsLightningBaseComponentModule(name) {
				fmt.Fprint(w, LightningBaseComponentModuleJS(name))
			} else {
				http.NotFound(w, r)
			}
		case strings.HasPrefix(r.URL.Path, "/lightning/runtime/"):
			path := strings.TrimPrefix(r.URL.Path, "/lightning/runtime/")
			if strings.HasSuffix(path, ".js") {
				path = strings.TrimSuffix(path, ".js") + ".mjs"
			}
			http.ServeFile(w, r, filepath.Join("..", "..", "lwcruntime", "src", filepath.Clean(path)))
		case strings.HasPrefix(r.URL.Path, "/lightning/modules/"):
			http.StripPrefix("/lightning/modules/", http.FileServer(http.Dir(manifest.OutDir))).ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	input, err := json.Marshal(map[string]any{"url": server.URL, "module": module, "executable": os.Getenv("GLADE_LWC_BROWSER_EXECUTABLE"), "rows": rows})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", "testdata/b11_small_exports/project.mjs")
	cmd.Stdin = bytes.NewReader(input)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("project browser: %v\n%s", err, out)
	}
}
