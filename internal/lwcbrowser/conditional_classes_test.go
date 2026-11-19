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

// The captured Lightning Experience rows: conditional SLDS classes follow
// component state. slds-has-error appears only after a failed reportValidity,
// slds-is-open only on an open combobox, slds-combobox never on a radio group,
// inputs carry slds-input and textareas slds-textarea, and a stateful button
// carries exactly one of slds-is-selected and slds-not-selected.
func TestConditionalBaseComponentClassesMatchNative(t *testing.T) {
	f, err := os.Open("testdata/conditional_classes/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 23 || strings.Join(rows[0], ",") != "case,api,route,result,outcome" {
		t.Fatalf("native matrix rows = %d header = %v", len(rows), rows[0])
	}
	shims := map[string]string{}
	for _, name := range []string{"input", "textarea", "combobox", "radioGroup", "buttonStateful"} {
		shims[name] = LightningBaseComponentModuleJS(name)
	}
	root, err := gladehome.Root()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "third_party/lwc/node_modules/@lwc/engine-dom/dist/index.js")); err != nil {
		t.Skipf("LWC engine unavailable: %v", err)
	}
	module := os.Getenv("GLADE_LWC_PLAYWRIGHT_MODULE")
	if module == "" {
		module = filepath.Join("..", "..", "lwcruntime", "node_modules", "playwright")
	}
	module, err = filepath.Abs(module)
	if err != nil {
		t.Fatal(err)
	}
	imports := map[string]string{"lwc": "/lwc.js", "@glade/shell/diagnostics": "/diagnostics.js", "@salesforce/i18n/locale": "/i18n/locale.js", "@salesforce/i18n/timeZone": "/i18n/timeZone.js"}
	for name := range shims {
		imports["lightning/"+name] = "/shim/" + name + ".js"
	}
	importMap, err := json.Marshal(map[string]any{"imports": imports})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/":
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, `<!doctype html><script>window.process={env:{NODE_ENV:"production"}}</script><script type="importmap">%s</script><script type="module" src="/entry.js"></script>`, importMap)
		case r.URL.Path == "/lwc.js":
			http.ServeFile(w, r, filepath.Join(root, "third_party/lwc/node_modules/@lwc/engine-dom/dist/index.js"))
		case r.URL.Path == "/diagnostics.js":
			w.Header().Set("Content-Type", "text/javascript")
			fmt.Fprint(w, `export function reportDiagnostic() {}`)
		case r.URL.Path == "/i18n/locale.js":
			w.Header().Set("Content-Type", "text/javascript")
			fmt.Fprint(w, `export default "en-US";`)
		case r.URL.Path == "/i18n/timeZone.js":
			w.Header().Set("Content-Type", "text/javascript")
			fmt.Fprint(w, `export default "UTC";`)
		case r.URL.Path == "/entry.js":
			w.Header().Set("Content-Type", "text/javascript")
			http.ServeFile(w, r, "testdata/conditional_classes/entry.mjs")
		case strings.HasPrefix(r.URL.Path, "/shim/"):
			w.Header().Set("Content-Type", "text/javascript")
			fmt.Fprint(w, shims[strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/shim/"), ".js")])
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	input, err := json.Marshal(map[string]any{"url": server.URL, "module": module, "executable": os.Getenv("GLADE_LWC_BROWSER_EXECUTABLE"), "rows": rows[1:]})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", "testdata/conditional_classes/browser.mjs")
	cmd.Stdin = bytes.NewReader(input)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("conditional class conformance: %v\n%s", err, output)
	}
}
