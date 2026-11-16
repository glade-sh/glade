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
	"b07": "lightning-datatable renders a plain table shim: no navigation-mode header, row selection or row-error icon button, so the rowTitle accessible names are not produced",
	"b15": "lightning-input is a plain input: no date, time or datetime validation help text, locale format examples or date picker hints",
	"b18": "lightning-input-field is source-backed on local record endpoints; the Contact Email placeholder needs a record-edit-form fixture with field metadata that this harness does not build",
	"b25": "lightning-tabset renders every tab: it measures no width, so no overflow button or More Tabs menu appears",
}

// TestAPI68LWCBaseComponentNative replays the B01, B17 and B24 native rows
// against the browser-rendered local shims. CI neither imports the capture
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
	shims := map[string]string{}
	for _, name := range []string{"checkboxGroup", "inputAddress", "slider"} {
		shims[name] = LightningBaseComponentModuleJS(name)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/":
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, `<!doctype html><script>window.process={env:{NODE_ENV:"production"}}</script><script type="importmap">{"imports":{"lwc":"/lwc.js","@glade/shell/diagnostics":"/diagnostics.js"}}</script><script type="module" src="/entry.js"></script>`)
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
			fmt.Fprint(w, shims[strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/shim/"), ".js")])
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	input, err := json.Marshal(map[string]any{"url": server.URL, "module": module, "executable": os.Getenv("GLADE_LWC_BROWSER_EXECUTABLE"), "items": items})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
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

// api68LWCContains reports whether every native attribute is present with the
// same value and the visible text lines are equal. The shim may carry extra
// attributes the capture did not record.
func api68LWCContains(got, want any) bool {
	g, _ := got.(map[string]any)
	w, _ := want.(map[string]any)
	if !reflect.DeepEqual(g["lines"], w["lines"]) {
		return false
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
	return true
}
