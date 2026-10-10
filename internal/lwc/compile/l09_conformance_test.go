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
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/gladehome"
	"github.com/glade-sh/glade/internal/lwc/compile"
	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/resource"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/server"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

type l09Case struct {
	ID                 string               `json:"id"`
	Group              string               `json:"group"`
	Kind               string               `json:"kind"`
	Basis              string               `json:"basis"`
	JS                 string               `json:"js"`
	NativeClassNames   map[string]string    `json:"native_class_names,omitempty"`
	Template           string               `json:"template"`
	MetaFragment       string               `json:"meta_fragment"`
	Expected           map[string]string    `json:"expected"`
	DiagnosticExpected map[string]string    `json:"diagnostic_expected"`
	Carry              *l07Carry            `json:"carry,omitempty"`
	DiagnosticCarries  map[string]*l07Carry `json:"diagnostic_carries,omitempty"`
	MismatchReason     string               `json:"mismatch_reason,omitempty"`
	Control            bool                 `json:"control,omitempty"`
	OracleRevision     string               `json:"oracle_revision,omitempty"`
	Payload            *l09PayloadProbe     `json:"payload,omitempty"`
}

type l09PayloadProbe struct {
	FixtureClock string                             `json:"fixture_clock"`
	UserName     string                             `json:"user_name"`
	Fields       []string                           `json:"fields"`
	Create       lwcbrowser.WireCreateRecordRequest `json:"create"`
}

// TestL09SalesforceConformance replays the owned LDS fixture through the product
// compiler, browser runtime and server, comparing each complete native row.
// The native context supplies relationship and date payload expectations.
// The same-source isolated schema rejection supersedes its batch acceptance.
// GLADE_L09_CAPTURE=1 records mismatches without failing; GLADE_L09_REPORT
// selects the per-row TSV. The fixture and oracle are credential-free exports.
func TestL09SalesforceConformance(t *testing.T) {
	t.Run("BrowserRuntime", l09SalesforceConformance)
}

func l09SalesforceConformance(t *testing.T) {
	capture := os.Getenv("GLADE_L09_CAPTURE") == "1"
	data, err := os.ReadFile("testdata/l09_salesforce.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []l09Case
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	versions := []string{"59.0", "67.0"}
	seen, groups := map[string]bool{}, map[string]bool{}
	var compileCases, runtimeCases, payloadCases []l09Case
	diagnosticRows := 0
	for _, c := range cases {
		if c.ID == "" || seen[c.ID] || c.Basis != "org" || len(c.Expected) != len(versions) {
			t.Fatalf("invalid or duplicate L09 row %q", c.ID)
		}
		seen[c.ID], groups[c.Group] = true, true
		if c.OracleRevision != "" {
			t.Logf("native oracle revision %s: %s", c.ID, c.OracleRevision)
		}
		carries := []*l07Carry{c.Carry}
		for api, carry := range c.DiagnosticCarries {
			if (api != "59.0" && api != "67.0") || carry == nil || c.DiagnosticExpected[api] == "" {
				t.Fatalf("invalid L09 diagnostic carry %s API %s", c.ID, api)
			}
			carries = append(carries, carry)
		}
		for _, carry := range carries {
			if carry != nil && (carry.Owner == "" || strings.HasPrefix(carry.Owner, "L09") || carry.Reason == "" || strings.ContainsAny(carry.Owner+carry.Reason, "\r\n\t")) {
				t.Fatalf("invalid L09 carry %s", c.ID)
			}
		}
		if strings.ContainsAny(c.MismatchReason, "\r\n\t") {
			t.Fatalf("invalid L09 mismatch reason %s", c.ID)
		}
		switch c.Kind {
		case "compile":
			if c.JS == "" || c.Template == "" || len(c.NativeClassNames) != len(versions) {
				t.Fatalf("missing L09 compile input %s", c.ID)
			}
			compileCases = append(compileCases, c)
		case "runtime":
			runtimeCases = append(runtimeCases, c)
		case "payload":
			if c.Payload == nil || len(c.Payload.Fields) == 0 || c.Payload.UserName == "" {
				t.Fatalf("missing L09 payload fixture %s", c.ID)
			}
			payloadCases = append(payloadCases, c)
		default:
			t.Fatalf("invalid L09 row kind %s: %s", c.ID, c.Kind)
		}
		if len(c.DiagnosticExpected) != 0 {
			diagnosticRows++
			if c.Kind != "compile" || len(c.DiagnosticExpected) != len(versions) {
				t.Fatalf("invalid L09 diagnostic row %s", c.ID)
			}
		}
		for _, api := range versions {
			if c.Kind == "compile" && c.NativeClassNames[api] == "" {
				t.Fatalf("missing captured L09 class name %s API %s", c.ID, api)
			}
			want, ok := c.Expected[api]
			if !ok || (c.Kind == "compile" && want != "COMPILE_OK" && want != "COMPILE_ERROR") ||
				(c.Kind != "compile" && (!strings.HasPrefix(want, "JSON|") || !json.Valid([]byte(strings.TrimPrefix(want, "JSON|"))))) {
				t.Fatalf("missing or invalid native L09 row %s API %s", c.ID, api)
			}
			if (want == "COMPILE_ERROR") != (c.DiagnosticExpected[api] != "") {
				t.Fatalf("missing or unexpected native L09 diagnostic %s API %s", c.ID, api)
			}
		}
	}
	if len(cases) != 368 || len(compileCases) != 58 || len(runtimeCases) != 309 || len(payloadCases) != 1 || diagnosticRows != 11 {
		t.Fatalf("L09 requires 58 compile, 309 DOM and one payload row with 11 rejection diagnostics, got %d/%d/%d/%d", len(compileCases), len(runtimeCases), len(payloadCases), diagnosticRows)
	}
	for _, group := range []string{"reads", "writes", "input-helpers", "cache-notifications"} {
		if !groups[group] {
			t.Fatalf("missing L09 group %s", group)
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
			t.Fatal(err) // Missing dependencies cannot count as native rejections.
		}
	}

	var report strings.Builder
	fmt.Fprintln(&report, "api\tid\tkind\tstatus\tactual\texpected\treason\tdiagnostic_status\tdiagnostic_actual\tdiagnostic_expected\tdiagnostic_reason")
	matches, total, compileMatches, domMatches, localDOMRows := 0, 0, 0, 0, 0
	payloadMatches := 0
	diagnosticMatches, diagnosticTotal := 0, 0
	carriedRows, diagnosticCarries := 0, 0
	originalMatches, controlMatches := 0, 0
	bundleNames := map[string]string{}
	for index, c := range compileCases {
		bundleNames[c.ID] = fmt.Sprintf("familyL09Compile%03d", index)
	}
	for _, api := range versions {
		compiled := l09CompileCases(t, api, compileCases)
		dom, domErrors, runtimeCompileErr, browserErr := l09ObserveDOM(t, api, runtimeCases, dependencyRoot)
		if browserErr != nil {
			t.Fatalf("L09 browser environment: %v", browserErr)
		}
		for id, value := range l09ObservePayload(t, api, payloadCases) {
			dom[id] = value
		}
		apiMatches, apiDOMRows, originalAPIMatches, controlAPIMatches := 0, 0, 0, 0
		for _, c := range cases {
			want, got, reason := c.Expected[api], "", ""
			diagnosticWant, diagnosticGot, diagnosticStatus := c.DiagnosticExpected[api], "", ""
			diagnosticCarry := c.DiagnosticCarries[api]
			diagnosticWant = strings.ReplaceAll(diagnosticWant, "{bundle}", bundleNames[c.ID])
			diagnosticReason := ""
			if c.Kind == "compile" {
				result := compiled[c.ID]
				got = "COMPILE_OK"
				if result.Err != nil {
					got = "COMPILE_ERROR"
				}
				if diagnosticWant != "" {
					diagnosticGot = "MISSING_DIAGNOSTIC"
					if len(result.Diagnostics) == 1 {
						diagnosticGot = result.Diagnostics[0].Message
					} else if len(result.Diagnostics) > 1 {
						encoded, err := json.Marshal(result.Diagnostics)
						if err != nil {
							t.Fatal(err)
						}
						diagnosticGot = string(encoded)
					} else if result.Err != nil {
						// Config errors wrap the native diagnostic with its local
						// filename. Compare the diagnostic cause, not that context.
						cause := result.Err
						for errors.Unwrap(cause) != nil {
							cause = errors.Unwrap(cause)
						}
						diagnosticGot = cause.Error()
					}
					diagnosticTotal++
					diagnosticStatus = "MISMATCH"
					diagnosticReason = "exact native rejection diagnostic differs"
					if diagnosticGot == diagnosticWant {
						diagnosticStatus = "MATCH"
						diagnosticReason = ""
						diagnosticMatches++
					} else if diagnosticCarry != nil {
						diagnosticStatus = "CARRY"
						diagnosticReason = diagnosticCarry.Owner + ": " + diagnosticCarry.Reason
						diagnosticCarries++
						t.Logf("carry %s API %s diagnostic: %s; expected <%s> actual <%s>", c.ID, api, diagnosticReason, diagnosticWant, diagnosticGot)
					} else if !capture {
						t.Errorf("%s API %s diagnostic expected <%s> actual <%s>", c.ID, api, diagnosticWant, diagnosticGot)
					}
				}
				if got == want {
					compileMatches++
				} else {
					reason = "source compile differs from native"
					if result.Err != nil {
						reason += ": " + result.Err.Error()
					}
				}
			} else if raw, ok := dom[c.ID]; ok {
				// Reuse L07's native compact/sorted/ensure_ascii serializer.
				// It preserves every field and raw null; no semantic normalization.
				text, err := l07BrowserText(raw)
				if err != nil {
					t.Fatal(err)
				}
				got = "JSON|" + strings.TrimPrefix(text, "BROWSER|")
				if got != want {
					reason = "LDS observation differs from native"
				}
				if c.Kind == "runtime" {
					localDOMRows++
					apiDOMRows++
					if got == want {
						domMatches++
					}
				} else if got == want {
					payloadMatches++
				}
			} else {
				got, reason = "BROWSER_ERROR", "local DOM observation missing"
				if runtimeCompileErr != nil {
					got = "BROWSER_COMPILE_ERROR"
					reason += ": " + runtimeCompileErr.Error()
				} else if domErrors[c.ID] != "" {
					reason += ": " + domErrors[c.ID]
				}
			}
			status := "MISMATCH"
			if got != want && c.MismatchReason != "" && got != "BROWSER_ERROR" && got != "BROWSER_COMPILE_ERROR" {
				reason = "" + c.MismatchReason
			}
			if got == want {
				status = "MATCH"
				matches++
				apiMatches++
				if c.Control {
					controlAPIMatches++
				} else {
					originalAPIMatches++
				}
			} else if c.Carry != nil && got != "BROWSER_ERROR" && got != "BROWSER_COMPILE_ERROR" {
				status = "CARRY"
				reason = c.Carry.Owner + ": " + c.Carry.Reason
				carriedRows++
				t.Logf("carry %s API %s: %s; expected <%s> actual <%s>", c.ID, api, reason, want, got)
			} else if !capture {
				t.Errorf("%s API %s expected <%s> actual <%s>: %s", c.ID, api, want, got, l07OneLine(reason))
			}
			total++
			fmt.Fprintf(&report, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", api, c.ID, c.Kind, status, l07OneLine(got), l07OneLine(want), l07OneLine(reason), diagnosticStatus, l07OneLine(diagnosticGot), l07OneLine(diagnosticWant), l07OneLine(diagnosticReason))
		}
		fmt.Fprintf(&report, "API_TOTAL\t%s\t%d/%d\nLOCAL_DOM_ROWS\t%s\t%d/%d\n", api, apiMatches, len(cases), api, apiDOMRows, len(runtimeCases))
		t.Logf("L09 API %s matches %d/%d; observed local DOM %d/%d", api, apiMatches, len(cases), apiDOMRows, len(runtimeCases))
		fmt.Fprintf(&report, "ORIGINAL_API_TOTAL\t%s\t%d/248\nCONTROL_API_TOTAL\t%s\t%d/120\n", api, originalAPIMatches, api, controlAPIMatches)
		t.Logf("L09 API %s original rows %d/248; native controls %d/120", api, originalAPIMatches, controlAPIMatches)
		originalMatches += originalAPIMatches
		controlMatches += controlAPIMatches
	}
	fmt.Fprintf(&report, "TOTAL\t%d/%d\nCOMPILE_TOTAL\t%d/%d\nDOM_TOTAL\t%d/%d\nDIAGNOSTIC_TOTAL\t%d/%d\n", matches, total, compileMatches, len(compileCases)*len(versions), domMatches, len(runtimeCases)*len(versions), diagnosticMatches, diagnosticTotal)
	fmt.Fprintf(&report, "PAYLOAD_TOTAL\t%d/%d\n", payloadMatches, len(payloadCases)*len(versions))
	fmt.Fprintf(&report, "CARRIED_ROWS\t%d\nCARRIED_DIAGNOSTICS\t%d\n", carriedRows, diagnosticCarries)
	fmt.Fprintf(&report, "ORIGINAL_TOTAL\t%d/496\nCONTROL_TOTAL\t%d/240\n", originalMatches, controlMatches)
	t.Logf("L09 matches %d/%d; compile %d/%d; DOM %d/%d; observed local DOM %d/%d; diagnostics %d/%d", matches, total, compileMatches, len(compileCases)*len(versions), domMatches, len(runtimeCases)*len(versions), localDOMRows, len(runtimeCases)*len(versions), diagnosticMatches, diagnosticTotal)
	t.Logf("L09 native payload matches %d/%d", payloadMatches, len(payloadCases)*len(versions))
	if path := os.Getenv("GLADE_L09_REPORT"); path != "" {
		l07Write(t, path, []byte(report.String()))
	}
}

func l09ObservePayload(t *testing.T, api string, cases []l09Case) map[string]json.RawMessage {
	t.Helper()
	p, err := project.Load(filepath.Join("testdata", "l09_runtime", "api"+api[:2]))
	if err != nil {
		t.Fatal(err)
	}
	source, err := server.NewSourceMetadataFromProject(p)
	if err != nil {
		t.Fatal(err)
	}
	observed := map[string]json.RawMessage{}
	for _, c := range cases {
		fixture := c.Payload
		clock, err := time.Parse(time.RFC3339, fixture.FixtureClock)
		if err != nil {
			t.Fatal(err)
		}
		// Native context masks raw timestamps. This explicit CI clock is not a
		// claimed native instant; it fixes the UTC context for its display check.
		org := storage.NewOrgState()
		org.APIVersion, org.Now = api, func() time.Time { return clock }
		storage.EnsureDeterministicPlatformData(&org)
		user := org.Objects["User"]
		record := user.Records[storage.ID("005000000000001")]
		record.Fields["Name"] = storage.StringValue(fixture.UserName)
		user.Records[record.ID] = record
		org.Objects["User"] = user
		body, err := json.Marshal(fixture.Create)
		if err != nil {
			t.Fatal(err)
		}
		product := server.NewWithSource(&org, source)
		response := httptest.NewRecorder()
		product.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/lightning/wire/createRecord", bytes.NewReader(body)))
		var envelope struct {
			Data  map[string]any `json:"data"`
			Error any            `json:"error"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		answer := map[string]any{"error": envelope.Error}
		if fields, ok := envelope.Data["fields"].(map[string]any); ok && envelope.Error == nil {
			projection := map[string]any{}
			for _, name := range fixture.Fields {
				field, ok := fields[name].(map[string]any)
				if !ok {
					projection[name] = fields[name]
					continue
				}
				value := field["value"]
				if related, ok := value.(map[string]any); ok {
					// Keep the captured label and nested Name wrapper, leaving
					// relationship identity/audit metadata outside this projection.
					nested, _ := related["fields"].(map[string]any)
					value = map[string]any{"fields": map[string]any{"Name": nested["Name"]}}
				} else if text, ok := value.(string); ok && strings.HasSuffix(text, "Z") {
					if _, err := time.Parse(time.RFC3339Nano, text); err == nil {
						value = "[DATETIME]" // Same raw-timestamp mask as native stable().
					}
				}
				projection[name] = map[string]any{"displayValue": field["displayValue"], "value": value}
			}
			answer = map[string]any{"returned": projection}
		}
		encoded, err := json.Marshal(answer)
		if err != nil {
			t.Fatal(err)
		}
		observed[c.ID] = encoded
	}
	return observed
}

func l09CompileCases(t *testing.T, api string, cases []l09Case) map[string]compile.BundleResult {
	t.Helper()
	root := t.TempDir()
	keys := map[string]string{}
	for index, c := range cases {
		name := fmt.Sprintf("familyL09Compile%03d", index)
		rel := filepath.Join("cases", fmt.Sprintf("%04d", index), "force-app", "main", "default", "lwc", name)
		keys[c.ID] = filepath.ToSlash(rel)
		bundle := filepath.Join(root, rel)
		// Replay the captured source name: native diagnostic columns depend on
		// its width, and the original API 59/67 captures used different names.
		l07Write(t, filepath.Join(bundle, name+".js"), []byte(strings.ReplaceAll(c.JS, "FamilyCompile", c.NativeClassNames[api])))
		l07Write(t, filepath.Join(bundle, name+".html"), []byte(c.Template))
		fragment := c.MetaFragment
		if fragment == "" {
			fragment = "<isExposed>false</isExposed>"
		}
		l07Write(t, filepath.Join(bundle, name+".js-meta.xml"), []byte(`<?xml version="1.0" encoding="UTF-8"?><LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion>`+fragment+`</LightningComponentBundle>`))
	}
	l07Write(t, filepath.Join(root, "sfdx-project.json"), []byte(`{"packageDirectories":[{"path":"cases","default":true}],"namespace":"","sourceApiVersion":"`+api+`"}`))
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
			t.Fatalf("missing L09 batch row %s", c.ID)
		}
		l07CheckCompilerEnvironment(t, result.Err)
		results[c.ID] = result
	}
	return results
}

func l09ObserveDOM(t *testing.T, api string, cases []l09Case, dependencyRoot string) (map[string]json.RawMessage, map[string]string, error, error) {
	t.Helper()
	root := t.TempDir()
	l07Copy(t, filepath.Join("testdata", "l09_runtime", "api"+api[:2]), root)
	p, err := project.Load(root)
	if err != nil {
		return nil, nil, nil, err
	}
	manifest, err := compile.Compile(p, compile.Options{OutDir: filepath.Join(root, "dist")})
	l07CheckCompilerEnvironment(t, err)
	if err != nil {
		return nil, nil, err, nil
	}
	page := "familyL09Runtime" + api[:2]
	if _, ok := manifest.Modules["c:"+page]; !ok {
		return nil, nil, fmt.Errorf("runtime entry %s missing", page), nil
	}
	localManifest := lwcbrowser.Manifest{Modules: map[string]lwcbrowser.ModuleEntry{}}
	for qualified, module := range manifest.Modules {
		rel, err := filepath.Rel(manifest.OutDir, module.File)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, nil, nil, fmt.Errorf("L09 module escapes output: %s", module.File)
		}
		localManifest.Modules[qualified] = lwcbrowser.ModuleEntry{URL: "/lightning/modules/" + filepath.ToSlash(rel), Tag: module.Tag}
	}
	bootstrap := lwcbrowser.BootstrapHTML(lwcbrowser.PageConfig{Namespace: "c", Manifest: localManifest})
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
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	l09SeedNativeModifier(t, &org, api, cases)
	org.Metadata = registry
	product := server.NewWithSource(&org, source)
	product.SetProjectIndex(typesys.Build(p, schema))
	mux := http.NewServeMux()
	mux.Handle("/lightning/", product) // Product LDS and Apex routes; no oracle mocks.
	mux.Handle("/lightning/modules/", http.StripPrefix("/lightning/modules/", http.FileServer(http.Dir(manifest.OutDir))))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><script>window.__l09Error=null;</script><script id="glade-lwc-context" type="application/json">{}</script><main id="l09-host"></main>%s<script>window.$Lightning.createComponent(%q,{},"l09-host",(_el,status,message)=>{if(status!=="SUCCESS")window.__l09Error={name:"Error",message};});</script>`, bootstrap, "c:"+page)
	})
	local := httptest.NewServer(mux)
	defer local.Close()
	playwright := os.Getenv("GLADE_LWC_PLAYWRIGHT_MODULE")
	if playwright == "" {
		playwright = filepath.Join(dependencyRoot, "lwcruntime", "node_modules", "playwright")
	}
	observer, err := filepath.Abs("testdata/l09_browser.mjs")
	if err != nil {
		return nil, nil, nil, err
	}
	ids := make([]string, len(cases))
	known := map[string]bool{}
	for index, c := range cases {
		ids[index], known[c.ID] = c.ID, true
	}
	config, err := json.Marshal(map[string]any{"url": local.URL, "ids": ids, "playwrightModule": playwright, "executablePath": os.Getenv("GLADE_LWC_BROWSER_EXECUTABLE")})
	if err != nil {
		return nil, nil, nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
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
		if !known[id] || observed.Errors[id] != "" {
			return nil, nil, nil, fmt.Errorf("unexpected or conflicting local L09 row %s", id)
		}
	}
	for id := range observed.Errors {
		if !known[id] {
			return nil, nil, nil, fmt.Errorf("unexpected local L09 error row %s", id)
		}
	}
	return observed.Values, observed.Errors, nil, nil
}

func l09SeedNativeModifier(t *testing.T, org *storage.OrgState, api string, cases []l09Case) {
	t.Helper()
	for _, c := range cases {
		if c.ID != "ctrl_conditionalUpdate_modifierContext" {
			continue
		}
		var answer struct {
			Returned struct {
				LastModifiedBy string `json:"lastModifiedBy"`
			} `json:"returned"`
		}
		if err := json.Unmarshal([]byte(strings.TrimPrefix(c.Expected[api], "JSON|")), &answer); err != nil || answer.Returned.LastModifiedBy == "" {
			t.Fatalf("invalid native modifier context API %s", api)
		}
		// Map the captured execution user's name onto the local fixture user.
		// Product collision messages still resolve the stored modifier record.
		user := org.Objects["User"]
		record, ok := user.Records[storage.ID("005000000000001")]
		if !ok {
			t.Fatal("missing local execution user")
		}
		record.Fields["Name"] = storage.StringValue(answer.Returned.LastModifiedBy)
		user.Records[record.ID] = record
		org.Objects["User"] = user
		return
	}
	t.Fatal("missing captured native modifier context")
}
