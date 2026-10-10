package lwcbrowser

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/glade-sh/glade/internal/gladehome"
	"github.com/glade-sh/glade/internal/lwc/compile"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/resource"
)

type l15Case struct {
	ID                 string            `json:"id"`
	Group              string            `json:"group"`
	Kind               string            `json:"kind"`
	JS                 string            `json:"js"`
	Template           string            `json:"template"`
	ChannelBody        string            `json:"channel_body"`
	Expected           map[string]string `json:"expected"`
	DiagnosticExpected map[string]string `json:"diagnostic_expected"`
	DiagnosticCarried  map[string]string `json:"diagnostic_carried"`
	Carried            map[string]string `json:"carried"`
}

type l15RuntimeProject struct {
	Page  string            `json:"page"`
	Files map[string]string `json:"files"`
}

// TestL15SalesforceConformance replays all 200 native rows at source API 59/67.
// Sources and exact org.tsv answers were exported from the owned message
// service captures; CI neither imports the capture tools nor calls Salesforce.
// GLADE_L15_CAPTURE=1 reports mismatches without failing for before/after counts.
// Normal mode asserts matching rows and logs declared external-owner carries.
// Infrastructure and incomplete fixtures fail in either mode.
func TestL15SalesforceConformance(t *testing.T) {
	capture := os.Getenv("GLADE_L15_CAPTURE") != ""
	var cases []l15Case
	var runtimeProjects map[string]l15RuntimeProject
	l15ReadJSON(t, "testdata/l15_salesforce.json", &cases)
	l15ReadJSON(t, "testdata/l15_runtime.json", &runtimeProjects)
	versions := []string{"59.0", "67.0"}
	seen, groups := map[string]bool{}, map[string]bool{}
	var compilerCases, runtimeCases []l15Case
	metadataRows := 0
	for _, c := range cases {
		if c.ID == "" || seen[c.ID] || (c.Kind != "compile" && c.Kind != "runtime") {
			t.Fatalf("invalid or duplicate L15 row %q (%s)", c.ID, c.Kind)
		}
		seen[c.ID], groups[c.Group] = true, true
		for _, api := range versions {
			want, ok := c.Expected[api]
			if !ok {
				t.Fatalf("missing L15 native answer for %s at API %s", c.ID, api)
			}
			if c.Kind == "compile" {
				if want != "COMPILE_OK" && want != "COMPILE_ERROR" {
					t.Fatalf("invalid native compile answer for %s at API %s: %q", c.ID, api, want)
				}
				if want == "COMPILE_ERROR" && c.DiagnosticExpected[api] == "" {
					t.Fatalf("missing L15 native rejection diagnostic for %s at API %s", c.ID, api)
				}
			} else {
				var observation struct {
					ID    string `json:"id"`
					Stage string `json:"stage"`
				}
				if !strings.HasPrefix(want, "BROWSER|") || json.Unmarshal([]byte(strings.TrimPrefix(want, "BROWSER|")), &observation) != nil || observation.ID != c.ID || observation.Stage != "done" {
					t.Fatalf("missing or incomplete native DOM answer for %s at API %s", c.ID, api)
				}
			}
			for _, carry := range []string{c.Carried[api], c.DiagnosticCarried[api]} {
				if carry != "" && (c.Kind != "compile" || c.ChannelBody != "" || !strings.HasPrefix(carry, "L23: ") && !strings.HasPrefix(carry, "L07: ")) {
					t.Fatalf("L15 carry requires its external compiler owner: %s API %s", c.ID, api)
				}
			}
		}
		if c.Kind == "runtime" {
			runtimeCases = append(runtimeCases, c)
		} else if c.ChannelBody != "" {
			metadataRows++
		} else {
			compilerCases = append(compilerCases, c)
		}
	}
	if len(cases) != 200 || len(compilerCases) != 44 || metadataRows != 12 || len(runtimeCases) != 144 {
		t.Fatalf("L15 requires 44 compiler, 12 channel-metadata and 144 DOM rows; got %d/%d/%d", len(compilerCases), metadataRows, len(runtimeCases))
	}
	for _, group := range []string{"subscribe-lifecycle", "scopes", "cross-context-delivery"} {
		if !groups[group] {
			t.Fatalf("missing L15 group %s", group)
		}
	}
	// Missing Node/compiler/browser dependencies must not count as rejections.
	repo, err := gladehome.SourceRoot()
	if err != nil {
		t.Fatal(err)
	}
	dependencies, err := gladehome.EnsureRoot()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{
		filepath.Join(repo, "third_party/lwc/compile.mjs"),
		filepath.Join(dependencies, "third_party/lwc/node_modules/@lwc/compiler/package.json"),
		filepath.Join(dependencies, "third_party/lwc/node_modules/@lwc/engine-dom/dist/index.js"),
		filepath.Join(dependencies, "third_party/lwc/node_modules/@lwc/synthetic-shadow/dist/index.js"),
	} {
		if _, err := os.Stat(file); err != nil {
			t.Fatal(err)
		}
	}

	var report strings.Builder
	matches, total, compilerMatches, browserMatches, localBrowserRows := 0, 0, 0, 0, 0
	diagnosticMatches, diagnosticTotal, diagnosticCarried, carried := 0, 0, 0, 0
	for _, api := range versions {
		t.Logf("L15 API %s: compiling %d source fixtures", api, len(compilerCases))
		compiled := l15CompileRows(t, api, cases)
		runtimeProject, ok := runtimeProjects[api]
		if !ok || runtimeProject.Page == "" || len(runtimeProject.Files) != 12 {
			t.Fatalf("missing complete captured runtime project at API %s", api)
		}
		t.Logf("L15 API %s: source fixtures compiled; preparing %d DOM rows", api, len(runtimeCases))
		browser, browserErrors, runtimeErr := l15ObserveDOM(t, api, runtimeCases, runtimeProject, repo, dependencies)
		apiMatches := 0
		for index, c := range cases {
			want, got, reason := c.Expected[api], "COMPILE_OK", ""
			var compileErr error
			if c.Kind == "runtime" {
				if raw, ok := browser[c.ID]; ok {
					got, err = l15BrowserText(raw)
					if err != nil {
						t.Fatalf("invalid local observation %s: %v", c.ID, err)
					}
					localBrowserRows++
				} else {
					got = "BROWSER_ERROR"
					reason = "L15: local DOM observation missing"
					if browserErrors[c.ID] != "" {
						reason += ": " + browserErrors[c.ID]
					} else if runtimeErr != nil {
						reason += ": " + runtimeErr.Error()
					}
				}
			} else {
				compileErr = compiled[c.ID]
				if c.ChannelBody != "" {
					compileErr = l15LoadChannel(t, api, index, c.ChannelBody)
				}
				if compileErr != nil {
					got = "COMPILE_ERROR"
				}
				if got != want {
					reason = "L15: LMS fixture compile acceptance differs from native"
					if c.ChannelBody != "" {
						reason = "L15: message-channel metadata acceptance differs from native"
					}
					if compileErr != nil {
						reason += ": " + compileErr.Error()
					}
				}
			}
			status := "MISMATCH"
			// Go equality compares the entire text exactly: case, length and JSON
			// nulls are preserved. Never use Apex assertEquals or == for rows.
			if got == want {
				status = "MATCH"
				matches++
				apiMatches++
				if c.Kind == "runtime" {
					browserMatches++
				} else {
					compilerMatches++
				}
			} else if carry := c.Carried[api]; carry != "" {
				status, reason = "CARRIED", carry
				carried++
				t.Logf("%s API %s %s expected <%s> actual <%s>", c.ID, api, carry, want, got)
			} else if reason == "" {
				reason = "L15: LMS DOM observation differs at " + l15Difference(want, got)
			}
			total++
			fmt.Fprintf(&report, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", api, c.ID, c.Kind, status, got, want, l15OneLine(reason))
			if !capture && status == "MISMATCH" {
				t.Errorf("%s API %s expected <%s> actual <%s>: %s", c.ID, api, want, got, l15OneLine(reason))
			}
			if expected := c.DiagnosticExpected[api]; expected != "" {
				actual := l15RejectionDiagnostic(compileErr, c.ChannelBody != "")
				diagnosticStatus, diagnosticReason := "MISMATCH", "L15: rejection diagnostic differs from native"
				diagnosticTotal++
				if actual == expected {
					diagnosticStatus, diagnosticReason = "MATCH", ""
					diagnosticMatches++
				} else if status == "CARRIED" {
					diagnosticStatus, diagnosticReason = "CARRIED", reason
				} else if carry := c.DiagnosticCarried[api]; got == want && carry != "" {
					diagnosticStatus, diagnosticReason = "CARRIED", carry
				}
				if diagnosticStatus == "CARRIED" {
					diagnosticCarried++
					t.Logf("%s API %s diagnostic %s expected <%s> actual <%s>", c.ID, api, diagnosticReason, l15OneLine(expected), l15OneLine(actual))
				}
				fmt.Fprintf(&report, "%s\t%s\tdiagnostic\t%s\t%s\t%s\t%s\n", api, c.ID, diagnosticStatus, l15OneLine(actual), l15OneLine(expected), l15OneLine(diagnosticReason))
				if !capture && diagnosticStatus == "MISMATCH" {
					t.Errorf("%s API %s diagnostic expected <%s> actual <%s>", c.ID, api, l15OneLine(expected), l15OneLine(actual))
				}
			}
		}
		fmt.Fprintf(&report, "API_TOTAL\t%s\t%d/%d\n", api, apiMatches, len(cases))
	}
	fmt.Fprintf(&report, "TOTAL\t%d/%d\nCOMPILE_METADATA_TOTAL\t%d/112\nBROWSER_TOTAL\t%d/288\nLOCAL_BROWSER_ROWS\t%d/288\n", matches, total, compilerMatches, browserMatches, localBrowserRows)
	fmt.Fprintf(&report, "DIAGNOSTIC_TOTAL\t%d/%d\nDIAGNOSTIC_CARRIED_TOTAL\t%d/%d\nCARRIED_TOTAL\t%d/%d\n", diagnosticMatches, diagnosticTotal, diagnosticCarried, diagnosticTotal, carried, total)
	t.Logf("L15 matches %d/%d; compile/metadata %d/112; browser %d/288; local DOM rows %d/288", matches, total, compilerMatches, browserMatches, localBrowserRows)
	t.Logf("L15 rejection diagnostics %d/%d, carried %d/%d; carried rows %d/%d", diagnosticMatches, diagnosticTotal, diagnosticCarried, diagnosticTotal, carried, total)
	if path := os.Getenv("GLADE_L15_REPORT"); path != "" {
		if err := os.WriteFile(path, []byte(report.String()), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func l15ReadJSON(t *testing.T, file string, value any) {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, value); err != nil {
		t.Fatal(err)
	}
}

func l15WriteFile(t *testing.T, file, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func l15WriteProject(t *testing.T, root, api, packages string) {
	t.Helper()
	l15WriteFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"`+packages+`","default":true}],"namespace":"","sourceApiVersion":"`+api+`"}`)
}

const l15ChannelBody = `<masterLabel>L15 owned channel</masterLabel><isExposed>true</isExposed><description>Owned stable LMS fixture</description><lightningMessageFields><fieldName>value</fieldName><description>Fixture payload</description></lightningMessageFields>`

func l15WriteChannel(t *testing.T, root, name, body string) {
	t.Helper()
	l15WriteFile(t, filepath.Join(root, "force-app/main/default/messageChannels", name+".messageChannel-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?><LightningMessageChannel xmlns="http://soap.sforce.com/2006/04/metadata">`+body+`</LightningMessageChannel>`)
}

func l15LoadChannel(t *testing.T, api string, index int, body string) error {
	t.Helper()
	root := t.TempDir()
	l15WriteProject(t, root, api, "force-app")
	l15WriteChannel(t, root, "FamilyL15Channel", l15ChannelBody)
	l15WriteChannel(t, root, "FamilyL15Other", l15ChannelBody)
	l15WriteChannel(t, root, fmt.Sprintf("FamilyL15Case%03d", index), body)
	p, err := project.Load(root)
	if err != nil {
		return err
	}
	// Channel discovery, schema validation and loading all run in product code.
	registry, err := resource.LoadProject(p)
	if err == nil && (len(p.MessageChannelFiles) != 3 || len(registry.MessageChannels) != 3) {
		t.Fatalf("L15 channel loader requires all three fixture channels; discovered %d, loaded %d", len(p.MessageChannelFiles), len(registry.MessageChannels))
	}
	return err
}

func l15CompileRows(t *testing.T, api string, cases []l15Case) map[string]error {
	t.Helper()
	root := t.TempDir()
	keys := map[string]string{}
	for index, c := range cases {
		if c.Kind != "compile" || c.ChannelBody != "" {
			continue
		}
		fixture := filepath.Join(root, "cases", fmt.Sprintf("%04d", index))
		name := fmt.Sprintf("familyL15%03d", index)
		bundle := filepath.Join(fixture, "force-app/main/default/lwc", name)
		l15WriteFile(t, filepath.Join(bundle, name+".js"), strings.ReplaceAll(c.JS, "FamilyLms", strings.ToUpper(name[:1])+name[1:]))
		l15WriteFile(t, filepath.Join(bundle, name+".html"), c.Template)
		l15WriteFile(t, filepath.Join(bundle, name+".js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><isExposed>false</isExposed></LightningComponentBundle>`)
		l15WriteChannel(t, fixture, "FamilyL15Channel", l15ChannelBody)
		l15WriteChannel(t, fixture, "FamilyL15Other", l15ChannelBody)
		key, err := filepath.Rel(root, bundle)
		if err != nil {
			t.Fatal(err)
		}
		keys[c.ID] = filepath.ToSlash(key)
	}
	l15WriteProject(t, root, api, "cases")
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	results, err := compile.CompileBatch(p, compile.Options{OutDir: filepath.Join(root, "dist"), Namespace: "c"})
	if err != nil {
		t.Fatal(err)
	}
	errors := map[string]error{}
	for id, key := range keys {
		result, ok := results[key]
		if !ok {
			t.Fatalf("missing L15 batch compiler result for %s (%s)", id, key)
		}
		errors[id] = result.Err
	}
	return errors
}

var l15DiagnosticPosition = regexp.MustCompile(`\[Line: [0-9]+, Col: [0-9]+\]\s*`)
var l15DiagnosticSource = regexp.MustCompile(`/[^\s:]+\.js`)
var l15DiagnosticURL = regexp.MustCompile(`https?://[^\s"'<>]+`)

func l15RejectionDiagnostic(err error, metadata bool) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	if !metadata {
		message = strings.TrimPrefix(message, "lwc compile: exit status 1\n")
		message = strings.TrimSuffix(message, "\n")
		message = strings.TrimPrefix(message, "Error: ")
	}
	// Match the captured diagnostic transport: deployment positions and compiler
	// host paths are envelopes. Retain the full message and code frame, including
	// exact case and punctuation; never replace it with the expected diagnostic.
	message = l15DiagnosticPosition.ReplaceAllString(message, "")
	message = l15DiagnosticSource.ReplaceAllString(message, "<source>")
	return l15DiagnosticURL.ReplaceAllString(message, "<URL>")
}

func l15ObserveDOM(t *testing.T, api string, cases []l15Case, fixture l15RuntimeProject, repo, dependencies string) (map[string]json.RawMessage, map[string]string, error) {
	t.Helper()
	root := t.TempDir()
	for rel, content := range fixture.Files {
		if filepath.IsAbs(rel) || filepath.Clean(rel) != rel || strings.HasPrefix(rel, "../") {
			t.Fatalf("runtime fixture escapes L15 project: %s", rel)
		}
		l15WriteFile(t, filepath.Join(root, filepath.FromSlash(rel)), content)
	}
	p, err := project.Load(root)
	if err != nil {
		return nil, nil, err
	}
	if p.SourceAPIVersion != api {
		t.Fatalf("L15 runtime project API %s differs from %s", p.SourceAPIVersion, api)
	}
	if _, err := resource.LoadProject(p); err != nil {
		return nil, nil, err
	}
	compiled, err := compile.Compile(p, compile.Options{OutDir: filepath.Join(root, "dist"), Namespace: "c"})
	if err != nil {
		return nil, nil, fmt.Errorf("L15 captured runtime project compilation: %w", err)
	}
	entry, ok := compiled.Modules["c:"+fixture.Page]
	if !ok {
		return nil, nil, fmt.Errorf("L15 runtime page %s missing from compilation", fixture.Page)
	}
	manifest := ManifestFromCompile(compiled, "/lightning/modules")
	caseIDs := make([]string, len(cases))
	allowed := map[string]bool{}
	for index, c := range cases {
		caseIDs[index], allowed[c.ID] = c.ID, true
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		file := ""
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		switch r.URL.Path {
		case "/":
			id := r.URL.Query().Get("c__case")
			if !allowed[id] {
				http.NotFound(w, r)
				return
			}
			pageReference, _ := json.Marshal(map[string]any{"pageReference": map[string]any{"type": "standard__navItemPage", "attributes": map[string]any{"apiName": "FamilyL15Oracle" + api[:2]}, "state": map[string]string{"c__case": id}}})
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprintf(w, `<!doctype html><script>window.process={env:{NODE_ENV:"production"}}</script><script type="importmap">%s</script><script type="application/json" id="glade-lightning-config">%s</script><main></main><script type="module">import "@lwc/synthetic-shadow";import {createElement} from "lwc";import Page from "c/%s";document.querySelector("main").appendChild(createElement("%s",{is:Page}));</script>`, importMapJSON(PageConfig{Namespace: "c", Manifest: manifest}), pageReference, fixture.Page, entry.Tag)
			return
		case "/favicon.ico":
			w.WriteHeader(http.StatusNoContent)
			return
		case "/lightning/vendor/lwc.js":
			file = filepath.Join(dependencies, "third_party/lwc/node_modules/@lwc/engine-dom/dist/index.js")
		case "/lightning/vendor/synthetic-shadow.js":
			file = filepath.Join(dependencies, "third_party/lwc/node_modules/@lwc/synthetic-shadow/dist/index.js")
		case "/lightning/shims/lightning/messageService.js":
			fmt.Fprint(w, MessageServiceModuleJS())
			return
		case "/lightning/shims/lightning/navigation.js":
			fmt.Fprint(w, NavigationModuleJS())
			return
		case "/lightning/shims/messageChannel/FamilyL15Channel__c":
			fmt.Fprint(w, MessageChannelModuleJS("FamilyL15Channel__c"))
			return
		case "/lightning/shims/messageChannel/FamilyL15Other__c":
			fmt.Fprint(w, MessageChannelModuleJS("FamilyL15Other__c"))
			return
		default:
			switch {
			case strings.HasPrefix(r.URL.Path, "/lightning/modules/"):
				// Compiled entries import sibling .html.js and .css.js modules.
				// Serve the entire compiler output, as the product server does.
				file = filepath.Join(compiled.OutDir, filepath.FromSlash(strings.TrimPrefix(r.URL.Path, "/lightning/modules/")))
				rel, err := filepath.Rel(compiled.OutDir, file)
				if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
					http.NotFound(w, r)
					return
				}
			case strings.HasPrefix(r.URL.Path, "/lightning/runtime/shell/"):
				base := strings.TrimPrefix(r.URL.Path, "/lightning/runtime/shell/")
				if filepath.Base(base) == base && (strings.HasSuffix(base, ".js") || strings.HasSuffix(base, ".mjs")) {
					base = strings.TrimSuffix(base, ".js")
					if !strings.HasSuffix(base, ".mjs") {
						base += ".mjs"
					}
					file = filepath.Join(repo, "lwcruntime/src/shell", base)
				}
			}
		}
		content, err := os.ReadFile(file)
		if file == "" || err != nil {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(content)
	}))
	defer server.Close()
	playwright := os.Getenv("GLADE_LWC_PLAYWRIGHT_MODULE")
	if playwright == "" {
		playwright = filepath.Join(repo, "lwcruntime/node_modules/playwright")
	}
	// Pass only row IDs and launch settings, never native expectations.
	config, err := json.Marshal(map[string]any{"url": server.URL, "ids": caseIDs, "playwrightModule": playwright, "executablePath": os.Getenv("GLADE_LWC_BROWSER_EXECUTABLE"), "concurrency": browserRowConcurrency})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	t.Logf("L15 API %s: runtime compiled; observing DOM rows", api)
	cmd := exec.CommandContext(ctx, "node", filepath.Join(repo, "internal/lwcbrowser/testdata/l15_browser.mjs"))
	// Bound pipe draining if a killed Node leaves a browser descendant alive.
	cmd.WaitDelay = 5 * time.Second
	cmd.Stdin = bytes.NewReader(config)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		// Browser startup/transport is infrastructure, never a behavior answer.
		t.Fatalf("L15 local browser infrastructure at API %s (context %v): %v: %s", api, ctx.Err(), err, l15OneLine(stderr.String()))
	}
	var observed struct {
		Values map[string]json.RawMessage `json:"values"`
		Errors map[string]string          `json:"errors"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &observed); err != nil {
		t.Fatal(err)
	}
	for id := range observed.Values {
		if !allowed[id] {
			t.Fatalf("unknown L15 browser observation %s", id)
		}
	}
	for _, id := range caseIDs {
		if _, ok := observed.Values[id]; !ok && observed.Errors[id] == "" {
			t.Fatalf("missing L15 browser result/error %s", id)
		}
	}
	return observed.Values, observed.Errors, nil
}

// Match org.tsv's sorted, compact, ensure_ascii JSON serialization. Only
// object-key order and JSON encoding are canonicalized; values remain exact.
func l15BrowserText(raw json.RawMessage) (string, error) {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return "", err
	}
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return "", err
	}
	var text strings.Builder
	text.WriteString("BROWSER|")
	for _, r := range strings.TrimSuffix(encoded.String(), "\n") {
		if r < 0x7f {
			text.WriteRune(r)
		} else if r <= 0xffff {
			fmt.Fprintf(&text, `\u%04x`, r)
		} else {
			hi, lo := utf16.EncodeRune(r)
			fmt.Fprintf(&text, `\u%04x\u%04x`, hi, lo)
		}
	}
	return text.String(), nil
}

func l15Difference(want, got string) string {
	var expected, actual any
	if json.Unmarshal([]byte(strings.TrimPrefix(want, "BROWSER|")), &expected) != nil || json.Unmarshal([]byte(strings.TrimPrefix(got, "BROWSER|")), &actual) != nil {
		return "$"
	}
	return l15DifferencePath(expected, actual, "$")
}

func l15DifferencePath(want, got any, path string) string {
	if reflect.DeepEqual(want, got) {
		return ""
	}
	switch expected := want.(type) {
	case map[string]any:
		actual, ok := got.(map[string]any)
		if !ok {
			return path
		}
		keys := map[string]bool{}
		for key := range expected {
			keys[key] = true
		}
		for key := range actual {
			keys[key] = true
		}
		var ordered []string
		for key := range keys {
			ordered = append(ordered, key)
		}
		sort.Strings(ordered)
		for _, key := range ordered {
			expectedValue, wantPresent := expected[key]
			actualValue, gotPresent := actual[key]
			if wantPresent != gotPresent {
				return path + "." + key
			}
			if diff := l15DifferencePath(expectedValue, actualValue, path+"."+key); diff != "" {
				return diff
			}
		}
	case []any:
		actual, ok := got.([]any)
		if !ok || len(expected) != len(actual) {
			return path
		}
		for index, value := range expected {
			if diff := l15DifferencePath(value, actual[index], fmt.Sprintf("%s[%d]", path, index)); diff != "" {
				return diff
			}
		}
	}
	return path
}

func l15OneLine(value string) string {
	return strings.NewReplacer("\r", `\r`, "\n", `\n`, "\t", `\t`).Replace(value)
}

// browserRowConcurrency is the number of independent browser contexts that
// replay conformance rows at once. Rows are dominated by fixed observation
// windows, so this overlaps idle time without sharing any page state.
const browserRowConcurrency = 6
