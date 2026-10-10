package compile_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/gladehome"
	"github.com/glade-sh/glade/internal/lwc/compile"
	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/resource"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/server"
	"github.com/glade-sh/glade/internal/typesys"
)

type l12Case struct {
	ID                 string            `json:"id"`
	Group              string            `json:"group"`
	Kind               string            `json:"kind"`
	Basis              string            `json:"basis"`
	JS                 string            `json:"js"`
	Template           string            `json:"template"`
	MetaFragment       string            `json:"meta_fragment"`
	Expected           map[string]string `json:"expected"`
	DiagnosticExpected map[string]string `json:"diagnostic_expected"`
}

// TestL12SalesforceConformance compares every native org.tsv row as exact text.
// Inputs and API 59/67 observations are exported from the native GraphQL
// capture: 42 compile/metadata and 178 native DOM rows per API.
// The original metadata target row uses a separately captured isolated replay.
// Thirty additional API 59/67 review controls assert empty/null selection
// validation and the retained or/not/nin/like predicates.
// Comparison predicates were removed; their observations remain in the raw
// evidence export and are not counted as local support or carried assertions.
// Matching rows are asserted; named L07 differences are logged with their owner.
// GLADE_L12_REPORT selects the TSV destination, without changing assertions.
// CI never calls Salesforce.
func TestL12SalesforceConformance(t *testing.T) {
	t.Run("MetadataTargets", l12MetadataTargets)
	t.Run("BrowserRuntime", l12SalesforceConformance)
}

// These isolated deploy controls are additional observations. The original
// 220-row before/after denominator includes the isolated original-bundle replay.
func l12MetadataTargets(t *testing.T) {
	data, err := os.ReadFile("testdata/l12_metadata_targets.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []l12Case
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) != 3 {
		t.Fatalf("L12 requires three isolated metadata target controls, got %d", len(cases))
	}
	for _, api := range []string{"59.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			compiled := l12CompileCases(t, api, cases)
			matches := 0
			for _, c := range cases {
				if c.Kind != "compile" || c.Basis != "org" || c.Expected[api] == "" {
					t.Fatalf("invalid native metadata target control %s API %s", c.ID, api)
				}
				result := compiled[c.ID]
				got, diagnostic := "COMPILE_OK", ""
				if result.Err != nil {
					got = "COMPILE_ERROR"
					cause := result.Err
					for errors.Unwrap(cause) != nil {
						cause = errors.Unwrap(cause)
					}
					diagnostic = cause.Error()
				}
				want, diagnosticWant := c.Expected[api], c.DiagnosticExpected[api]
				if want == "COMPILE_ERROR" && diagnosticWant == "" {
					t.Fatalf("missing exact native target rejection for %s API %s", c.ID, api)
				}
				if got != want || diagnostic != diagnosticWant {
					t.Errorf("%s API %s expected <%s> diagnostic <%s> actual <%s> diagnostic <%s>", c.ID, api, want, diagnosticWant, got, diagnostic)
				} else {
					matches++
				}
			}
			t.Logf("L12 isolated metadata targets API %s matches %d/%d", api, matches, len(cases))
		})
	}
}

func l12SalesforceConformance(t *testing.T) {
	data, err := os.ReadFile("testdata/l12_salesforce.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []l12Case
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) != 220 {
		t.Fatalf("L12 requires 220 original rows, got %d", len(cases))
	}
	originalCount := len(cases)
	reviewData, err := os.ReadFile("testdata/l12_review_controls.json")
	if err != nil {
		t.Fatal(err)
	}
	var reviewCases []l12Case
	if err := json.Unmarshal(reviewData, &reviewCases); err != nil {
		t.Fatal(err)
	}
	if len(reviewCases) != 30 {
		t.Fatalf("L12 requires 30 retained review controls, got %d", len(reviewCases))
	}
	reviewIDs := map[string]bool{}
	for _, c := range reviewCases {
		if !strings.HasPrefix(c.ID, "r_control_") || c.Kind != "runtime" {
			t.Fatalf("invalid review control %s", c.ID)
		}
		reviewIDs[c.ID] = true
	}
	cases = append(cases, reviewCases...)
	versions := []string{"59.0", "67.0"}
	seen, groups := map[string]bool{}, map[string]bool{}
	var compileCases, runtimeCases []l12Case
	for _, c := range cases {
		if c.ID == "" || seen[c.ID] || c.Basis != "org" {
			t.Fatalf("invalid or duplicate L12 row %q", c.ID)
		}
		seen[c.ID], groups[c.Group] = true, true
		switch c.Kind {
		case "compile":
			if c.JS == "" || c.Template == "" {
				t.Fatalf("missing L12 compile input %s", c.ID)
			}
			compileCases = append(compileCases, c)
		case "runtime":
			runtimeCases = append(runtimeCases, c)
		default:
			t.Fatalf("invalid L12 row kind %s: %s", c.ID, c.Kind)
		}
		for _, api := range versions {
			want, ok := c.Expected[api]
			if !ok || (c.Kind == "compile" && want != "COMPILE_OK" && want != "COMPILE_ERROR") ||
				(c.Kind == "runtime" && (!strings.HasPrefix(want, "BROWSER|") || !json.Valid([]byte(strings.TrimPrefix(want, "BROWSER|"))))) {
				t.Fatalf("missing or invalid native L12 row %s API %s", c.ID, api)
			}
			if c.Kind == "compile" && want == "COMPILE_ERROR" && c.DiagnosticExpected[api] == "" {
				t.Fatalf("missing native L12 rejection diagnostic %s API %s", c.ID, api)
			}
		}
	}
	if len(cases) != 250 || len(compileCases) != 42 || len(runtimeCases) != 208 {
		t.Fatalf("L12 requires 42 compile and 208 DOM rows including review controls, got %d/%d", len(compileCases), len(runtimeCases))
	}
	for _, group := range []string{"envelope and errors", "aliases and fragments", "pagination", "nullability"} {
		if !groups[group] {
			t.Fatalf("missing L12 group %s", group)
		}
	}
	dependencyRoot, err := gladehome.Root()
	if err != nil {
		t.Fatal(err)
	}
	toolchain, err := gladehome.LWCToolchainDir()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"@lwc/compiler/package.json", "@lwc/engine-dom/dist/index.js", "@lwc/synthetic-shadow/dist/index.js"} {
		if _, err := os.Stat(filepath.Join(toolchain, "node_modules", filepath.FromSlash(rel))); err != nil {
			t.Fatal(err) // Missing tooling must never count as a native rejection.
		}
	}
	var report strings.Builder
	fmt.Fprintln(&report, "api\tid\tkind\tstatus\tactual\texpected\treason\tactual_diagnostic\texpected_diagnostic")
	matches, total, compileMatches, domMatches, localDOMRows := 0, 0, 0, 0, 0
	diagnosticMatches, diagnosticTotal := 0, 0
	for _, api := range versions {
		compiled := l12CompileCases(t, api, compileCases)
		dom, domErrors, productErr, browserErr := l12ObserveDOM(t, api, runtimeCases, dependencyRoot)
		if browserErr != nil {
			t.Fatalf("L12 browser environment: %v", browserErr)
		}
		apiMatches, originalMatches, reviewMatches := 0, 0, 0
		for _, c := range cases {
			want, got, reason := c.Expected[api], "", ""
			diagnosticGot, diagnosticWant := "", c.DiagnosticExpected[api]
			diagnosticCarry := ""
			if c.Kind == "compile" {
				result := compiled[c.ID]
				compileErr := result.Err
				got = "COMPILE_OK"
				if compileErr != nil {
					got = "COMPILE_ERROR"
				}
				if diagnosticWant != "" {
					diagnosticGot = "MISSING_DIAGNOSTIC"
					if len(result.Diagnostics) == 1 {
						diagnosticGot = result.Diagnostics[0].Message
					} else if compileErr != nil {
						// Metadata is rejected before Node; unwrap its path context.
						for errors.Unwrap(compileErr) != nil {
							compileErr = errors.Unwrap(compileErr)
						}
						diagnosticGot = compileErr.Error()
					}
					if result.Err != nil {
						// Original causes stay separate from the complete deployment
						// diagnostic; assert the latter when the compiler supplies it.
						const prefix = "lwc compile: exit status 1\nError: "
						if strings.HasPrefix(result.Err.Error(), prefix) {
							deployment := strings.TrimSuffix(strings.TrimPrefix(result.Err.Error(), prefix), "\n")
							if strings.HasPrefix(deployment, "[Line: ") && !strings.Contains(deployment, "\n    at ") {
								diagnosticGot = deployment
							}
						}
					}
					diagnosticGot = l12DiagnosticText(diagnosticGot)
					diagnosticWant = l12DiagnosticText(diagnosticWant)
					diagnosticTotal++
					if diagnosticGot == diagnosticWant {
						diagnosticMatches++
					} else {
						diagnosticCarry = l12WireDiagnosticCarries[c.ID]
						if diagnosticCarry != "" {
							t.Logf("L12 diagnostic carry %s API %s owner L07: %s; expected <%s> actual <%s>", c.ID, api, diagnosticCarry, diagnosticWant, diagnosticGot)
						} else {
							t.Errorf("%s diagnostic API %s expected <%s> actual <%s>", c.ID, api, diagnosticWant, diagnosticGot)
						}
					}
				}
				if got == want {
					compileMatches++
				} else {
					reason = "source compile differs from native"
					if compileErr != nil {
						reason += ": " + compileErr.Error()
					}
				}
			} else if raw, ok := dom[c.ID]; ok {
				// Shared serialization only: no fields removed, null unchanged.
				got, err = l07BrowserText(raw)
				if err != nil {
					t.Fatal(err)
				}
				localDOMRows++
				if got == want {
					domMatches++
				} else {
					reason = "GraphQL DOM observation differs from native"
				}
			} else {
				got, reason = "BROWSER_ERROR", "local DOM observation missing"
				if productErr != nil {
					reason += ": " + productErr.Error()
				} else if domErrors[c.ID] != "" {
					reason += ": " + domErrors[c.ID]
				}
			}
			status := "MISMATCH"
			if got == want {
				status = "MATCH"
				matches++
				apiMatches++
				if reviewIDs[c.ID] {
					reviewMatches++
				} else {
					originalMatches++
				}
			}
			if diagnosticWant != "" && diagnosticGot != diagnosticWant {
				if diagnosticCarry != "" {
					status += "+DIAGNOSTIC_CARRY"
					reason += "; owner L07: " + diagnosticCarry
				} else {
					status += "+DIAGNOSTIC_MISMATCH"
					reason += "; exact native rejection diagnostic differs"
				}
			}
			carry := ""
			if got == strings.ReplaceAll(want, `\ud800`, `\ufffd`) {
				carry = l12DOMCarries[c.ID]
			}
			if got != want && carry != "" {
				status = "CARRY"
				reason = "owner L07: " + carry
			}
			total++
			fmt.Fprintf(&report, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", api, c.ID, c.Kind, status, l07OneLine(got), l07OneLine(want), l07OneLine(reason), l07OneLine(diagnosticGot), l07OneLine(diagnosticWant))
			if got != want {
				if carry != "" {
					t.Logf("L12 carry %s API %s %s; expected <%s> actual <%s>", c.ID, api, reason, want, got)
				} else {
					t.Errorf("%s API %s expected <%s> actual <%s>: %s", c.ID, api, want, got, l07OneLine(reason))
				}
			}
		}
		fmt.Fprintf(&report, "API_TOTAL\t%s\t%d/%d\n", api, apiMatches, len(cases))
		fmt.Fprintf(&report, "ORIGINAL_API_TOTAL\t%s\t%d/%d\nREVIEW_API_TOTAL\t%s\t%d/%d\n", api, originalMatches, originalCount, api, reviewMatches, len(reviewCases))
		t.Logf("L12 API %s matches %d/%d", api, apiMatches, len(cases))
		t.Logf("L12 API %s original matches %d/%d; retained review controls %d/%d", api, originalMatches, originalCount, reviewMatches, len(reviewCases))
	}
	fmt.Fprintf(&report, "TOTAL\t%d/%d\nCOMPILE_TOTAL\t%d/%d\nDOM_TOTAL\t%d/%d\nLOCAL_DOM_ROWS\t%d/%d\n", matches, total, compileMatches, len(compileCases)*len(versions), domMatches, len(runtimeCases)*len(versions), localDOMRows, len(runtimeCases)*len(versions))
	fmt.Fprintf(&report, "DIAGNOSTIC_TOTAL\t%d/%d\n", diagnosticMatches, diagnosticTotal)
	t.Logf("L12 matches %d/%d; compile %d/%d; DOM %d/%d; observed local DOM %d/%d", matches, total, compileMatches, len(compileCases)*len(versions), domMatches, len(runtimeCases)*len(versions), localDOMRows, len(runtimeCases)*len(versions))
	t.Logf("L12 exact native rejection diagnostics %d/%d", diagnosticMatches, diagnosticTotal)
	if path := os.Getenv("GLADE_L12_REPORT"); path != "" {
		if err := os.WriteFile(path, []byte(report.String()), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

// Only the differing diagnostic is carried; each wire
// compile outcome is still asserted. A matching diagnostic is never carried.
var l12WireDiagnosticCarries = map[string]string{
	"c_wire_wire_duplicate": "wire transform lacks the native duplicate-decorator deployment envelope and formatted source frame",
	"c_wire_wire_public":    "wire transform lacks the native api/wire conflict deployment envelope and formatted source frame",
}

var l12DOMCarries = map[string]string{
	"r_variables_surrogate": "shared l07BrowserText replaces a lone UTF16 surrogate while canonicalizing JSON",
}

var l12CompilerPath = regexp.MustCompile(`(?:/[^\s:]+/)+(?P<file>familyL12\d+\.js)`)

// Only the compiler-host path is unstable. Preserve the complete captured
// diagnostic, including position, source frame, case and punctuation.
func l12DiagnosticText(message string) string {
	return l12CompilerPath.ReplaceAllString(message, "${file}")
}

func l12CompileCases(t *testing.T, api string, cases []l12Case) map[string]compile.BundleResult {
	t.Helper()
	root := t.TempDir()
	keys := map[string]string{}
	for index, c := range cases {
		name := fmt.Sprintf("familyL12%03d", index)
		rel := filepath.Join("cases", fmt.Sprintf("%04d", index), "force-app", "main", "default", "lwc", name)
		keys[c.ID] = filepath.ToSlash(rel)
		bundle := filepath.Join(root, rel)
		l07Write(t, filepath.Join(bundle, name+".js"), []byte(strings.ReplaceAll(c.JS, "FamilyUiGraphql", "FamilyL12"+fmt.Sprintf("%03d", index))))
		l07Write(t, filepath.Join(bundle, name+".html"), []byte(c.Template))
		fragment := c.MetaFragment
		if fragment == "" {
			fragment = "<isExposed>false</isExposed>"
		}
		l07Write(t, filepath.Join(bundle, name+".js-meta.xml"), []byte(`<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion>`+strings.ReplaceAll(fragment, "{api}", api)+`</LightningComponentBundle>`))
	}
	l07Copy(t, filepath.Join("testdata", "l12_runtime", "api"+api[:2], "force-app", "main", "default", "objects"), filepath.Join(root, "cases", "common", "force-app", "main", "default", "objects"))
	l07Write(t, filepath.Join(root, "sfdx-project.json"), []byte(`{"packageDirectories":[{"path":"cases","default":true}],"sourceApiVersion":"`+api+`"}`))
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	batch, err := compile.CompileBatch(p, compile.Options{OutDir: filepath.Join(root, "dist")})
	if err != nil {
		t.Fatal(err)
	}
	results := map[string]compile.BundleResult{}
	for _, c := range cases {
		result, ok := batch[keys[c.ID]]
		if !ok {
			t.Fatalf("missing L12 batch row %s", c.ID)
		}
		l07CheckCompilerEnvironment(t, result.Err)
		results[c.ID] = result
	}
	return results
}

func l12ObserveDOM(t *testing.T, api string, cases []l12Case, dependencyRoot string) (map[string]json.RawMessage, map[string]string, error, error) {
	t.Helper()
	root := t.TempDir()
	l07Copy(t, filepath.Join("testdata", "l12_runtime", "api"+api[:2]), root)
	p, err := project.Load(root)
	if err != nil {
		return nil, nil, nil, err
	}
	manifest, err := compile.Compile(p, compile.Options{OutDir: filepath.Join(root, "dist")})
	l07CheckCompilerEnvironment(t, err)
	if err != nil {
		return nil, nil, err, nil
	}
	page := "familyL12Runtime" + api[:2]
	entry, ok := manifest.Modules["c:"+page]
	if !ok {
		return nil, nil, fmt.Errorf("runtime entry %s missing", page), nil
	}
	localManifest := lwcbrowser.Manifest{Modules: map[string]lwcbrowser.ModuleEntry{}}
	for qualified, module := range manifest.Modules {
		rel, err := filepath.Rel(manifest.OutDir, module.File)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, nil, nil, fmt.Errorf("L12 module escapes output: %s", module.File)
		}
		localManifest.Modules[qualified] = lwcbrowser.ModuleEntry{URL: "/lightning/modules/" + filepath.ToSlash(rel), Tag: module.Tag}
	}
	registry, err := resource.LoadProject(p)
	if err != nil {
		return nil, nil, nil, err
	}
	schema, err := gladeschema.LoadProject(p)
	if err != nil {
		return nil, nil, nil, err
	}
	source, err := server.NewSourceMetadataFromProject(p)
	if err != nil {
		return nil, nil, nil, err
	}
	index := typesys.Build(p, schema)
	org := apextest.OrgFromIndex(index)
	org.Metadata = registry
	product := server.NewWithSource(&org, source)
	product.SetProjectIndex(index)
	defer product.ResetLightningCache()
	// Run the unchanged owned seed through the shared product endpoint. LWC's
	// API 59 floor is independent of the Apex seed's API 67 endpoint.
	seed, err := os.ReadFile("testdata/l12_runtime/seed.apex")
	if err != nil {
		return nil, nil, nil, err
	}
	body, err := json.Marshal(map[string]string{"anonymousBody": string(seed)})
	if err != nil {
		return nil, nil, nil, err
	}
	seedResponse := httptest.NewRecorder()
	product.ServeHTTP(seedResponse, httptest.NewRequest(http.MethodPost, "/services/data/v67.0/tooling/executeAnonymous", bytes.NewReader(body)))
	var seeded struct {
		Compiled bool `json:"compiled"`
		Success  bool `json:"success"`
	}
	if err := json.Unmarshal(seedResponse.Body.Bytes(), &seeded); err != nil {
		return nil, nil, nil, fmt.Errorf("decode owned seed response: %w", err)
	}
	if seedResponse.Code != http.StatusOK || !seeded.Compiled || !seeded.Success {
		return nil, nil, fmt.Errorf("owned seed failed: %s", seedResponse.Body.String()), nil
	}
	mux := http.NewServeMux()
	mux.Handle("/lightning/", product)
	mux.Handle("/services/", product)
	mux.Handle("/lightning/modules/", http.StripPrefix("/lightning/modules/", http.FileServer(http.Dir(manifest.OutDir))))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		bootstrap := lwcbrowser.BootstrapHTML(lwcbrowser.PageConfig{Namespace: "c", Manifest: localManifest, PageReference: map[string]any{"type": "standard__navItemPage", "attributes": map[string]string{"apiName": "FamilyL12Oracle" + api[:2]}, "state": map[string]string{"c__case": r.URL.Query().Get("c__case")}}})
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		// The native fixture is a rendered tab, so use the shared LWC host path.
		// Import errors stay visible instead of becoming Lightning Out dependency
		// errors. Construct the root before resolving appendChild for lifecycle hooks.
		fmt.Fprintf(w, `<!doctype html><script>window.__l12Error=null;</script><script id="glade-lwc-context" type="application/json">{}</script><main id="l12-host"></main>%s<script type="module">import "@lwc/synthetic-shadow";import {createElement} from "lwc";try{const {default:Page}=await import(%q);const el=createElement(%q,{is:Page});document.querySelector("#l12-host").appendChild(el);}catch(error){window.__l12Error={name:error.name,message:error.message};}</script>`, bootstrap, "c/"+page, entry.Tag)
	})
	local := httptest.NewServer(mux)
	defer local.Close()
	playwright := os.Getenv("GLADE_LWC_PLAYWRIGHT_MODULE")
	if playwright == "" {
		playwright = filepath.Join(dependencyRoot, "lwcruntime", "node_modules", "playwright")
	}
	observer, err := filepath.Abs("testdata/l12_browser.mjs")
	if err != nil {
		return nil, nil, nil, err
	}
	ids := make([]string, len(cases))
	allowed := map[string]bool{}
	for index, c := range cases {
		ids[index], allowed[c.ID] = c.ID, true
	}
	config, err := json.Marshal(map[string]any{"url": local.URL, "ids": ids, "playwrightModule": playwright, "executablePath": os.Getenv("GLADE_LWC_BROWSER_EXECUTABLE")})
	if err != nil {
		return nil, nil, nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", observer)
	cmd.Stdin = bytes.NewReader(config)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("local observer: %w: %s", err, l07OneLine(stderr.String()))
	}
	var observed struct {
		Values map[string]json.RawMessage `json:"values"`
		Errors map[string]string          `json:"errors"`
	}
	if err := json.Unmarshal(output, &observed); err != nil {
		return nil, nil, nil, err
	}
	for id := range observed.Values {
		if !allowed[id] || observed.Errors[id] != "" {
			return nil, nil, nil, fmt.Errorf("unexpected or conflicting local L12 row %s", id)
		}
	}
	for id := range observed.Errors {
		if !allowed[id] {
			return nil, nil, nil, fmt.Errorf("unexpected local L12 error row %s", id)
		}
	}
	if len(observed.Values)+len(observed.Errors) != len(cases) {
		return nil, nil, nil, fmt.Errorf("local observer returned %d/%d rows", len(observed.Values)+len(observed.Errors), len(cases))
	}
	return observed.Values, observed.Errors, nil, nil
}
