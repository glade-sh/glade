package compile_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/gladehome"
	"github.com/glade-sh/glade/internal/lwc/compile"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/resource"
	"github.com/glade-sh/glade/internal/server"
	"github.com/glade-sh/glade/internal/storage"
)

type l17ConsoleExclusion struct {
	Entry    json.RawMessage `json:"entry"`
	Reason   string          `json:"reason"`
	Evidence json.RawMessage `json:"evidence"`
}

type l17Case struct {
	NativeBundle       string            `json:"native_bundle"`
	Excluded           string            `json:"excluded"`
	ID                 string            `json:"id"`
	Kind               string            `json:"kind"`
	Group              string            `json:"group"`
	JS                 string            `json:"js"`
	Template           string            `json:"template"`
	CSS                *string           `json:"css"`
	MetaFragment       string            `json:"meta_fragment"`
	Method             string            `json:"method"`
	Expected           map[string]string `json:"expected"`
	DiagnosticExpected map[string]string `json:"diagnostic_expected"`
}

// TestL17SalesforceConformance replays the complete owned native API 59/67
// oracle and isolated resource-reference controls. One hook
// effect is unobservable; every captured DOM field is still asserted, with
// source-qualified host console exclusions only. Capture mode measures the
// product tree without turning behavioral mismatches into failures.
func TestL17SalesforceConformance(t *testing.T) {
	capture := os.Getenv("GLADE_L17_CAPTURE") != ""
	data, err := os.ReadFile("testdata/l17_salesforce.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []l17Case
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	seen, groups := map[string]bool{}, map[string]bool{}
	var compilerCases, runtimeCases []l17Case
	for _, c := range cases {
		if c.ID == "" || seen[c.ID] || c.JS == "" || c.Template == "" {
			t.Fatalf("invalid or duplicate L17 input %q", c.ID)
		}
		seen[c.ID], groups[c.Group] = true, true
		for _, api := range []string{"59.0", "67.0"} {
			want, ok := c.Expected[api]
			if !ok || (c.Kind == "compile" && want != "COMPILE_OK" && want != "COMPILE_ERROR") || (c.Kind == "runtime" && !strings.HasPrefix(want, "DOM|")) {
				t.Fatalf("missing native %s answer: %s API %s", c.Kind, c.ID, api)
			}
			if (want == "COMPILE_ERROR") != (c.DiagnosticExpected[api] != "") {
				t.Fatalf("native rejection/diagnostic disagreement: %s API %s", c.ID, api)
			}
			if c.Kind == "runtime" {
				var native struct {
					ID string `json:"id"`
				}
				if err := json.Unmarshal([]byte(strings.TrimPrefix(want, "DOM|")), &native); err != nil || native.ID != c.ID {
					t.Fatalf("invalid native DOM answer: %s API %s", c.ID, api)
				}
			}
		}
		switch c.Kind {
		case "compile":
			compilerCases = append(compilerCases, c)
		case "runtime":
			if c.Method != "loadScript" && c.Method != "loadStyle" && c.Method != "styles" {
				t.Fatalf("invalid L17 browser method %s", c.ID)
			}
			runtimeCases = append(runtimeCases, c)
		default:
			t.Fatalf("invalid L17 row kind %s", c.Kind)
		}
	}
	if len(cases) != 214 || len(compilerCases) != 138 || len(runtimeCases) != 76 {
		t.Fatalf("L17 requires 138 compile/metadata and 76 DOM rows, got %d/%d", len(compilerCases), len(runtimeCases))
	}
	for _, group := range []string{"repeat loads", "failures", "subpaths", "styles"} {
		if !groups[group] {
			t.Fatalf("missing L17 group %s", group)
		}
	}
	consoleExclusions := l17LoadConsoleExclusions(t, cases)
	dependencies, err := gladehome.EnsureRoot()
	if err != nil {
		t.Fatal(err)
	}
	// Toolchain failures must never count as native compile rejections.
	if _, err := exec.LookPath("node"); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{
		"third_party/lwc/node_modules/@lwc/compiler/package.json",
		"third_party/lwc/node_modules/@lwc/engine-dom/dist/index.js",
		"third_party/lwc/node_modules/@lwc/synthetic-shadow/dist/index.js",
	} {
		if _, err := os.Stat(filepath.Join(dependencies, filepath.FromSlash(file))); err != nil {
			t.Fatal(err)
		}
	}
	var report strings.Builder
	report.WriteString("API\tID\tKIND\tSTATUS\tACTUAL\tEXPECTED\tREASON\n")
	matches, total, compileMatches, domMatches, observedDOM := 0, 0, 0, 0, 0
	excluded := 0
	hostConsoleExcluded := 0
	diagnosticMatches, diagnosticTotal := 0, 0
	compileOutcomeMatches := 0
	l17ResourceControls(t, &report, capture)
	for _, api := range []string{"59.0", "67.0"} {
		p, names := l17Project(t, api, compilerCases, false)
		compiled, err := compile.CompileBatch(p, compile.Options{OutDir: filepath.Join(p.Root, "dist"), Namespace: "c"})
		if err != nil {
			t.Fatal(err)
		}
		dom, domErrors, domErr := l17ObserveDOM(t, api, runtimeCases, dependencies)
		compileIndex, apiMatches := 0, 0
		for _, c := range cases {
			if c.Excluded != "" {
				if c.ID != "dom_style_base_button_hook" || c.Kind != "runtime" {
					t.Fatalf("unexpected L17 exclusion %s", c.ID)
				}
				// The intended background hook effect cannot be observed in
				// native. Its other captured fields still follow the same
				// strict comparison as every other row.
			}
			got, want, reason := "", c.Expected[api], ""
			if c.Kind == "compile" {
				key := "force-app/main/default/lwc/" + names[compileIndex]
				compileIndex++
				result, ok := compiled[key]
				if !ok {
					t.Fatalf("missing L17 compiler result %s", key)
				}
				got = "COMPILE_OK"
				if result.Err != nil {
					got, reason = "COMPILE_ERROR", result.Err.Error()
				}
				if got != want {
					reason = "L17: compiler acceptance differs; " + reason
				} else {
					compileOutcomeMatches++
				}
				if diagnosticWant := c.DiagnosticExpected[api]; diagnosticWant != "" {
					diagnosticGot := l17Diagnostic(result.Err)
					diagnosticStatus := "MISMATCH"
					diagnosticTotal++
					if diagnosticGot == diagnosticWant {
						diagnosticStatus = "MATCH"
						diagnosticMatches++
					} else if !capture {
						t.Errorf("%s API %s diagnostic expected <%s> actual <%s>", c.ID, api, diagnosticWant, diagnosticGot)
					}
					diagnosticReason := ""
					if diagnosticStatus != "MATCH" {
						diagnosticReason = "L17: compiler deployment diagnostic differs"
					}
					fmt.Fprintf(&report, "%s\t%s\tdiagnostic\t%s\t%s\t%s\t%s\n", api, c.ID, diagnosticStatus, l13Cell(diagnosticGot), l13Cell(diagnosticWant), diagnosticReason)
					// Rejection text is part of the row's exact match, not just
					// an extra assertion outside the reported case denominator.
					got += "|" + diagnosticGot
					want += "|" + diagnosticWant
					if diagnosticStatus != "MATCH" {
						reason = diagnosticReason
					}
				}
			} else if raw, ok := dom[c.ID]; ok {
				// Canonicalize serialization only: preserve nulls, all fields,
				// array order, case and diagnostic text. The report stays raw;
				// qualified host console exclusions apply only to comparison.
				got = "DOM|" + l13CanonicalJSON(t, string(raw))
				observedDOM++
			} else {
				got, reason = "BROWSER_ERROR", domErrors[c.ID]
				if reason == "" && domErr != nil {
					reason = domErr.Error()
				}
			}
			comparisonWant, consoleReason := want, ""
			if c.Kind == "runtime" && strings.HasPrefix(got, "DOM|") && got != want {
				entries := consoleExclusions[api][c.ID]
				comparisonWant = l17WithoutHostConsole(t, want, entries)
				for _, entry := range entries {
					hostConsoleExcluded++
					t.Logf("L17 native host console excluded %s API %s: %s", c.ID, api, entry.Reason)
					fmt.Fprintf(&report, "%s\t%s\tconsole-host\tEXCLUDED\t\t%s\t%s\n", api, c.ID, l13Cell(string(entry.Entry)), l13Cell(entry.Reason))
					consoleReason += "; native host console excluded: " + entry.Reason
				}
				if got != comparisonWant {
					reason = l17DOMReason(t, strings.TrimPrefix(comparisonWant, "DOM|"), strings.TrimPrefix(got, "DOM|"))
				}
				reason += consoleReason
			}
			status := "MISMATCH"
			if got == want {
				status = "MATCH"
				if c.Excluded == "" {
					matches++
					apiMatches++
					if c.Kind == "compile" {
						compileMatches++
					} else {
						domMatches++
					}
				}
			}
			if got != want {
				if got == comparisonWant {
					status = "HOST_CONSOLE_EXCLUDED"
				}
			}
			reportedStatus := status
			if c.Excluded == "" {
				total++
			} else {
				excluded++
				t.Logf("L17 unobservable effect %s API %s: %s; captured fields %s", c.ID, api, c.Excluded, status)
				reason = c.Excluded + "; captured fields " + status + ": " + reason
				if status != "MISMATCH" {
					reportedStatus = "EXCLUDED"
				}
			}
			fmt.Fprintf(&report, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", api, c.ID, c.Kind, reportedStatus, l13Cell(got), l13Cell(want), l13Cell(reason))
			if !capture && got != comparisonWant {
				t.Errorf("%s API %s expected <%s> actual <%s>: %s", c.ID, api, want, got, reason)
			}
		}
		fmt.Fprintf(&report, "API_TOTAL\t%s\t%d/213\n", api, apiMatches)
		t.Logf("L17 API %s matches %d/213", api, apiMatches)
		if domErr != nil {
			t.Logf("L17 API %s browser: %v", api, domErr)
		}
	}
	fmt.Fprintf(&report, "TOTAL\t%d/%d\nCOMPILE_TOTAL\t%d/276\nDOM_TOTAL\t%d/150\nLOCAL_DOM_ROWS\t%d/152\n", matches, total, compileMatches, domMatches, observedDOM)
	fmt.Fprintf(&report, "CARRIED_TOTAL\t0\nEXCLUDED_TOTAL\t%d\n", excluded)
	fmt.Fprintf(&report, "HOST_CONSOLE_EXCLUDED_TOTAL\t%d\n", hostConsoleExcluded)
	fmt.Fprintf(&report, "COMPILE_OUTCOME_TOTAL\t%d/276\n", compileOutcomeMatches)
	fmt.Fprintf(&report, "DIAGNOSTIC_TOTAL\t%d/%d\n", diagnosticMatches, diagnosticTotal)
	t.Logf("L17 matches %d/%d; compile/metadata %d/276; native-vs-local DOM %d/150; observed DOM %d/152", matches, total, compileMatches, domMatches, observedDOM)
	t.Logf("L17 carried 0; excluded %d; diagnostics %d/%d", excluded, diagnosticMatches, diagnosticTotal)
	t.Logf("L17 unobservable effect rows %d; all captured fields compared strictly", excluded)
	t.Logf("L17 native host console entries excluded %d; raw native rows and full-row match counts retained", hostConsoleExcluded)
	if path := os.Getenv("GLADE_L17_REPORT"); path != "" {
		if err := os.WriteFile(path, []byte(report.String()), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

// Only native warning entries with captured host-only source stacks can be
// excluded. The original oracle and the complete control observations remain
// exported; no local console entry is filtered or saved as an expectation.
func l17LoadConsoleExclusions(t *testing.T, cases []l17Case) map[string]map[string][]l17ConsoleExclusion {
	t.Helper()
	data, err := os.ReadFile("testdata/l17_console_provenance_controls.json")
	if err != nil {
		t.Fatal(err)
	}
	var controls struct {
		Captured   map[string]map[string]json.RawMessage       `json:"captured"`
		Exclusions map[string]map[string][]l17ConsoleExclusion `json:"host_console_exclusions"`
	}
	if err := json.Unmarshal(data, &controls); err != nil {
		t.Fatal(err)
	}
	byID := map[string]l17Case{}
	for _, c := range cases {
		byID[c.ID] = c
	}
	for _, api := range []string{"59.0", "67.0"} {
		if len(controls.Captured[api]) != 14 || len(controls.Exclusions[api]) != 3 {
			t.Fatalf("missing L17 console provenance controls API %s", api)
		}
		count := 0
		for id, entries := range controls.Exclusions[api] {
			c, ok := byID[id]
			if !ok || c.Kind != "runtime" || len(entries) == 0 {
				t.Fatalf("invalid L17 host console exclusion %s API %s", id, api)
			}
			var captured struct {
				ID         string            `json:"id"`
				Provenance []json.RawMessage `json:"consoleProvenance"`
			}
			if err := json.Unmarshal(controls.Captured[api][id], &captured); err != nil || captured.ID != id {
				t.Fatalf("missing native source capture %s API %s", id, api)
			}
			for _, entry := range entries {
				var diagnostic struct {
					ID, Type, Message string
				}
				if err := json.Unmarshal(entry.Entry, &diagnostic); err != nil || diagnostic.ID != id || diagnostic.Type != "warning" || entry.Reason == "" {
					t.Fatalf("host console exclusion is not a qualified native warning: %s API %s", id, api)
				}
				var evidence struct {
					Kind, Type, Message, ArgumentText, NativeSource string
					Stack                                           struct {
						CallFrames []struct {
							Source struct{ Path string }
						}
					}
				}
				if err := json.Unmarshal(entry.Evidence, &evidence); err != nil {
					t.Fatal(err)
				}
				found := false
				for _, record := range captured.Provenance {
					found = found || l13CanonicalJSON(t, string(record)) == l13CanonicalJSON(t, string(entry.Evidence))
				}
				frames := evidence.Stack.CallFrames
				if !found || evidence.Type != "warning" || len(frames) == 0 {
					t.Fatalf("uncaptured host warning source %s API %s", id, api)
				}
				for _, frame := range frames {
					if !strings.Contains(frame.Source.Path, "/auraFW/") && !strings.Contains(frame.Source.Path, "/aurafile/") {
						t.Fatalf("component or LWC-engine frame cannot be excluded: %s API %s", id, api)
					}
				}
				switch evidence.Kind {
				case "browser-log":
					if evidence.Message != diagnostic.Message || evidence.NativeSource != "security" || !strings.HasSuffix(frames[0].Source.Path, "/aura_prod.js") {
						t.Fatalf("invalid native Aura security warning %s API %s", id, api)
					}
				case "console-api":
					if evidence.ArgumentText != diagnostic.Message || !strings.HasSuffix(frames[0].Source.Path, "/apppart4-4.js") {
						t.Fatalf("invalid native profiler bootstrap warning %s API %s", id, api)
					}
				default:
					t.Fatalf("unqualified native console source %s API %s", id, api)
				}
				count++
			}
			// Also require every excluded entry to occur exactly in this
			// row's original native console, with its original ID and text.
			l17WithoutHostConsole(t, c.Expected[api], entries)
		}
		if count != 5 {
			t.Fatalf("L17 needs five captured host warning entries API %s, got %d", api, count)
		}
	}
	return controls.Exclusions
}

func l17WithoutHostConsole(t *testing.T, expected string, exclusions []l17ConsoleExclusion) string {
	t.Helper()
	if len(exclusions) == 0 {
		return expected
	}
	var native map[string]json.RawMessage
	if err := json.Unmarshal([]byte(strings.TrimPrefix(expected, "DOM|")), &native); err != nil {
		t.Fatal(err)
	}
	var entries []json.RawMessage
	if err := json.Unmarshal(native["console"], &entries); err != nil {
		t.Fatal(err)
	}
	for _, exclusion := range exclusions {
		found := false
		for index, entry := range entries {
			if l13CanonicalJSON(t, string(entry)) == l13CanonicalJSON(t, string(exclusion.Entry)) {
				entries = append(entries[:index], entries[index+1:]...)
				found = true
				break
			}
		}
		if !found {
			t.Fatal("host console exclusion must match an exact native entry")
		}
	}
	var err error
	native["console"], err = json.Marshal(entries)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(native)
	if err != nil {
		t.Fatal(err)
	}
	return "DOM|" + l13CanonicalJSON(t, string(data))
}

// Resource-reference controls retain the isolated native bundle identities so
// exact rejection text includes the captured filename without rewriting it.
func l17ResourceControls(t *testing.T, report *strings.Builder, capture bool) {
	t.Helper()
	data, err := os.ReadFile("testdata/l17_resource_reference_controls.json")
	if err != nil {
		t.Fatal(err)
	}
	var controls struct {
		Cases []l17Case `json:"cases"`
	}
	if err := json.Unmarshal(data, &controls); err != nil {
		t.Fatal(err)
	}
	if len(controls.Cases) != 8 {
		t.Fatalf("L17 needs eight isolated resource controls, got %d", len(controls.Cases))
	}
	for _, api := range []string{"59.0", "67.0"} {
		p, names := l17Project(t, api, controls.Cases, false)
		results, err := compile.CompileBatch(p, compile.Options{OutDir: filepath.Join(p.Root, "dist"), Namespace: "c"})
		if err != nil {
			t.Fatal(err)
		}
		matched := 0
		for index, c := range controls.Cases {
			if c.NativeBundle == "" {
				t.Fatalf("missing native control bundle %s", c.ID)
			}
			result, ok := results["force-app/main/default/lwc/"+names[index]]
			if !ok {
				t.Fatalf("missing control result %s", c.ID)
			}
			got := "COMPILE_OK"
			if result.Err != nil {
				got = "COMPILE_ERROR"
			}
			want := c.Expected[api]
			diagnosticGot, diagnosticWant := l17Diagnostic(result.Err), c.DiagnosticExpected[api]
			if (want != "COMPILE_OK" && want != "COMPILE_ERROR") || ((want == "COMPILE_ERROR") != (diagnosticWant != "")) {
				t.Fatalf("missing captured control answer %s API %s", c.ID, api)
			}
			status := "MISMATCH"
			if got == want && diagnosticGot == diagnosticWant {
				status = "MATCH"
				matched++
			} else if !capture {
				t.Errorf("%s API %s expected <%s|%s> actual <%s|%s>", c.ID, api, want, diagnosticWant, got, diagnosticGot)
			}
			fmt.Fprintf(report, "%s\t%s\tcontrol\t%s\t%s\t%s\t\n", api, c.ID, status, l13Cell(got+"|"+diagnosticGot), l13Cell(want+"|"+diagnosticWant))
		}
		t.Logf("L17 isolated resource controls API %s: %d/8", api, matched)
	}
}

// Reasons do not alter observations, expectations, assertions or match counts.
// Remaining host differences are logged, never asserted against old local
// values. A carry cannot absorb a difference in an native-owned field.
func l17CarriedDifference(t *testing.T, expected, actual string, fields []string) bool {
	t.Helper()
	allowed := map[string]bool{}
	for _, field := range fields {
		allowed[field] = true
	}
	for _, field := range l17DOMDifferences(t, strings.TrimPrefix(expected, "DOM|"), strings.TrimPrefix(actual, "DOM|")) {
		carried := allowed[field]
		for path := range allowed {
			carried = carried || strings.HasPrefix(field, path+".") || strings.HasPrefix(field, path+"[")
		}
		if !carried {
			return false
		}
	}
	return true
}

func l17DOMDifferences(t *testing.T, expected, actual string) []string {
	t.Helper()
	var want, got map[string]json.RawMessage
	if err := json.Unmarshal([]byte(expected), &want); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(actual), &got); err != nil {
		t.Fatal(err)
	}
	var different []string
	var walk func(string, json.RawMessage, json.RawMessage, bool, bool)
	walk = func(path string, w, g json.RawMessage, wok, gok bool) {
		if wok && gok && l13CanonicalJSON(t, string(w)) == l13CanonicalJSON(t, string(g)) {
			return
		}
		if wok && gok && path == "console" {
			// HTTP diagnostics remain exact even when a future qualified
			// sandbox carry changes the number or ordering of other entries.
			// Preserve entry text, case, metadata, duplicates and order within
			// each stream; neither observations nor native rows are rewritten.
			wm, gm := l17ConsoleStreams(t, w), l17ConsoleStreams(t, g)
			before := len(different)
			for _, stream := range []string{"http", "sandbox", "other"} {
				walk(path+"."+stream, wm[stream], gm[stream], true, true)
			}
			if len(different) == before {
				// Equal streams can still have different global ordering.
				// No field carry may silently accept that console difference.
				different = append(different, path+".order")
			}
			return
		}
		if wok && gok {
			if bytes.HasPrefix(w, []byte("{")) && bytes.HasPrefix(g, []byte("{")) {
				var wm, gm map[string]json.RawMessage
				if err := json.Unmarshal(w, &wm); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(g, &gm); err != nil {
					t.Fatal(err)
				}
				keys := map[string]bool{}
				for key := range wm {
					keys[key] = true
				}
				for key := range gm {
					keys[key] = true
				}
				for key := range keys {
					w, wok := wm[key]
					g, gok := gm[key]
					walk(path+"."+key, w, g, wok, gok)
				}
				return
			}
			if bytes.HasPrefix(w, []byte("[")) && bytes.HasPrefix(g, []byte("[")) {
				var wa, ga []json.RawMessage
				if err := json.Unmarshal(w, &wa); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(g, &ga); err != nil {
					t.Fatal(err)
				}
				if len(wa) == len(ga) {
					for index := range wa {
						walk(fmt.Sprintf("%s[%d]", path, index), wa[index], ga[index], true, true)
					}
					return
				}
			}
		}
		different = append(different, path)
	}
	keys := map[string]bool{}
	for key := range want {
		keys[key] = true
	}
	for key := range got {
		keys[key] = true
	}
	for key := range keys {
		w, wok := want[key]
		g, gok := got[key]
		walk(key, w, g, wok, gok)
	}
	sort.Strings(different)
	return different
}

func l17ConsoleStreams(t *testing.T, raw json.RawMessage) map[string]json.RawMessage {
	t.Helper()
	var entries []json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil {
		t.Fatal(err)
	}
	streams := map[string][]json.RawMessage{"http": {}, "sandbox": {}, "other": {}}
	for _, entry := range entries {
		var diagnostic struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(entry, &diagnostic); err != nil {
			t.Fatal(err)
		}
		stream := "other"
		if diagnostic.Type == "error" && strings.HasPrefix(diagnostic.Message, "Failed to load resource: the server responded with a status of ") {
			stream = "http"
		} else if (diagnostic.Type == "error" && strings.HasPrefix(diagnostic.Message, "Not allowed to load local resource: blob:")) ||
			(diagnostic.Type == "warning" && diagnostic.Message == "An iframe which has both allow-scripts and allow-same-origin for its sandbox attribute can escape its sandboxing.") {
			stream = "sandbox"
		}
		streams[stream] = append(streams[stream], entry)
	}
	result := map[string]json.RawMessage{}
	for name, entries := range streams {
		data, err := json.Marshal(entries)
		if err != nil {
			t.Fatal(err)
		}
		result[name] = data
	}
	return result
}

// A sandbox carry must not hide deletion, changes, duplicates or reordering
// of the native HTTP diagnostics (the independent review's regression).
func TestL17ConsoleHTTPDiagnosticsRemainExact(t *testing.T) {
	http := `{"id":"owned","type":"error","message":"Failed to load resource: the server responded with a status of 404 ()"}`
	sandbox := `{"id":"owned","type":"error","message":"Not allowed to load local resource: blob:[URL withheld]"}`
	want := `DOM|{"console":[` + http + `,` + sandbox + `]}`
	for _, row := range []struct {
		name, observed string
		allowed        bool
	}{
		{"sandbox_only", http, true},
		{"missing_http", "", false},
		{"changed_status", strings.ReplaceAll(http, "404", "403"), false},
		{"changed_case", strings.ReplaceAll(http, "Failed", "failed"), false},
		{"duplicate_http", http + `,` + http, false},
		{"reordered_http_blob", sandbox + `,` + http, false},
	} {
		t.Run(row.name, func(t *testing.T) {
			got := `DOM|{"console":[` + row.observed + `]}`
			if carried := l17CarriedDifference(t, want, got, []string{"console.sandbox"}); carried != row.allowed {
				t.Fatalf("HTTP diagnostic carry allowed=%v want=%v", carried, row.allowed)
			}
		})
	}
}

func TestL17HostConsoleExclusionsRemainScoped(t *testing.T) {
	data, err := os.ReadFile("testdata/l17_salesforce.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []l17Case
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	exclusions := l17LoadConsoleExclusions(t, cases)
	for _, api := range []string{"59.0", "67.0"} {
		id := "dom_repeat_loadScript_once"
		entries := exclusions[api][id]
		http := `{"id":"` + id + `","type":"error","message":"Failed to load resource: the server responded with a status of 404 ()"}`
		blob := `{"id":"` + id + `","type":"error","message":"Not allowed to load local resource: blob:[URL withheld]"}`
		host := string(entries[0].Entry)
		want := l17WithoutHostConsole(t, `DOM|{"value":null,"console":[`+http+`,`+host+`,`+blob+`]}`, entries)
		for _, row := range []struct {
			name, console string
			matches       bool
		}{
			{"host_only_exclusion", http + `,` + blob, true},
			{"missing_http", blob, false},
			{"changed_http_case", strings.ReplaceAll(http, "Failed", "failed") + `,` + blob, false},
			{"duplicate_http", http + `,` + http + `,` + blob, false},
			{"missing_blob", http, false},
			{"reordered_diagnostics", blob + `,` + http, false},
			{"local_warning_is_not_excluded", http + `,` + host + `,` + blob, false},
		} {
			t.Run(api+"/"+row.name, func(t *testing.T) {
				got := `DOM|{"value":null,"console":[` + row.console + `]}`
				if matches := l17CarriedDifference(t, want, got, nil); matches != row.matches {
					t.Fatalf("qualified host exclusion accepted=%v want=%v", matches, row.matches)
				}
			})
		}
		if l17CarriedDifference(t, want, strings.ReplaceAll(want, `"value":null`, `"value":"null"`), nil) {
			t.Fatal("host warning exclusion must preserve raw native null")
		}
	}
}

func l17DOMReason(t *testing.T, expected, actual string) string {
	t.Helper()
	var reasons []string
	for _, key := range l17DOMDifferences(t, expected, actual) {
		reasons = append(reasons, "L17: native "+key+" differs without a qualified carry")
	}
	sort.Strings(reasons)
	return strings.Join(reasons, "; ")
}

func l17Diagnostic(err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	if _, output, ok := strings.Cut(message, "\n"); ok {
		message = output
	}
	return strings.TrimPrefix(strings.Split(message, "\n")[0], "Error: ")
}

func l17Project(t *testing.T, api string, cases []l17Case, runtime bool) (project.Project, []string) {
	t.Helper()
	root := t.TempDir()
	l13CopyFixtures(t, "testdata/l17_fixtures", filepath.Join(root, "force-app/main/default"))
	names := make([]string, len(cases))
	var children strings.Builder
	for index, c := range cases {
		name := fmt.Sprintf("familyL17Compile%03d", index)
		if c.NativeBundle != "" {
			name = c.NativeBundle
		}
		if runtime {
			name = fmt.Sprintf("familyL17Dom%03dV%s", index, api[:2])
		}
		names[index] = name
		bundle := filepath.Join(root, "force-app/main/default/lwc", name)
		js := strings.ReplaceAll(c.JS, "FamilyResources", strings.ToUpper(name[:1])+name[1:])
		l13Write(t, filepath.Join(bundle, name+".js"), []byte(js))
		l13Write(t, filepath.Join(bundle, name+".html"), []byte(c.Template))
		fragment := c.MetaFragment
		if fragment == "" {
			fragment = "<isExposed>false</isExposed>"
		}
		l13Write(t, filepath.Join(bundle, name+".js-meta.xml"), []byte(`<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion>`+fragment+`</LightningComponentBundle>`))
		if c.CSS != nil {
			l13Write(t, filepath.Join(bundle, name+".css"), []byte(*c.CSS))
		}
		// Use the compiler's canonical camel-case-to-kebab registration.
		tag := fmt.Sprintf("c-family-l17-dom%03d-v%s", index, api[:2])
		children.WriteString("<" + tag + "></" + tag + ">")
	}
	if runtime {
		name := "familyL17Runtime" + api[:2]
		bundle := filepath.Join(root, "force-app/main/default/lwc", name)
		l13Write(t, filepath.Join(bundle, name+".js"), []byte(`import { LightningElement } from 'lwc'; export default class FamilyResources extends LightningElement { value='owned'; flag=true; handle() {} }`))
		l13Write(t, filepath.Join(bundle, name+".html"), []byte("<template>"+children.String()+"</template>"))
		l13Write(t, filepath.Join(bundle, name+".js-meta.xml"), []byte(`<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><isExposed>true</isExposed><targets><target>lightning__Tab</target></targets></LightningComponentBundle>`))
		// Export the captured native app/tab host metadata, not a replacement
		// root document. These inputs match the API59/67 runtime projects.
		tab := "FamilyL17Oracle" + api[:2]
		ns := ` xmlns="http://soap.sforce.com/2006/04/metadata"`
		l13Write(t, filepath.Join(root, "force-app/main/default/tabs", tab+".tab-meta.xml"), []byte(`<CustomTab`+ns+`><label>L17 Oracle `+api[:2]+`</label><lwcComponent>`+name+`</lwcComponent><motif>Custom1: Heart</motif></CustomTab>`))
		l13Write(t, filepath.Join(root, "force-app/main/default/applications/FamilyL17App67.app-meta.xml"), []byte(`<CustomApplication`+ns+`><formFactors>Large</formFactors><label>L17 Oracle</label><navType>Standard</navType><tabs>`+tab+`</tabs><uiType>Lightning</uiType></CustomApplication>`))
		l13Write(t, filepath.Join(root, "force-app/main/default/permissionsets/FamilyL17Access"+api[:2]+".permissionset-meta.xml"), []byte(`<PermissionSet`+ns+`><applicationVisibilities><application>FamilyL17App67</application><visible>true</visible></applicationVisibilities><label>L17 Oracle access `+api[:2]+`</label><tabSettings><tab>`+tab+`</tab><visibility>Visible</visibility></tabSettings></PermissionSet>`))
	}
	l13Write(t, filepath.Join(root, "sfdx-project.json"), []byte(`{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"","sourceApiVersion":"`+api+`"}`))
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	return p, names
}

func l17ObserveDOM(t *testing.T, api string, cases []l17Case, dependencies string) (map[string]json.RawMessage, map[string]string, error) {
	t.Helper()
	p, _ := l17Project(t, api, cases, true)
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	var err error
	org.Metadata, err = resource.LoadProject(p)
	if err != nil {
		return nil, nil, err
	}
	product := server.NewWithSource(&org, server.SourceMetadata{Project: p})
	defer product.ResetLightningCache()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// native adds this public alias to the same production tab handler. The
		// assigned canonical still exposes its preview spelling; translate
		// only the spelling here, retaining real HTML, headers and failures.
		if strings.HasPrefix(r.URL.Path, "/lightning/n/") {
			r = r.Clone(r.Context())
			requestURL := *r.URL
			requestURL.Path = "/lwc/preview/tab/" + strings.TrimPrefix(r.URL.Path, "/lightning/n/")
			requestURL.RawPath = ""
			r.URL = &requestURL
		}
		product.ServeHTTP(w, r)
	})
	// Native captures use HTTPS/HTTP2. Match the transport as well as the
	// real tab document so status diagnostics and MIME checks are comparable.
	local := httptest.NewUnstartedServer(handler)
	local.EnableHTTP2 = true
	local.StartTLS()
	defer local.Close()
	pageURL := local.URL + "/lightning/n/FamilyL17Oracle" + api[:2]
	response, err := local.Client().Get(pageURL)
	if err != nil {
		return nil, nil, err
	}
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		return nil, nil, err
	}
	if response.StatusCode != http.StatusOK || !strings.HasPrefix(response.Header.Get("Content-Type"), "text/html") {
		return nil, nil, fmt.Errorf("L17 production tab HTTP %d MIME %q: %s", response.StatusCode, response.Header.Get("Content-Type"), body)
	}
	t.Logf("L17 API %s production CustomTab: %s HTTP %d MIME %s X-Content-Type-Options %q", api, response.Proto, response.StatusCode, response.Header.Get("Content-Type"), response.Header.Get("X-Content-Type-Options"))
	// Retain actual host/header evidence for the relative conversion rows.
	// This never supplies an expected answer to the browser or changes a
	// response: it records the same production route used by the loader.
	documentURL, err := url.Parse(pageURL)
	if err != nil {
		return nil, nil, err
	}
	for _, relative := range []string{"123", "null", "undefined"} {
		probeURL := documentURL.ResolveReference(&url.URL{Path: relative}).String()
		request, err := http.NewRequest(http.MethodGet, probeURL, nil)
		if err != nil {
			return nil, nil, err
		}
		request.Header.Set("Sec-Fetch-Dest", "style")
		response, err := local.Client().Do(request)
		if err != nil {
			return nil, nil, err
		}
		_, err = io.Copy(io.Discard, response.Body)
		response.Body.Close()
		if err != nil {
			return nil, nil, err
		}
		t.Logf("L17 API %s production tab relative stylesheet %s: %s HTTP %d MIME %s X-Content-Type-Options %q", api, relative, response.Proto, response.StatusCode, response.Header.Get("Content-Type"), response.Header.Get("X-Content-Type-Options"))
	}
	ids, assignments := []string{}, map[string]int{}
	scriptIndex := 0
	for _, c := range cases {
		ids = append(ids, c.ID)
		assignments[c.ID] = 0
		if c.Method == "loadScript" {
			assignments[c.ID] = 1 + scriptIndex%2
			scriptIndex++
		}
	}
	playwright := os.Getenv("GLADE_LWC_PLAYWRIGHT_MODULE")
	if playwright == "" {
		playwright = filepath.Join(dependencies, "lwcruntime/node_modules/playwright")
	}
	observer, err := filepath.Abs("testdata/l17_browser.mjs")
	if err != nil {
		return nil, nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", observer)
	// Native captures checkpoint and yield individual rows to pending trains.
	// This credential-free observer belongs to one Go conformance invocation;
	// the enclosing runner manages its scheduling and termination. Do not let
	// the exported capture transport publish a native-capture yield checkpoint.
	cmd.Env = append(os.Environ(), "GLADE_BROWSER_PAGES=3", "GLADE_BROWSER_YIELD_FILE=")
	cmd.Stdin = strings.NewReader(l13JSON(t, map[string]any{"url": pageURL, "ids": ids, "pageAssignments": assignments, "playwrightModule": playwright, "executablePath": os.Getenv("GLADE_LWC_BROWSER_EXECUTABLE")}))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		return nil, nil, fmt.Errorf("L17 local browser: %w: %s", err, stderr.String())
	}
	var observed struct {
		Values map[string]json.RawMessage `json:"values"`
		Errors map[string]string          `json:"errors"`
		Host   json.RawMessage            `json:"host"`
	}
	if err := json.Unmarshal(output, &observed); err != nil {
		return nil, nil, err
	}
	t.Logf("L17 API %s browser document: %s", api, observed.Host)
	for _, id := range ids {
		_, hasValue := observed.Values[id]
		_, hasError := observed.Errors[id]
		if !hasValue && !hasError {
			if observed.Errors == nil {
				observed.Errors = map[string]string{}
			}
			observed.Errors[id] = "L17 local browser omitted the row without an observation or error"
		}
	}
	return observed.Values, observed.Errors, nil
}
