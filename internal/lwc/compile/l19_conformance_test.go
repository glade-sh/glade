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

type l19Case struct {
	ID                 string            `json:"id"`
	Group              string            `json:"group"`
	Kind               string            `json:"kind"`
	Basis              string            `json:"basis"`
	JS                 string            `json:"js"`
	Template           string            `json:"template"`
	MetaFragment       string            `json:"meta_fragment"`
	Spec               json.RawMessage   `json:"spec"`
	Observer           map[string]string `json:"observer"`
	Expected           map[string]string `json:"expected"`
	DiagnosticExpected map[string]string `json:"diagnostic_expected"`
	SourceRow          string            `json:"source_row"`
	RuntimeProject     map[string]string `json:"runtime_project"`
	CachePhase         map[string]string `json:"cache_phase"`
	Projection         string            `json:"projection"`
	Owner              string            `json:"owner"`
	Reason             string            `json:"reason"`
}

type l19Removal struct {
	ID     string   `json:"id"`
	APIs   []string `json:"apis"`
	Reason string   `json:"reason"`
}

type l19FixturePicklist struct {
	Object    string                  `json:"object"`
	Field     string                  `json:"field"`
	SourceRow string                  `json:"source_row"`
	Values    []storage.PicklistValue `json:"values"`
}

var l19DiagnosticURL = regexp.MustCompile(`(?i)https?://[^\s"'<>]+`)

func l19InteractionText(t *testing.T, text string) string {
	t.Helper()
	var row struct {
		Observation map[string]json.RawMessage `json:"observation"`
	}
	if !strings.HasPrefix(text, "JSON|") || json.Unmarshal([]byte(strings.TrimPrefix(text, "JSON|")), &row) != nil || row.Observation == nil {
		t.Fatal("invalid native record forms interaction row")
	}
	var trace []map[string]json.RawMessage
	if err := json.Unmarshal(row.Observation["eventTrace"], &trace); err != nil {
		t.Fatal(err)
	}
	for _, event := range trace {
		// Only hosted scheduling fields are outside the semantic projection.
		delete(event, "stage")
		delete(event, "elapsedMs")
	}
	details, err := json.Marshal(trace)
	if err != nil {
		t.Fatal(err)
	}
	row.Observation["eventDetails"] = details
	delete(row.Observation, "eventTrace")
	delete(row.Observation, "sourceApi")
	value, err := json.Marshal(row.Observation)
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := l07BrowserText(value)
	if err != nil {
		t.Fatal(err)
	}
	return "JSON|" + strings.TrimPrefix(canonical, "BROWSER|")
}

// TestL19SalesforceConformance replays the owned native fixture through the
// compiler, browser runtime and product LDS/REST server using owned exports.
// Excluding Aura-only observations and malformed injected-picker interactions,
// the retained main denominator is 193/API; every exclusion has a reason.
// Fifty-six fresh/remount event/interaction controls/API are reported separately. The
// original 104/430 before and superseded observations stay in export provenance.
// CI uses only credential-free owned exports.
// GLADE_L19_CAPTURE=1 reports exact-text differences without failing them;
// GLADE_L19_REPORT selects the per-row TSV, including the before match count.
func TestL19SalesforceConformance(t *testing.T) {
	t.Run("BrowserRuntime", l19SalesforceConformance)
	t.Run("BrowserFixtures", l19BrowserFixtures)
}

func l19SalesforceConformance(t *testing.T) {
	capture := os.Getenv("GLADE_L19_CAPTURE") == "1"
	data, err := os.ReadFile("testdata/l19_salesforce.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases               []l19Case                    `json:"cases"`
		Controls            []l19Case                    `json:"controls"`
		Removals            []l19Removal                 `json:"removals"`
		VersionControls     map[string]map[string]string `json:"version_controls"`
		VersionInteractions map[string]map[string]string `json:"version_interactions"`
		FixturePicklists    []l19FixturePicklist         `json:"fixture_picklists"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	versions := []string{"59.0", "67.0"}
	removed := map[string]map[string]string{}
	for _, row := range fixture.Removals {
		if row.ID == "" || len(row.APIs) == 0 || row.Reason == "" || strings.ContainsAny(row.Reason, "\r\n\t") {
			t.Fatalf("invalid record forms removal %q", row.ID)
		}
		for _, api := range row.APIs {
			if api != "59.0" && api != "67.0" {
				t.Fatalf("invalid record forms removal API %s", api)
			}
			if removed[api] == nil {
				removed[api] = map[string]string{}
			}
			if removed[api][row.ID] != "" {
				t.Fatalf("duplicate record forms removal %s API %s", row.ID, api)
			}
			removed[api][row.ID] = row.Reason
		}
	}
	seen, groups := map[string]bool{}, map[string]bool{}
	var compileCases []l19Case
	runtimeCases := map[string][]l19Case{}
	apiTotals := map[string]int{}
	controlAPITotals := map[string]int{}
	diagnosticRows := 0
	allCases := append(append([]l19Case(nil), fixture.Cases...), fixture.Controls...)
	for _, c := range allCases {
		if c.ID == "" || seen[c.ID] || c.Basis != "org" || len(c.Expected) == 0 || len(c.Expected) > len(versions) {
			t.Fatalf("invalid or duplicate record forms row %q", c.ID)
		}
		seen[c.ID], groups[c.Group] = true, true
		if c.Owner != "" || c.Reason != "" {
			t.Fatalf("record forms row %s retains a carry to a completed dependency", c.ID)
		}
		switch c.Kind {
		case "compile":
			if c.JS == "" || c.Template == "" || len(c.Expected) != len(versions) {
				t.Fatalf("missing record forms compile input %s", c.ID)
			}
			compileCases = append(compileCases, c)
		case "runtime":
			var spec struct {
				ID string `json:"id"`
			}
			sourceID := c.ID
			if c.SourceRow != "" {
				sourceID = c.SourceRow
			}
			if err := json.Unmarshal(c.Spec, &spec); err != nil || spec.ID != sourceID || len(c.Observer) != len(c.Expected) || len(c.DiagnosticExpected) != 0 ||
				(c.Projection != "" && c.Projection != "event-details" && c.Projection != "interaction") {
				t.Fatalf("invalid record forms runtime spec %s", c.ID)
			}
		default:
			t.Fatalf("invalid record forms kind %s: %s", c.ID, c.Kind)
		}
		for _, api := range versions {
			want, ok := c.Expected[api]
			if !ok {
				if c.Kind != "runtime" || removed[api][c.ID] == "" {
					t.Fatalf("missing native record forms answer without removal %s API %s", c.ID, api)
				}
				continue
			}
			if removed[api][c.ID] != "" || (c.Kind == "compile" && want != "COMPILE_OK" && want != "COMPILE_ERROR") ||
				(c.Kind == "runtime" && (!strings.HasPrefix(want, "JSON|") || !json.Valid([]byte(strings.TrimPrefix(want, "JSON|"))))) {
				t.Fatalf("invalid native record forms answer %s API %s", c.ID, api)
			}
			if (want == "COMPILE_ERROR") != (c.DiagnosticExpected[api] != "") {
				t.Fatalf("missing or unexpected record forms diagnostic %s API %s", c.ID, api)
			}
			if c.Kind == "runtime" {
				if c.Observer[api] != "dom" && c.Observer[api] != "inspector" {
					t.Fatalf("invalid record forms observer %s API %s", c.ID, api)
				}
				if (c.RuntimeProject[api] != "" && c.RuntimeProject[api] != "controls" && c.RuntimeProject[api] != "interactions") ||
					(c.CachePhase[api] != "" && c.CachePhase[api] != "fresh" && c.CachePhase[api] != "same-page-remount") {
					t.Fatalf("invalid record forms source/cache precondition %s API %s", c.ID, api)
				}
				if c.Projection == "event-details" && (c.SourceRow == "" || c.RuntimeProject[api] != "controls" ||
					c.CachePhase[api] == "" || fixture.VersionControls[api][strings.TrimPrefix(c.ID, "ctrl_")] != want) {
					t.Fatalf("missing native record forms cache-control source %s API %s", c.ID, api)
				}
				if c.Projection == "interaction" && (c.SourceRow == "" || c.RuntimeProject[api] != "interactions" || c.CachePhase[api] == "" ||
					l19InteractionText(t, fixture.VersionInteractions[api][strings.TrimPrefix(c.ID, "ctrl_")]) != want) {
					t.Fatalf("missing native record forms interaction source %s API %s", c.ID, api)
				}
				runtimeCases[api] = append(runtimeCases[api], c)
			}
			if c.Projection != "" {
				controlAPITotals[api]++
			} else {
				apiTotals[api]++
			}
			if c.DiagnosticExpected[api] != "" {
				diagnosticRows++
			}
		}
	}
	if len(fixture.Cases) != 193 || len(fixture.Controls) != 56 || len(compileCases) != 52 ||
		apiTotals["59.0"] != 193 || apiTotals["67.0"] != 193 || controlAPITotals["59.0"] != 56 || controlAPITotals["67.0"] != 56 ||
		diagnosticRows != 48 || len(removed["59.0"]) != 34 || len(removed["67.0"]) != 34 {
		t.Fatalf("record forms native denominator changed: cases=%d compile=%d APIs=%v diagnostics=%d removals=%d/%d", len(fixture.Cases), len(compileCases), apiTotals, diagnosticRows, len(removed["59.0"]), len(removed["67.0"]))
	}
	if len(fixture.FixturePicklists) != 1 {
		t.Fatal("missing native record forms picklist fixture input")
	}
	for _, input := range fixture.FixturePicklists {
		if input.Object == "" || input.Field == "" || len(input.Values) == 0 || !seen[input.SourceRow] {
			t.Fatal("invalid native record forms picklist fixture input")
		}
		for _, api := range versions {
			for _, c := range fixture.Controls {
				if c.ID != input.SourceRow {
					continue
				}
				var native struct {
					Action struct {
						Options []struct{ Value, Text string } `json:"options"`
					} `json:"action"`
				}
				if err := json.Unmarshal([]byte(strings.TrimPrefix(c.Expected[api], "JSON|")), &native); err != nil {
					t.Fatal(err)
				}
				var choices []storage.PicklistValue
				for _, option := range native.Action.Options {
					if option.Value != "" {
						choices = append(choices, storage.PicklistValue{Value: option.Value, Label: option.Text})
					}
				}
				actual, _ := json.Marshal(input.Values)
				want, _ := json.Marshal(choices)
				if !bytes.Equal(actual, want) {
					t.Fatalf("record forms picklist fixture differs from native %s API %s", c.ID, api)
				}
			}
		}
	}
	if len(fixture.VersionControls) != 9 || len(fixture.VersionInteractions) != 9 {
		t.Fatal("missing record forms intermediate source API captures")
	}
	for version := 59; version <= 67; version++ {
		api := fmt.Sprintf("%d.0", version)
		if len(fixture.VersionControls[api]) != 38 {
			t.Fatalf("missing record forms native cache controls at API %s", api)
		}
		if len(fixture.VersionInteractions[api]) != 18 {
			t.Fatalf("missing record forms native interaction controls at API %s", api)
		}
		for _, c := range fixture.Controls {
			if c.Projection == "interaction" {
				text := l19InteractionText(t, fixture.VersionInteractions[api][strings.TrimPrefix(c.ID, "ctrl_")])
				if text != c.Expected["59.0"] {
					t.Fatalf("record forms interaction version boundary differs at API %s: %s", api, c.ID)
				}
				continue
			}
			text := fixture.VersionControls[api][strings.TrimPrefix(c.ID, "ctrl_")]
			if !strings.HasPrefix(text, "JSON|") || !json.Valid([]byte(strings.TrimPrefix(text, "JSON|"))) {
				t.Fatalf("invalid record forms native control %s API %s", c.ID, api)
			}
		}
	}
	for _, group := range []string{"form modes", "field rendering", "submit/error flows", "record picker"} {
		if !groups[group] {
			t.Fatalf("missing record forms group %s", group)
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
			t.Fatal(err) // Infrastructure failures cannot count as native rejections.
		}
	}
	var report strings.Builder
	fmt.Fprintln(&report, "api\tid\tkind\tstatus\tactual\texpected\treason\tdiagnostic_status\tdiagnostic_actual\tdiagnostic_expected")
	matches, total, compileMatches, domMatches, localDOMRows := 0, 0, 0, 0, 0
	diagnosticMatches, diagnosticTotal := 0, 0
	controlMatches, controlTotal, localControlRows := 0, 0, 0
	writeReport := func() {
		if path := os.Getenv("GLADE_L19_REPORT"); path != "" {
			l07Write(t, path, []byte(report.String()))
		}
	}
	for _, api := range versions {
		compiled := l19CompileCases(t, api, compileCases)
		dom, domErrors, runtimeCompileErr, browserErr := l19ObserveDOM(t, api, runtimeCases[api], dependencyRoot, fixture.FixturePicklists)
		if browserErr != nil {
			writeReport()
			t.Fatalf("record forms browser environment: %v", browserErr)
		}
		apiMatches, apiDOMRows, apiControlMatches, apiControlRows := 0, 0, 0, 0
		for _, c := range allCases {
			want, applicable := c.Expected[api]
			if !applicable {
				continue // No native observation exists for this removed API/row pair.
			}
			got, reason := "", ""
			diagnosticWant, diagnosticGot, diagnosticStatus := c.DiagnosticExpected[api], "", ""
			if c.Kind == "compile" {
				result := compiled[c.ID]
				got = "COMPILE_OK"
				if result.Err != nil {
					got = "COMPILE_ERROR"
				}
				if diagnosticWant != "" {
					diagnosticGot = "MISSING_DIAGNOSTIC"
					if len(result.ReportedDiagnostics) == 1 {
						diagnosticGot = result.ReportedDiagnostics[0]
					} else if len(result.ReportedDiagnostics) > 1 {
						encoded, err := json.Marshal(result.ReportedDiagnostics)
						if err != nil {
							t.Fatal(err)
						}
						diagnosticGot = string(encoded)
					} else if len(result.Diagnostics) == 1 {
						diagnosticGot = result.Diagnostics[0].Message
					} else if len(result.Diagnostics) > 1 {
						encoded, err := json.Marshal(result.Diagnostics)
						if err != nil {
							t.Fatal(err)
						}
						diagnosticGot = string(encoded)
					} else if result.Err != nil {
						cause := result.Err
						for errors.Unwrap(cause) != nil {
							cause = errors.Unwrap(cause)
						}
						diagnosticGot = cause.Error()
					}
					// The native capture exports public diagnostic URLs as <URL>.
					// Keep its projection, including every position and message byte.
					diagnosticGot = l19DiagnosticURL.ReplaceAllString(diagnosticGot, "<URL>")
					diagnosticTotal++
					diagnosticStatus = "MISMATCH"
					if diagnosticGot == diagnosticWant {
						diagnosticStatus = "MATCH"
						diagnosticMatches++
					} else if !capture {
						t.Errorf("%s API %s diagnostic expected <%s> actual <%s>", c.ID, api, diagnosticWant, diagnosticGot)
					}
				}
				if got == want {
					compileMatches++
				} else {
					reason = "record forms: local compile outcome differs from native"
					if result.Err != nil {
						reason += ": " + result.Err.Error()
					}
				}
			} else if raw, ok := dom[c.ID]; ok {
				// Shared wire serializer preserves every field and raw null.
				text, err := l07BrowserText(raw)
				if err != nil {
					t.Fatal(err)
				}
				got = "JSON|" + strings.TrimPrefix(text, "BROWSER|")
				if c.Projection != "" {
					localControlRows++
					apiControlRows++
				} else {
					localDOMRows++
					apiDOMRows++
					if got == want {
						domMatches++
					}
				}
				if got != want {
					reason = "record forms: record form/field/picker DOM observation differs from native"
				}
			} else {
				got, reason = "BROWSER_ERROR", "record forms: local DOM observation missing"
				if runtimeCompileErr != nil {
					got = "BROWSER_COMPILE_ERROR"
					reason += ": " + runtimeCompileErr.Error()
				} else if domErrors[c.ID] != "" {
					reason += ": " + domErrors[c.ID]
				}
			}
			status := "MISMATCH"
			if got == want {
				status = "MATCH"
				if c.Projection != "" {
					controlMatches++
					apiControlMatches++
				} else {
					matches++
					apiMatches++
				}
			} else if !capture {
				t.Errorf("%s API %s expected <%s> actual <%s>: %s", c.ID, api, want, got, l07OneLine(reason))
			}
			if c.Projection != "" {
				controlTotal++
			} else {
				total++
			}
			fmt.Fprintf(&report, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", api, c.ID, c.Kind, status, l07OneLine(got), l07OneLine(want), l07OneLine(reason), diagnosticStatus, l07OneLine(diagnosticGot), l07OneLine(diagnosticWant))
		}
		mainDOMRows := len(runtimeCases[api]) - controlAPITotals[api]
		fmt.Fprintf(&report, "API_TOTAL\t%s\t%d/%d\nLOCAL_DOM_ROWS\t%s\t%d/%d\nAPI_CONTROL_TOTAL\t%s\t%d/%d\n", api, apiMatches, apiTotals[api], api, apiDOMRows, mainDOMRows, api, apiControlMatches, controlAPITotals[api])
		t.Logf("record forms API %s matches %d/%d; observed local DOM %d/%d; controls %d/%d, observed %d", api, apiMatches, apiTotals[api], apiDOMRows, mainDOMRows, apiControlMatches, controlAPITotals[api], apiControlRows)
		writeReport()
	}
	domTotal := len(runtimeCases["59.0"]) + len(runtimeCases["67.0"]) - controlTotal
	fmt.Fprintf(&report, "TOTAL\t%d/%d\nCOMPILE_TOTAL\t%d/%d\nDOM_TOTAL\t%d/%d\nLOCAL_DOM_TOTAL\t%d/%d\nDIAGNOSTIC_TOTAL\t%d/%d\n", matches, total, compileMatches, len(compileCases)*len(versions), domMatches, domTotal, localDOMRows, domTotal, diagnosticMatches, diagnosticTotal)
	t.Logf("record forms matches %d/%d; compile %d/104; DOM %d/%d; observed local DOM %d/%d; diagnostics %d/%d", matches, total, compileMatches, domMatches, domTotal, localDOMRows, domTotal, diagnosticMatches, diagnosticTotal)
	fmt.Fprintf(&report, "CONTROL_TOTAL\t%d/%d\nLOCAL_CONTROL_TOTAL\t%d/%d\n", controlMatches, controlTotal, localControlRows, controlTotal)
	t.Logf("record forms native cache/interaction controls %d/%d; observed %d/%d", controlMatches, controlTotal, localControlRows, controlTotal)
	writeReport()
}

func l19CompileCases(t *testing.T, api string, cases []l19Case) map[string]compile.BundleResult {
	t.Helper()
	root := t.TempDir()
	keys := map[string]string{}
	for index, c := range cases {
		name := fmt.Sprintf("familyL19%03d", index)
		rel := filepath.Join("cases", fmt.Sprintf("%04d", index), "force-app", "main", "default", "lwc", name)
		keys[c.ID] = filepath.ToSlash(rel)
		bundle := filepath.Join(root, rel)
		l07Write(t, filepath.Join(bundle, name+".js"), []byte(c.JS))
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
			t.Fatalf("missing record forms compile row %s", c.ID)
		}
		l07CheckCompilerEnvironment(t, result.Err)
		results[c.ID] = result
	}
	return results
}

func l19ObserveDOM(t *testing.T, api string, cases []l19Case, dependencyRoot string, picklists []l19FixturePicklist) (map[string]json.RawMessage, map[string]string, error, error) {
	t.Helper()
	values, failures := map[string]json.RawMessage{}, map[string]string{}
	var compileErr error
	for _, runtimeProject := range []string{"", "controls", "interactions"} {
		var selected []l19Case
		for _, c := range cases {
			if c.RuntimeProject[api] == runtimeProject {
				selected = append(selected, c)
			}
		}
		if len(selected) == 0 {
			continue
		}
		observed, rowErrors, projectCompileErr, browserErr := l19ObserveProjectDOM(t, api, selected, dependencyRoot, runtimeProject, picklists)
		if browserErr != nil {
			return nil, nil, nil, browserErr
		}
		compileErr = errors.Join(compileErr, projectCompileErr)
		for id, value := range observed {
			values[id] = value
		}
		for id, reason := range rowErrors {
			failures[id] = reason
		}
	}
	return values, failures, compileErr, nil
}

func l19ObserveProjectDOM(t *testing.T, api string, cases []l19Case, dependencyRoot, runtimeProject string, picklists []l19FixturePicklist) (map[string]json.RawMessage, map[string]string, error, error) {
	t.Helper()
	root := t.TempDir()
	fixture, page := "l19_runtime", "familyL19Runtime"+api[:2]
	if runtimeProject == "controls" {
		fixture, page = "l19_load_runtime", "familyL19ControlLoads"
	}
	if runtimeProject == "interactions" {
		fixture, page = "l19_interaction_runtime", "familyL19Controlf9db75e5"
	}
	l07Copy(t, filepath.Join("testdata", fixture, "api"+api[:2]), root)
	p, err := project.Load(root)
	if err != nil {
		return nil, nil, nil, err
	}
	manifest, err := compile.Compile(p, compile.Options{OutDir: filepath.Join(root, "dist")})
	l07CheckCompilerEnvironment(t, err)
	if err != nil {
		return nil, nil, err, nil
	}
	if _, ok := manifest.Modules["c:"+page]; !ok {
		return nil, nil, fmt.Errorf("runtime entry %s missing", page), nil
	}
	localManifest := lwcbrowser.Manifest{Modules: map[string]lwcbrowser.ModuleEntry{}}
	for qualified, module := range manifest.Modules {
		rel, err := filepath.Rel(manifest.OutDir, module.File)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, nil, nil, fmt.Errorf("record forms module escapes output: %s", module.File)
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
	for _, input := range picklists {
		object, ok := org.Objects[input.Object]
		if !ok {
			t.Fatalf("record forms fixture object not found: %s", input.Object)
		}
		field, ok := object.Definition.Fields[input.Field]
		if !ok {
			t.Fatalf("record forms fixture field not found: %s.%s", input.Object, input.Field)
		}
		field.PicklistValues = append([]storage.PicklistValue(nil), input.Values...)
		for index := range field.PicklistValues {
			field.PicklistValues[index].Active = true // Captured visible, selectable choices.
		}
		object.Definition.Fields[input.Field] = field
		org.Objects[input.Object] = object
	}
	org.Metadata = registry
	product := server.NewWithSource(&org, source)
	product.SetProjectIndex(typesys.Build(p, schema))
	mux := http.NewServeMux()
	mux.Handle("/lightning/", product)
	mux.Handle("/services/data/", product) // Owned fixture DML uses product REST, not mocks.
	mux.Handle("/lightning/modules/", http.StripPrefix("/lightning/modules/", http.FileServer(http.Dir(manifest.OutDir))))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><script>window.__l19Error=null;</script><script id="glade-lwc-context" type="application/json">{}</script><main id="l19-host"></main>%s<script>window.$Lightning.createComponent(%q,{},"l19-host",(_el,status,message)=>{if(status!=="SUCCESS")window.__l19Error={name:"Error",message};});</script>`, bootstrap, "c:"+page)
	})
	local := httptest.NewServer(mux)
	defer local.Close()
	playwright := os.Getenv("GLADE_LWC_PLAYWRIGHT_MODULE")
	if playwright == "" {
		playwright = filepath.Join(dependencyRoot, "lwcruntime", "node_modules", "playwright")
	}
	if _, err := os.Stat(filepath.Join(playwright, "package.json")); err != nil {
		return nil, nil, nil, err
	}
	observer, err := filepath.Abs("testdata/l19_browser.mjs")
	if err != nil {
		return nil, nil, nil, err
	}
	inputs := make([]map[string]any, len(cases))
	known := map[string]bool{}
	for index, c := range cases {
		inputs[index] = map[string]any{"id": c.ID, "spec": c.Spec, "observer": c.Observer[api], "cachePhase": c.CachePhase[api], "projection": c.Projection}
		known[c.ID] = true
	}
	config, err := json.Marshal(map[string]any{
		"url": local.URL, "api": api, "cases": inputs,
		// Fixture DML uses supported REST API 67 independently of the captured
		// LWC source API (59.0 or 67.0) compiled above.
		"fixtureApi":       "67.0",
		"playwrightModule": playwright,
		"executablePath":   os.Getenv("GLADE_LWC_BROWSER_EXECUTABLE"),
		"controlProject":   runtimeProject != "",
	})
	if err != nil {
		return nil, nil, nil, err
	}
	deadline := time.Now().Add(45 * time.Minute)
	if testDeadline, ok := t.Deadline(); ok && testDeadline.Before(deadline) {
		// Give Playwright time to close Chromium before the package alarm.
		deadline = testDeadline.Add(-15 * time.Second)
	}
	ctx, cancel := context.WithDeadline(t.Context(), deadline)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", observer)
	cmd.Cancel = func() error { return cmd.Process.Signal(os.Interrupt) }
	cmd.WaitDelay = 5 * time.Second
	cmd.Stdin = bytes.NewReader(config)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("local observer: %w: %s", err, l07OneLine(stderr.String()))
	}
	var observed struct {
		Values           map[string]json.RawMessage `json:"values"`
		Errors           map[string]string          `json:"errors"`
		EnvironmentError string                     `json:"environmentError"`
	}
	if err := json.Unmarshal(output, &observed); err != nil {
		return nil, nil, nil, err
	}
	if observed.EnvironmentError != "" {
		return nil, nil, nil, errors.New(observed.EnvironmentError)
	}
	for id := range observed.Values {
		if !known[id] || observed.Errors[id] != "" {
			return nil, nil, nil, fmt.Errorf("unexpected or conflicting local record forms row %s", id)
		}
	}
	for id := range observed.Errors {
		if !known[id] {
			return nil, nil, nil, fmt.Errorf("unexpected local record forms error row %s", id)
		}
	}
	return observed.Values, observed.Errors, nil, nil
}

// Exercise the existing browser fixtures whose module dependencies record forms changed.
func l19BrowserFixtures(t *testing.T) {
	t.Helper()
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", "--test", "--test-concurrency=1", "--test-reporter=tap",
		"lwcruntime/test/base-components.test.mjs",
		"lwcruntime/test/base-components-expanded.test.mjs",
		"lwcruntime/test/record-picker.test.mjs")
	cmd.Dir = root
	cmd.Cancel = func() error { return cmd.Process.Signal(os.Interrupt) }
	cmd.WaitDelay = 5 * time.Second
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("record forms browser fixtures: %v\n%s", err, output)
	}
	if !regexp.MustCompile(`(?m)^# skipped 0$`).Match(output) {
		t.Fatalf("record forms browser fixtures did not run without skips:\n%s", output)
	}
	t.Logf("record forms browser fixtures:\n%s", output)
}
