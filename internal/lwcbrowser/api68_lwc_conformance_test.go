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
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/gladehome"
)

// api68LWCGaps lists the API 68 capture items whose native rendering the local
// shell does not reproduce. The native rows (testdata/api68_lwc is exported
// from captures-lwc) show the same output at saved component API 67.0 and
// 68.0: these base-component changes follow the org release, not the version.
var api68LWCGaps = map[string]string{
	"m03_m04":       "toast timing: the local toast service has no timers, so a toast stays until cleared. The capture measured onScreenMs per mode with 100 ms polling (event dismissible and pester about 20.6 s, module dismissible about 5.1 s, the other modes still shown at 30 s); those are not exact timer constants",
	"b11_time_half": "a time typed into the datetime time input is not validated: the local datetime keeps one value and its time input has no input handler",
	"b15_picker":    "date and datetime pickers: the native picker button text (Select a date for Date) and the Date and Time sub-labels are not rendered; only the messages and hints are matched",
}

// TestAPI68LWCBaseComponentNative replays the B01, B07, B11, B15, B17, B18, B24
// and B25 native rows against the browser-rendered local shims. CI neither imports the capture
// tools nor calls Salesforce.
func TestAPI68LWCBaseComponentNative(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not available")
	}
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	module := os.Getenv("GLADE_LWC_PLAYWRIGHT_MODULE")
	if module == "" {
		module = filepath.Join(repoRoot, "lwcruntime", "node_modules", "playwright")
	}
	if _, err := os.Stat(module); err != nil {
		if os.Getenv("GLADE_LWC_PLAYWRIGHT_MODULE") != "" {
			t.Fatalf("configured Playwright module is unavailable: %v", err)
		}
		t.Skip("playwright node module not installed")
	}
	toolchain := repoRoot
	if _, err := os.Stat(filepath.Join(toolchain, "third_party", "lwc", "node_modules", "@lwc", "engine-dom")); err != nil {
		root, err := gladehome.EnsureRoot()
		if err != nil {
			t.Skipf("LWC toolchain node modules not installed: %v", err)
		}
		toolchain = root
	}
	file, err := os.Open("testdata/api68_lwc/native.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	var items []string
	native := map[string]json.RawMessage{}
	for _, row := range rows[1:] {
		if _, gap := api68LWCGaps[row[0]]; gap {
			t.Fatalf("%s is listed as a gap and has native rows", row[0])
		}
		key := row[0]
		if previous, seen := native[key]; seen {
			var a, b any
			_ = json.Unmarshal(previous, &a)
			_ = json.Unmarshal([]byte(row[2]), &b)
			if !reflect.DeepEqual(a, b) {
				t.Fatalf("%s native rows differ between API versions; the shim has no version gate", key)
			}
			continue
		}
		native[key] = json.RawMessage(row[2])
		items = append(items, key)
	}
	runtimeRoot := filepath.Join(repoRoot, "lwcruntime", "src")
	runtimeServer := http.StripPrefix("/lightning/runtime/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(runtimeRoot, filepath.FromSlash(r.URL.Path))
		if _, err := os.Stat(path); err != nil {
			// Source-backed components name .js files that live as .mjs.
			path = strings.TrimSuffix(path, ".js") + ".mjs"
		}
		switch filepath.Ext(path) {
		case ".css":
			w.Header().Set("Content-Type", "text/css")
		case ".woff2", ".woff", ".svg", ".png":
		default:
			w.Header().Set("Content-Type", "text/javascript")
		}
		http.ServeFile(w, r, path)
	}))
	imports := SalesforceImportMap()
	imports["lwc"] = "/lwc.js"
	imports["@lwc/synthetic-shadow"] = "/shadow.js"
	imports["@glade/shell/diagnostics"] = "/diagnostics.js"
	imports["@glade/slds"] = "/lightning/runtime/slds/slds-loader.js"
	importJSON := func(locale string) string {
		merged := map[string]string{}
		for key, value := range imports {
			merged[key] = value
		}
		if locale != "" {
			merged["@salesforce/i18n/locale"] = "/locale/" + locale + ".js"
		}
		raw, err := json.Marshal(merged)
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/":
			w.Header().Set("Content-Type", "text/html")
			locale := ""
			if strings.HasSuffix(r.URL.RawQuery, "_fr_FR") {
				locale = "fr-FR"
			}
			fmt.Fprintf(w, `<!doctype html><script>window.process={env:{NODE_ENV:"production"}}</script><script type="importmap">{"imports":%s}</script><script type="module" src="/entry.js"></script>`, importJSON(locale))
		case strings.HasPrefix(r.URL.Path, "/locale/"):
			w.Header().Set("Content-Type", "text/javascript")
			fmt.Fprintf(w, "export default %q;\n", strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/locale/"), ".js"))
		case r.URL.Path == "/shadow.js":
			w.Header().Set("Content-Type", "text/javascript")
			http.ServeFile(w, r, filepath.Join(toolchain, "third_party/lwc/node_modules/@lwc/synthetic-shadow/dist/index.js"))
		case strings.HasPrefix(r.URL.Path, "/lightning/shims/core/"):
			name := strings.TrimPrefix(r.URL.Path, "/lightning/shims/core/")
			path := filepath.Join(runtimeRoot, "shims", filepath.Base(name))
			if _, err := os.Stat(path); err != nil {
				path = strings.TrimSuffix(path, ".js") + ".mjs"
			}
			w.Header().Set("Content-Type", "text/javascript")
			http.ServeFile(w, r, path)
		case strings.HasPrefix(r.URL.Path, "/lightning/runtime/"):
			runtimeServer.ServeHTTP(w, r)
		case strings.HasPrefix(r.URL.Path, "/lightning/shims/i18n/"):
			w.Header().Set("Content-Type", "text/javascript")
			fmt.Fprint(w, I18nModuleJS(strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/lightning/shims/i18n/"), ".js")))
		case r.URL.Path == "/lightning/shims/lightning/uiObjectInfoApi.js":
			w.Header().Set("Content-Type", "text/javascript")
			fmt.Fprint(w, UIObjectInfoAPIModuleJS())
		case r.URL.Path == "/lightning/shims/lightning/uiRecordApi.js":
			w.Header().Set("Content-Type", "text/javascript")
			fmt.Fprint(w, UIRecordAPIModuleJS())
		case r.URL.Path == "/lwc.js":
			http.ServeFile(w, r, filepath.Join(toolchain, "third_party/lwc/node_modules/@lwc/engine-dom/dist/index.js"))
		case r.URL.Path == "/diagnostics.js":
			w.Header().Set("Content-Type", "text/javascript")
			fmt.Fprint(w, `export function reportDiagnostic() {}`)
		case r.URL.Path == "/entry.js":
			w.Header().Set("Content-Type", "text/javascript")
			http.ServeFile(w, r, "testdata/api68_lwc/entry.mjs")
		case strings.HasPrefix(r.URL.Path, "/shim/"):
			w.Header().Set("Content-Type", "text/javascript")
			fmt.Fprint(w, LightningBaseComponentModuleJS(strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/shim/"), ".js")))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	input, err := json.Marshal(map[string]any{"url": server.URL, "module": module, "executable": os.Getenv("GLADE_LWC_BROWSER_EXECUTABLE"), "items": items})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", "testdata/api68_lwc/browser.mjs")
	cmd.Stdin = bytes.NewReader(input)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("api68 lwc browser: %v\n%s\n%s", err, output, stderr.String())
	}
	var observed map[string]json.RawMessage
	if err := json.Unmarshal(output, &observed); err != nil {
		t.Fatalf("api68 lwc browser output: %v\n%s", err, output)
	}
	if string(observed["errors"]) != "[]" {
		t.Errorf("page errors: %s", observed["errors"])
	}
	for _, item := range items {
		var want, got any
		_ = json.Unmarshal(native[item], &want)
		if err := json.Unmarshal(observed[item], &got); err != nil {
			t.Fatalf("%s observation: %v", item, err)
		}
		if !api68LWCContains(got, want) {
			t.Errorf("%s does not match native\n  native: %s\n  glade:  %s", item, native[item], observed[item])
		}
	}
}

// api68LWCContains reports whether the glade observation satisfies a native
// row. A row with lines needs the visible text lines equal; a row with contains
// needs those lines in order among glade's lines and none of absent. Native
// attributes, error buttons, tab names, menu items and field input attributes
// must each match what the capture recorded. The shim may carry extra
// attributes and lines the capture row did not claim.
func api68LWCContains(got, want any) bool {
	g, _ := got.(map[string]any)
	w, _ := want.(map[string]any)
	if wantLines, ok := w["lines"]; ok && !reflect.DeepEqual(g["lines"], wantLines) {
		return false
	}
	gotLines, _ := g["lines"].([]any)
	if raw, ok := w["contains"].([]any); ok {
		next := 0
		for _, line := range gotLines {
			if next < len(raw) && line == raw[next] {
				next++
			}
		}
		if next != len(raw) {
			return false
		}
	}
	if raw, ok := w["absent"].([]any); ok {
		for _, absent := range raw {
			for _, line := range gotLines {
				if line == absent {
					return false
				}
			}
		}
	}
	gotAttrs, _ := g["attrs"].(map[string]any)
	wantAttrs, _ := w["attrs"].(map[string]any)
	for key, raw := range wantAttrs {
		have, _ := gotAttrs[key].(map[string]any)
		for name, value := range raw.(map[string]any) {
			if have[name] != value {
				return false
			}
		}
	}
	for _, key := range []string{"tabs", "menu"} {
		if value, ok := w[key]; ok && !reflect.DeepEqual(g[key], value) {
			return false
		}
	}
	if raw, ok := w["errorButtons"].([]any); ok {
		have, _ := g["errorButtons"].([]any)
		if len(have) != len(raw) {
			return false
		}
		for index := range raw {
			if !reflect.DeepEqual(have[index], raw[index]) {
				return false
			}
		}
	}
	if raw, ok := w["fieldInput"].(map[string]any); ok {
		have, _ := g["fieldInput"].(map[string]any)
		for name, value := range raw {
			if have[name] != value {
				return false
			}
		}
	}
	return true
}
