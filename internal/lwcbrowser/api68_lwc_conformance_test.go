package lwcbrowser

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/gladehome"
)

// api68LWCGaps lists the API 68 capture items whose native rendering the local
// shell does not reproduce. The native rows (testdata/api68_lwc is exported
// from captures-lwc) show the same output at saved component API 67.0 and
// 68.0: these base-component changes follow the org release, not the version.
var api68LWCGaps = map[string]string{}

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
	// Extra tab items. b25_4000_12_all measures the natural widths of 12 tabs and
	// the More button. The _menu items select a tab from the More menu after the
	// first render; their native answer is the matching _sel row.
	items = append(items, "b25_4000_12_all")
	menuNative := map[string]string{}
	for _, row := range []struct{ item, native string }{
		{"b25_260_12_menu3", "b25_260_12_sel3"}, {"b25_260_12_menu5", "b25_260_12_sel5"},
		{"b25_480_6_menu5", "b25_480_6_sel5"}, {"b25_480_6_menu4", "b25_480_6_sel4"}, {"b25_480_6_menu6", "b25_480_6_sel6"},
	} {
		items = append(items, row.item)
		menuNative[row.item] = row.native
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
		case r.URL.Path == "/lightning/shims/lightning/showToastEvent.js":
			w.Header().Set("Content-Type", "text/javascript")
			fmt.Fprint(w, ShowToastEventModuleJS())
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
	// Tab widths measured in this harness. When they equal the org's widths the
	// fixed-pixel native rows are checked as the reference; with another font
	// (Linux) only the derived expectation applies.
	var wide struct {
		TabWidths []float64 `json:"tabWidths"`
		MoreWidth float64   `json:"moreWidth"`
	}
	if err := json.Unmarshal(observed["b25_4000_12_all"], &wide); err != nil || len(wide.TabWidths) != 12 {
		t.Fatalf("b25 widths observation: %v %s", err, observed["b25_4000_12_all"])
	}
	orgWidths := []float64{112.81, 133.38, 118.03, 127.22, 113.64, 117, 137.61, 125.8, 119.06, 120.67, 140.7, 133.2}
	sameFont := math.Abs(wide.MoreWidth-82.41) < 0.5
	for index, width := range orgWidths {
		sameFont = sameFont && math.Abs(wide.TabWidths[index]-width) < 0.5
	}
	if !sameFont {
		t.Logf("b25 tab widths differ from the org capture (%v, more %v): fixed-pixel b25 rows are skipped, derived expectations apply", wide.TabWidths, wide.MoreWidth)
	}
	for _, item := range items {
		if item == "b25_4000_12_all" {
			continue
		}
		var got any
		if err := json.Unmarshal(observed[item], &got); err != nil {
			t.Fatalf("%s observation: %v", item, err)
		}
		if strings.HasPrefix(item, "b25") {
			api68LWCCheckTabs(t, item, observed[item], wide.TabWidths, wide.MoreWidth)
		}
		key := item
		if mapped, ok := menuNative[item]; ok {
			key = mapped
		}
		if strings.HasPrefix(item, "b25") && !sameFont {
			continue
		}
		var want any
		_ = json.Unmarshal(native[key], &want)
		if !api68LWCContains(got, want) {
			t.Errorf("%s does not match native\n  native: %s\n  glade:  %s", item, native[key], observed[item])
		}
	}
}

var api68TabItem = regexp.MustCompile(`^b25(?:_(\d+)_(\d+))?(?:_(?:sel|menu)(\d+))?(?:_all)?$`)

// api68LWCCheckTabs derives the tabs that stay in the bar from widths measured
// in this harness and the fit rule fitted to the native captures: the leading
// tabs plus the More button (unless every tab fits) plus a margin in (27.7,
// 31.4] px fit the container, a selected tab outside them joins the bar. Both
// ends of the margin range are accepted, so an observation inside the range
// passes. It does not assert the fitted constant itself.
func api68LWCCheckTabs(t *testing.T, item string, raw json.RawMessage, widths []float64, more float64) {
	t.Helper()
	m := api68TabItem.FindStringSubmatch(item)
	if m == nil {
		t.Errorf("%s: unrecognised tab item", item)
		return
	}
	container, count, active := 260.0, 12, -1
	if m[1] != "" {
		container, _ = strconv.ParseFloat(m[1], 64)
		count, _ = strconv.Atoi(m[2])
	}
	if m[3] != "" {
		active, _ = strconv.Atoi(m[3])
		active--
	}
	names := []string{"First", "Second", "Third", "Fourth", "Fifth", "Sixth", "Seventh", "Eighth", "Ninth", "Tenth", "Eleventh", "Twelfth"}
	bar := func(margin float64) []string {
		fits := func(sum float64, n int) bool {
			need := sum + margin
			if n < count {
				need += more
			}
			return need <= container
		}
		leading, total := 0, 0.0
		for n := 1; n <= count; n++ {
			total += widths[n-1]
			if fits(total, n) {
				leading = n
			}
		}
		if leading < 1 {
			leading = 1
		}
		var indexes []int
		if active < 0 || active < leading {
			for i := 0; i < leading; i++ {
				indexes = append(indexes, i)
			}
		} else {
			kept, sum := 0, widths[active]
			for k := 1; k < active; k++ {
				sum += widths[k-1]
				if !fits(sum, k+1) {
					break
				}
				kept = k
			}
			for i := 0; i < kept; i++ {
				indexes = append(indexes, i)
			}
			indexes = append(indexes, active)
		}
		var out []string
		for _, i := range indexes {
			out = append(out, names[i]+" long tab")
		}
		return out
	}
	var observed struct {
		Tabs []string `json:"tabs"`
		Menu []string `json:"menu"`
	}
	if err := json.Unmarshal(raw, &observed); err != nil {
		t.Errorf("%s: %v", item, err)
		return
	}
	lo, hi := bar(27.7), bar(31.4)
	if !reflect.DeepEqual(observed.Tabs, lo) && !reflect.DeepEqual(observed.Tabs, hi) {
		t.Errorf("%s: tabs in the bar %v, derived from measured widths %v (margin 31.4) to %v (margin 27.7)", item, observed.Tabs, hi, lo)
		return
	}
	var wantMenu []string
	for i := 0; i < count; i++ {
		name := names[i] + " long tab"
		inBar := false
		for _, tab := range observed.Tabs {
			inBar = inBar || tab == name
		}
		if !inBar {
			wantMenu = append(wantMenu, name)
		}
	}
	if !reflect.DeepEqual(observed.Menu, wantMenu) && !(len(observed.Menu) == 0 && len(wantMenu) == 0) {
		t.Errorf("%s: menu %v, want the tabs outside the bar %v", item, observed.Menu, wantMenu)
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
	if raw, ok := w["ariaInvalid"]; ok && !reflect.DeepEqual(g["ariaInvalid"], raw) {
		return false
	}
	// aria-describedby: per input, the elements it names, with their aria-live
	// and whether they hold text, in the captured order.
	if raw, ok := w["describedBy"]; ok && !reflect.DeepEqual(g["describedBy"], raw) {
		return false
	}
	if _, ok := w["toasts"]; ok && g["durationsFrozen"] != true {
		return false
	}
	if raw, ok := w["toasts"].(map[string]any); ok {
		have, _ := g["toasts"].(map[string]any)
		for kind, rawWant := range raw {
			want, _ := rawWant.(map[string]any)
			got, _ := have[kind].(map[string]any)
			if got == nil {
				return false
			}
			for _, key := range []string{"role", "live", "atomic", "close"} {
				if !reflect.DeepEqual(got[key], want[key]) {
					return false
				}
			}
			// Timer bands: present before presentUntilMs, gone by goneByMs. A
			// toast with no goneByMs is still shown at 30 s. Measurements, not
			// documented constants.
			gone, _ := got["goneAtMs"].(float64)
			if want["goneByMs"] == nil {
				if got["goneAtMs"] != nil {
					return false
				}
				continue
			}
			if got["goneAtMs"] == nil || gone <= want["presentUntilMs"].(float64) || gone > want["goneByMs"].(float64) {
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
