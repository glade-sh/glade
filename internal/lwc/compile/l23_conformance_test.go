package compile_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/glade-sh/glade/internal/gladehome"
	"github.com/glade-sh/glade/internal/lwc/compile"
	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/server"
	"github.com/glade-sh/glade/internal/storage"
)

type l23Case struct {
	ID          string              `json:"id"`
	Group       string              `json:"group"`
	Kind        string              `json:"kind"`
	Basis       string              `json:"basis"`
	BundleName  string              `json:"bundle_name"`
	JS          string              `json:"js"`
	Template    string              `json:"template"`
	Meta        string              `json:"meta_fragment"`
	Spec        json.RawMessage     `json:"spec"`
	Expected    map[string]string   `json:"expected"`
	Diagnostics map[string][]string `json:"diagnostics"`
}

type l23Runtime struct {
	Page  string            `json:"page"`
	Files map[string]string `json:"files"`
}

// TestL23SalesforceConformance follows the native capture adapter using the owned
// security-container operations were never invoked natively: the iframe srcdoc
// fixture was rejected. Their removal reasons are exported, not runtime answers.
// The remaining 100 compiler/metadata and 126 DOM rows per API compare exact
// text, including every JSON field and raw null. The shared L14 browser runner
// observes Glade's real tab host; native answers never enter the browser.
// Rejections also compare the captured diagnostics.json messages byte for byte,
// using the product deployment diagnostics, including their source positions.
// Capability answers use isolated capability controls at both APIs:
// unknown capabilities reject independently of exposure; valid twins compile.
// Isolated host-module controls retain the missing-import diagnostic and an
// exact valid-import twin, counted separately from the original before rows.
// GLADE_L23_CAPTURE=1 records behavior gaps without failing, and
// GLADE_L23_REPORT selects the per-row report. CI never contacts Salesforce.
func TestL23SalesforceConformance(t *testing.T) {
	capture := os.Getenv("GLADE_L23_CAPTURE") == "1"
	reportPath := os.Getenv("GLADE_L23_REPORT")
	if capture && reportPath == "" {
		t.Fatal("GLADE_L23_CAPTURE requires GLADE_L23_REPORT")
	}
	var data struct {
		Cases    []l23Case `json:"cases"`
		Controls []l23Case `json:"controls"`
		Removed  []struct {
			ID     string `json:"id"`
			Reason string `json:"reason"`
		} `json:"removed"`
	}
	l23Read(t, "testdata/l23_salesforce.json", &data)
	var fixtures map[string]l23Runtime
	l23Read(t, "testdata/l23_runtime.json", &fixtures)
	controlIDs := map[string]bool{}
	for _, c := range data.Controls {
		if c.Kind != "compile" {
			t.Fatalf("invalid L23 metadata control %q", c.ID)
		}
		controlIDs[c.ID] = true
	}
	allCases := append(append([]l23Case{}, data.Cases...), data.Controls...)
	seen, groups := map[string]bool{}, map[string]bool{}
	var compilerCases, runtimeCases []l23Case
	for _, c := range allCases {
		if c.ID == "" || seen[c.ID] || c.Basis != "org" {
			t.Fatalf("invalid or duplicate L23 row %q", c.ID)
		}
		seen[c.ID], groups[c.Group] = true, true
		for _, api := range []string{"59.0", "67.0"} {
			want, ok := c.Expected[api]
			if !ok {
				t.Fatalf("missing native L23 answer %s API %s", c.ID, api)
			}
			switch c.Kind {
			case "compile":
				if want != "COMPILE_OK" && want != "COMPILE_ERROR" {
					t.Fatalf("invalid native compile answer %s API %s", c.ID, api)
				}
				if messages, ok := c.Diagnostics[api]; !ok || messages == nil || (want == "COMPILE_ERROR" && len(messages) == 0) {
					t.Fatalf("missing native compiler diagnostics %s API %s", c.ID, api)
				}
			case "browser":
				var native struct {
					ID           string `json:"id"`
					Stage        string `json:"stage"`
					CaptureError string `json:"captureError"`
					Observation  struct {
						Kind string `json:"kind"`
					} `json:"observation"`
				}
				if !strings.HasPrefix(want, "BROWSER|") || json.Unmarshal([]byte(strings.TrimPrefix(want, "BROWSER|")), &native) != nil || native.ID != c.ID || native.Stage != "done" || native.CaptureError != "" || native.Observation.Kind == "native-metadata-boundary" {
					t.Fatalf("invalid or uninvoked native DOM answer %s API %s", c.ID, api)
				}
			default:
				t.Fatalf("unknown L23 kind %q", c.Kind)
			}
		}
		if c.Kind == "compile" {
			if c.JS == "" || c.Template == "" {
				t.Fatalf("missing compile input %s", c.ID)
			}
			compilerCases = append(compilerCases, c)
		} else {
			var spec struct {
				ID string `json:"id"`
			}
			if json.Unmarshal(c.Spec, &spec) != nil || spec.ID != c.ID {
				t.Fatalf("invalid runtime input %s", c.ID)
			}
			runtimeCases = append(runtimeCases, c)
		}
	}
	if len(data.Cases) != 226 || len(data.Controls) != 1 || len(compilerCases) != 101 || len(runtimeCases) != 126 || len(data.Removed) != 8 {
		t.Fatalf("L23 requires 100 primary compile/metadata, 1 metadata control, 126 DOM and 8 removed rows, got %d/%d/%d/%d", len(compilerCases)-len(data.Controls), len(data.Controls), len(runtimeCases), len(data.Removed))
	}
	for _, row := range data.Removed {
		if row.ID == "" || seen[row.ID] || row.Reason == "" {
			t.Fatalf("invalid removed L23 row %q", row.ID)
		}
		seen[row.ID] = true
	}
	for _, group := range []string{"quick action events", "flow screen events", "utility/workspace boundaries", "availability gating"} {
		if !groups[group] {
			t.Fatalf("missing L23 group %s", group)
		}
	}
	repo, err := gladehome.SourceRoot()
	if err != nil {
		t.Fatal(err)
	}
	dependencies, err := gladehome.EnsureRoot()
	if err != nil {
		t.Fatal(err)
	}
	// Missing tools are infrastructure failures, never native rejections.
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
	fmt.Fprintln(&report, "api\tid\tkind\tstatus\tactual\texpected\treason\tactual_diagnostics\texpected_diagnostics\tdiagnostic_status")
	matches, compileMatches, domMatches, domRows := 0, 0, 0, 0
	controlMatches := 0
	outcomeMatches, diagnosticMatches, diagnosticRows := 0, 0, 0
	for _, api := range []string{"59.0", "67.0"} {
		compiled := l23CompileCases(t, api, compilerCases)
		fixture, ok := fixtures[api]
		if !ok || fixture.Page != "familyL23Runtime"+api[:2] {
			t.Fatalf("missing captured L23 runtime fixture API %s", api)
		}
		dom := l23ObserveDOM(t, api, runtimeCases, fixture, dependencies)
		compileIndex, apiMatches, apiDOMRows, apiControlMatches := 0, 0, 0, 0
		for _, c := range allCases {
			want, got, reason := c.Expected[api], "COMPILE_OK", ""
			gotDiagnostics, wantDiagnostics, diagnosticStatus := "", "", ""
			diagnosticsMatch := true
			if c.Kind == "compile" {
				result := compiled[compileIndex]
				if compileErr := result.Err; compileErr != nil {
					got, reason = "COMPILE_ERROR", "L23: "+compileErr.Error()
				}
				messages := append([]string{}, result.DeploymentDiagnostics...)
				if len(messages) == 0 {
					// Metadata validation runs before the JavaScript compiler and
					// already exposes the native deployment message directly.
					for _, diagnostic := range result.Diagnostics {
						messages = append(messages, diagnostic.Message)
					}
				}
				if result.Err != nil && len(messages) == 0 {
					messages = append(messages, result.Err.Error())
				}
				for i, message := range messages {
					// browser_capture.sanitized redacts URLs in captured native
					// messages. Mirror that transport serialization and retain
					// all diagnostic text, positions and repeated messages.
					messages[i] = l23DiagnosticURL.ReplaceAllString(message, "<URL>")
				}
				gotDiagnostics, wantDiagnostics = l23JSON(t, messages), l23JSON(t, c.Diagnostics[api])
				diagnosticsMatch = gotDiagnostics == wantDiagnostics
				diagnosticStatus = "MISMATCH"
				if diagnosticsMatch {
					diagnosticStatus = "MATCH"
				}
				if want == "COMPILE_ERROR" {
					diagnosticRows++
					if diagnosticsMatch {
						diagnosticMatches++
					}
				}
				compileIndex++
			} else {
				got = dom[c.ID]
				if strings.HasPrefix(got, "BROWSER|") {
					domRows++
					apiDOMRows++
				} else {
					reason = got
				}
			}
			status := "MISMATCH"
			if got == want && !controlIDs[c.ID] {
				outcomeMatches++
			}
			if got == want && diagnosticsMatch {
				status = "MATCH"
				if controlIDs[c.ID] {
					controlMatches++
					apiControlMatches++
				} else {
					matches++
					apiMatches++
					if c.Kind == "compile" {
						compileMatches++
					} else {
						domMatches++
					}
				}
			} else {
				if got == want && !diagnosticsMatch {
					reason = "L23: native compiler diagnostic text differs"
				}
				if reason == "" {
					reason = "L23: local observation differs from native"
				}
				if !capture {
					if got != want {
						t.Errorf("%s API %s expected <%s> actual <%s>: %s", c.ID, api, want, got, reason)
					}
					if !diagnosticsMatch {
						t.Errorf("%s API %s expected diagnostics <%s> actual <%s>", c.ID, api, wantDiagnostics, gotDiagnostics)
					}
				}
			}
			fmt.Fprintf(&report, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", api, c.ID, c.Kind, status, l23OneLine(got), l23OneLine(want), l23OneLine(reason), l23OneLine(gotDiagnostics), l23OneLine(wantDiagnostics), diagnosticStatus)
		}
		fmt.Fprintf(&report, "API_TOTAL\t%s\t%d/226\nLOCAL_DOM_ROWS\t%s\t%d/126\n", api, apiMatches, api, apiDOMRows)
		fmt.Fprintf(&report, "API_CONTROL_TOTAL\t%s\t%d/%d\n", api, apiControlMatches, len(data.Controls))
		t.Logf("L23 API %s matches %d/226; local DOM rows %d/126", api, apiMatches, apiDOMRows)
	}
	fmt.Fprintf(&report, "TOTAL\t%d/452\nCOMPILE_TOTAL\t%d/200\nDOM_TOTAL\t%d/252\nLOCAL_DOM_ROWS\t%d/252\n", matches, compileMatches, domMatches, domRows)
	fmt.Fprintf(&report, "OUTCOME_TOTAL\t%d/452\nREJECTION_DIAGNOSTIC_TOTAL\t%d/%d\n", outcomeMatches, diagnosticMatches, diagnosticRows)
	fmt.Fprintf(&report, "CONTROL_TOTAL\t%d/%d\n", controlMatches, len(data.Controls)*2)
	for _, row := range data.Removed {
		fmt.Fprintf(&report, "REMOVED\t%s\t%s\n", row.ID, l23OneLine(row.Reason))
	}
	t.Logf("L23 matches %d/452; compile/metadata %d/200; native-vs-local DOM %d/252; observed DOM rows %d/252; 8 uninvoked native rows removed per API", matches, compileMatches, domMatches, domRows)
	t.Logf("L23 outcomes %d/452; exact rejection diagnostics %d/%d", outcomeMatches, diagnosticMatches, diagnosticRows)
	t.Logf("L23 isolated metadata controls %d/%d", controlMatches, len(data.Controls)*2)
	if reportPath != "" {
		l23Write(t, reportPath, report.String())
	}
}

func l23Read(t *testing.T, path string, dst any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, dst); err != nil {
		t.Fatal(err)
	}
}

func l23Write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
}

func l23CompileCases(t *testing.T, api string, cases []l23Case) []compile.BundleResult {
	t.Helper()
	root := t.TempDir()
	keys := make([]string, len(cases))
	for index, c := range cases {
		name := fmt.Sprintf("familyL23%03d", index)
		if c.BundleName != "" {
			name = c.BundleName
		}
		bundle := filepath.Join(root, "force-app/main/default/lwc", name)
		l23Write(t, filepath.Join(bundle, name+".js"), strings.ReplaceAll(c.JS, "FamilyHost", "Family"+strings.TrimPrefix(name, "family")))
		l23Write(t, filepath.Join(bundle, name+".html"), c.Template)
		fragment := c.Meta
		if fragment == "" {
			fragment = "<isExposed>false</isExposed>"
		}
		l23Write(t, filepath.Join(bundle, name+".js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion>`+fragment+`</LightningComponentBundle>`)
		key, err := filepath.Rel(root, bundle)
		if err != nil {
			t.Fatal(err)
		}
		keys[index] = filepath.ToSlash(key)
	}
	l23Write(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	batch, err := compile.CompileBatch(p, compile.Options{OutDir: filepath.Join(root, "dist"), Namespace: "c", LightningModules: lwcbrowser.SalesforceImportMap()})
	if err != nil {
		t.Fatal(err)
	}
	results := make([]compile.BundleResult, len(cases))
	for index, key := range keys {
		result, ok := batch[key]
		if !ok {
			t.Fatalf("missing L23 compiler result %s", key)
		}
		results[index] = result
	}
	return results
}

func l23ObserveDOM(t *testing.T, api string, cases []l23Case, fixture l23Runtime, dependencies string) map[string]string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range fixture.Files {
		if filepath.IsAbs(rel) || filepath.Clean(rel) != rel || strings.HasPrefix(rel, "../") {
			t.Fatalf("L23 fixture escapes project: %s", rel)
		}
		l23Write(t, filepath.Join(root, filepath.FromSlash(rel)), content)
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if p.SourceAPIVersion != api {
		t.Fatalf("L23 fixture API %s differs from %s", p.SourceAPIVersion, api)
	}
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	host := server.NewWithSource(&org, server.SourceMetadata{Project: p})
	defer host.ResetLightningCache()
	local := httptest.NewServer(host)
	defer local.Close()
	rows, allowed := []map[string]string{}, map[string]bool{}
	for _, c := range cases {
		query := url.Values{"c__case": {c.ID}}
		rows = append(rows, map[string]string{
			"id":       c.ID,
			"selector": "[data-l23-host]",
			"url":      local.URL + "/lightning/n/FamilyL23Oracle" + api[:2] + "?" + query.Encode(),
		})
		allowed[c.ID] = true
	}
	// Finish the real host's lazy compilation before starting Playwright.
	// An unsuccessful local host response is a behavior gap in capture mode.
	response, err := local.Client().Get(rows[0]["url"])
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 200 {
		values := make(map[string]string, len(cases))
		for _, c := range cases {
			values[c.ID] = fmt.Sprintf("BROWSER_ERROR|HOST_HTTP_%d|body=%s", response.StatusCode, body)
		}
		return values
	}
	playwright := os.Getenv("GLADE_LWC_PLAYWRIGHT_MODULE")
	if playwright == "" {
		playwright = filepath.Join(dependencies, "lwcruntime/node_modules/playwright")
	}
	observer, err := filepath.Abs("testdata/l14_browser.mjs")
	if err != nil {
		t.Fatal(err)
	}
	config := l23JSON(t, map[string]any{"rows": rows, "origin": local.URL, "playwrightModule": playwright, "executablePath": os.Getenv("GLADE_LWC_BROWSER_EXECUTABLE")})
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", observer)
	cmd.WaitDelay = 5 * time.Second
	cmd.Stdin = strings.NewReader(config)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	t.Logf("L23 API %s: observing %d DOM rows through the local tab host", api, len(rows))
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("L23 browser infrastructure API %s: %v (context %v): %s", api, err, ctx.Err(), stderr.String())
	}
	var observed struct {
		Values map[string]string `json:"values"`
	}
	if err := json.Unmarshal(output, &observed); err != nil {
		t.Fatal(err)
	}
	for id, value := range observed.Values {
		if !allowed[id] {
			t.Fatalf("unknown L23 observation %s", id)
		}
		if strings.HasPrefix(value, "BROWSER|") {
			var raw any
			decoder := json.NewDecoder(strings.NewReader(strings.TrimPrefix(value, "BROWSER|")))
			decoder.UseNumber()
			if err := decoder.Decode(&raw); err != nil {
				t.Fatalf("invalid L23 browser JSON %s: %v", id, err)
			}
			// Match native TSV serialization only: sorted keys and ASCII escapes.
			// Retain every scalar, null, field and array position.
			var canonical strings.Builder
			for _, r := range l23JSON(t, raw) {
				if r < 0x7f {
					canonical.WriteRune(r)
				} else if r <= 0xffff {
					fmt.Fprintf(&canonical, `\u%04x`, r)
				} else {
					hi, lo := utf16.EncodeRune(r)
					fmt.Fprintf(&canonical, `\u%04x\u%04x`, hi, lo)
				}
			}
			observed.Values[id] = "BROWSER|" + canonical.String()
		}
	}
	for _, c := range cases {
		if observed.Values[c.ID] == "" {
			t.Fatalf("missing L23 observation/error %s", c.ID)
		}
	}
	return observed.Values
}

var l23DiagnosticURL = regexp.MustCompile(`https?://[^\s"'<>]+`)

func l23JSON(t *testing.T, value any) string {
	t.Helper()
	var text bytes.Buffer
	encoder := json.NewEncoder(&text)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		t.Fatal(err)
	}
	return strings.TrimSuffix(text.String(), "\n")
}

func l23OneLine(text string) string {
	return strings.NewReplacer("\t", `\t`, "\r", `\r`, "\n", `\n`).Replace(text)
}
