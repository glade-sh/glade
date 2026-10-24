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

func mobileCapabilityRows(t *testing.T) [][]string {
	t.Helper()
	f, err := os.Open("testdata/mobile_capabilities/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	r.LazyQuotes = true
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("want header and two native API rows, got %d", len(rows))
	}
	return rows[1:]
}

// The same native component exercises both the bare shim token and its .js route.
func TestMobileCapabilitiesDesktopModuleRoutesNative(t *testing.T) {
	for _, row := range mobileCapabilityRows(t) {
		for _, token := range []string{"mobileCapabilities", "mobileCapabilities.js"} {
			t.Run(row[0]+"/"+token, func(t *testing.T) {
				source, ok := SimulatedNativeAPIModuleJS(token)
				if !ok {
					t.Fatal("missing mobile capability module")
				}
				input, err := json.Marshal(map[string]string{"source": source, "expected": row[2], "api": row[1][:2]})
				if err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				cmd := exec.CommandContext(ctx, "node", "testdata/mobile_capabilities/module.mjs")
				cmd.Stdin = bytes.NewReader(input)
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("module route: %v\n%s", err, out)
				}
			})
		}
	}
}

// Compile both public entry points and run the native synthetic project in a browser.
func TestMobileCapabilitiesDesktopProjectRoutesNative(t *testing.T) {
	module := os.Getenv("GLADE_LWC_PLAYWRIGHT_MODULE")
	if module == "" {
		module = filepath.Join("..", "..", "lwcruntime", "node_modules", "playwright")
	}
	module, err := filepath.Abs(module)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(module); err != nil {
		t.Skip("Playwright unavailable")
	}
	dependencies, err := gladehome.Root()
	if err != nil {
		t.Fatal(err)
	}
	p, err := project.Load("testdata/mobile_capabilities/project")
	if err != nil {
		t.Fatal(err)
	}
	var cfg PageConfig
	var compiled compile.Manifest
	cache := t.TempDir()
	t.Run("single_host_compile", func(t *testing.T) {
		var err error
		cfg, compiled, err = PreparePageConfig(p, cache)
		if err != nil {
			t.Fatalf("host compile route: %v", err)
		}
	})
	t.Run("batch_host_compile", func(t *testing.T) {
		batch, err := compile.CompileBatch(p, compile.Options{OutDir: filepath.Join(t.TempDir(), "batch"), LightningModules: SalesforceImportMap()})
		if err != nil {
			t.Fatal(err)
		}
		for name, result := range batch {
			if result.Err != nil {
				t.Errorf("batch route %s: %v", name, result.Err)
			}
		}
		if len(batch) != 2 {
			t.Fatalf("batch compiled %d bundles, want two", len(batch))
		}
	})
	if t.Failed() {
		return
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, `<!doctype html><script>window.process={env:{NODE_ENV:"production"}}</script><script type="importmap">%s</script><script type="module">import "@lwc/synthetic-shadow";import {createElement} from 'lwc';import A from 'c/deviceProbe62';import B from 'c/deviceProbe67';document.body.append(createElement('c-device-probe62',{is:A}),createElement('c-device-probe67',{is:B}));</script>`, importMapJSON(cfg))
		case "/lightning/vendor/lwc.js":
			http.ServeFile(w, r, filepath.Join(dependencies, "third_party/lwc/node_modules/@lwc/engine-dom/dist/index.js"))
		case "/lightning/vendor/synthetic-shadow.js":
			http.ServeFile(w, r, filepath.Join(dependencies, "third_party/lwc/node_modules/@lwc/synthetic-shadow/dist/index.js"))
		case "/lightning/shims/lightning/mobileCapabilities.js":
			source, _ := SimulatedNativeAPIModuleJS("mobileCapabilities")
			fmt.Fprint(w, source)
		default:
			if strings.HasPrefix(r.URL.Path, "/lightning/modules/") {
				http.StripPrefix("/lightning/modules/", http.FileServer(http.Dir(compiled.OutDir))).ServeHTTP(w, r)
			} else {
				http.NotFound(w, r)
			}
		}
	}))
	defer server.Close()
	input, err := json.Marshal(map[string]any{"url": server.URL, "module": module, "executable": os.Getenv("GLADE_LWC_BROWSER_EXECUTABLE"), "rows": mobileCapabilityRows(t)})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", "testdata/mobile_capabilities/browser.mjs")
	cmd.Stdin = bytes.NewReader(input)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("project browser route: %v\n%s", err, out)
	}
}
