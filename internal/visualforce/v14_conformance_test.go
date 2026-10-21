package visualforce_test

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/server"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/visualforce"
	"golang.org/x/net/html"
)

type v14Observation struct {
	Query           string            `json:"query"`
	Interaction     string            `json:"interaction"`
	RequestHeaders  map[string]string `json:"request_headers"`
	ResponseHeader  string            `json:"response_header"`
	HeaderReduction string            `json:"header_reduction"`
	ObserveCookie   bool              `json:"observe_cookie"`
	ObserveScheme   bool              `json:"observe_scheme"`
	CookieLifetime  bool              `json:"observe_cookie_lifetime"`
	ValueOnly       bool              `json:"control_value_only"`
	ActionResponse  bool              `json:"action_response"`
	ProtectedGET    bool              `json:"protected_get"`
}

type v14Case struct {
	ID           string                     `json:"id"`
	Group        string                     `json:"group"`
	Kind         string                     `json:"kind"`
	Inputs       map[string]v04Input        `json:"inputs"`
	Expected     map[string]string          `json:"expected"`
	Diagnostics  map[string][]v04Diagnostic `json:"diagnostics"`
	Observation  v14Observation             `json:"observation"`
	Carry        map[string]v14Carry        `json:"carry,omitempty"`
	Unobservable map[string]v14Unobservable `json:"unobservable,omitempty"`
}

type v14Unobservable struct {
	Reason      string          `json:"reason"`
	CompileID   string          `json:"compile_id"`
	Diagnostics []v04Diagnostic `json:"diagnostics"`
}

type v14Carry struct {
	Owner                 string                    `json:"owner"`
	Reason                string                    `json:"reason"`
	DiagnosticDifferences []v14DiagnosticDifference `json:"diagnostic_differences,omitempty"`
	DOMDifferences        []v14DOMDifference        `json:"dom_differences,omitempty"`
}

type v14DiagnosticDifference struct {
	Native v04Diagnostic `json:"native"`
	Local  v04Diagnostic `json:"local"`
}

type v14DOMDifference struct {
	Field         string          `json:"field"`
	NativePresent bool            `json:"native_present"`
	LocalPresent  bool            `json:"local_present"`
	Native        json.RawMessage `json:"native,omitempty"`
	Local         json.RawMessage `json:"local,omitempty"`
}

type v14Result struct {
	API         string          `json:"api"`
	ID          string          `json:"id"`
	Actual      string          `json:"actual"`
	Diagnostics []v04Diagnostic `json:"diagnostics"`
	Root        string          `json:"root"`
}

const v14BatchSize = 16

// TestV14SalesforceConformance replays the 166 compile/metadata and 127 native
// DOM rows plus request, query, 26 compile/10 DOM self-review controls and the
// two compile/two DOM incoming-header versus equal-value-write controls at
// API 59/67, and the independent-review qualified-query/bulk-header controls.
// Three restored enforced-query DOM rows run at API59; API67 omits only those
// runtimes whose captured compile counterparts reject. Original and supplemental
// match counts stay separate. Compile
// rows use V04's shared adapter; DOM rows use the product HTTP handler,
// request runtime, view state and partial
// responses. The observer follows the captured script-free text, control,
// header and cookie reductions. Every answer and diagnostic is compared as
// exact text, including whitespace and raw null; no expected values seed the
// execution. CI needs only the exported owned sources and native testdata.
// Sequential batches use this compiled test binary to release each batch's
// compiler/runtime allocations, including native parser memory, on exit.
// GLADE_V14_CAPTURE=1 records mismatches without failing; GLADE_V14_REPORT
// selects the optional per-row TSV report path.
var v14APIVersions = []string{"59.0", "67.0"}

func TestV14SalesforceConformance(t *testing.T) {
	capture := os.Getenv("GLADE_V14_CAPTURE") != ""
	data, err := os.ReadFile("testdata/v14_salesforce.json")
	if err != nil {
		t.Fatal(err)
	}
	var table struct {
		OracleCommit   string            `json:"oracle_commit"`
		FixtureAccount map[string]string `json:"fixture_account"`
		Cases          []v14Case         `json:"cases"`
	}
	if err := json.Unmarshal(data, &table); err != nil {
		t.Fatal(err)
	}
	if table.OracleCommit != "capture-revision-075780736040471116419815387387690789181439963745429628196339988119388690323893" || len(table.Cases) != 293 {
		t.Fatalf("V14 requires all 293 observed cases from the final native oracle, got %d at %s", len(table.Cases), table.OracleCommit)
	}
	originalCount := len(table.Cases)
	controlData, err := os.ReadFile("testdata/v14_request_controls.json")
	if err != nil {
		t.Fatal(err)
	}
	var controls struct {
		OracleCommit string    `json:"oracle_commit"`
		Cases        []v14Case `json:"cases"`
	}
	if err := json.Unmarshal(controlData, &controls); err != nil {
		t.Fatal(err)
	}
	if controls.OracleCommit != "capture-revision-019345355658226931679656844114749265143090103687012973787230617487167019509916" || len(controls.Cases) != 24 {
		t.Fatal("V14 requires all 24 captured request controls")
	}
	for _, c := range controls.Cases {
		if (!strings.HasPrefix(c.ID, "c_control_") && !strings.HasPrefix(c.ID, "r_control_")) || c.Group != "headers-cookies-redirects" {
			t.Fatalf("invalid V14 request control %q", c.ID)
		}
	}
	table.Cases = append(table.Cases, controls.Cases...)
	queryData, err := os.ReadFile("testdata/v14_query_controls.json")
	if err != nil {
		t.Fatal(err)
	}
	var queryControls struct {
		OracleCommit string    `json:"oracle_commit"`
		Cases        []v14Case `json:"cases"`
	}
	if err := json.Unmarshal(queryData, &queryControls); err != nil {
		t.Fatal(err)
	}
	if queryControls.OracleCommit != "capture-revision-043007246295575178775732173929333622138280611909624685461914777742396049222185" || len(queryControls.Cases) != 15 {
		t.Fatal("V14 requires all 15 captured query controls")
	}
	for _, c := range queryControls.Cases {
		if (!strings.HasPrefix(c.ID, "c_query_control_") && !strings.HasPrefix(c.ID, "r_query_control_")) || c.Group != "authorized-record-projection" {
			t.Fatalf("invalid V14 query control %q", c.ID)
		}
	}
	table.Cases = append(table.Cases, queryControls.Cases...)
	reviewData, err := os.ReadFile("testdata/v14_review_controls.json")
	if err != nil {
		t.Fatal(err)
	}
	var reviewControls struct {
		OracleCommit string    `json:"oracle_commit"`
		Cases        []v14Case `json:"cases"`
	}
	if err := json.Unmarshal(reviewData, &reviewControls); err != nil {
		t.Fatal(err)
	}
	if reviewControls.OracleCommit != "capture-revision-084508734866408910633528319687299319270731003721417375658788978649118463368665" || len(reviewControls.Cases) != 36 {
		t.Fatal("V14 requires all 36 captured self-review controls")
	}
	for _, c := range reviewControls.Cases {
		if (!strings.HasPrefix(c.ID, "c_review_") && !strings.HasPrefix(c.ID, "r_review_")) ||
			(c.Group != "authorized-record-projection" && c.Group != "headers-cookies-redirects") {
			t.Fatalf("invalid V14 self-review control %q", c.ID)
		}
	}
	table.Cases = append(table.Cases, reviewControls.Cases...)
	headerData, err := os.ReadFile("testdata/v14_header_write_controls.json")
	if err != nil {
		t.Fatal(err)
	}
	var headerControls struct {
		OracleCommit string    `json:"oracle_commit"`
		Cases        []v14Case `json:"cases"`
	}
	if err := json.Unmarshal(headerData, &headerControls); err != nil {
		t.Fatal(err)
	}
	if headerControls.OracleCommit != "capture-revision-000540760456532397856577741900083030564408164451368654225524384100854319070645" || len(headerControls.Cases) != 4 {
		t.Fatal("V14 requires all four captured header-write controls")
	}
	for _, c := range headerControls.Cases {
		if (!strings.HasPrefix(c.ID, "c_header_write_control_") && !strings.HasPrefix(c.ID, "r_header_write_control_")) ||
			c.Group != "headers-cookies-redirects" || len(c.Carry) != 0 {
			t.Fatalf("invalid V14 header-write control %q", c.ID)
		}
	}
	table.Cases = append(table.Cases, headerControls.Cases...)
	independentData, err := os.ReadFile("testdata/v14_independent_review_controls.json")
	if err != nil {
		t.Fatal(err)
	}
	var independentControls struct {
		OracleCommit string    `json:"oracle_commit"`
		Cases        []v14Case `json:"cases"`
	}
	if err := json.Unmarshal(independentData, &independentControls); err != nil {
		t.Fatal(err)
	}
	if independentControls.OracleCommit != "capture-revision-112084910557840980278523560936111317580970580411105463084548979824520939497932" || len(independentControls.Cases) != 40 {
		t.Fatal("V14 requires all 40 independent-review cases, with three native API67 runtime rejections")
	}
	for _, c := range independentControls.Cases {
		if (!strings.HasPrefix(c.ID, "c_review_") && !strings.HasPrefix(c.ID, "r_review_") && !v14EnforcedRuntime(c.ID)) ||
			(c.Group != "authorized-record-projection" && c.Group != "headers-cookies-redirects") || len(c.Carry) != 0 {
			t.Fatalf("invalid V14 independent-review control %q", c.ID)
		}
	}
	table.Cases = append(table.Cases, independentControls.Cases...)
	if len(table.FixtureAccount) != 4 || table.FixtureAccount["Name"] != "V14_OWNED_READ" ||
		table.FixtureAccount["Description"] != "V14_PRIVATE_SENTINEL" || table.FixtureAccount["Phone"] != "5550100" || table.FixtureAccount["Industry"] != "Technology" {
		t.Fatal("V14 requires the owned Account input used by the native capture")
	}
	counts, seen, groups := map[string]int{}, map[string]bool{}, map[string]int{}
	byID := map[string]v14Case{}
	for _, c := range table.Cases {
		byID[c.ID] = c
	}
	for _, c := range table.Cases {
		if c.ID == "" || seen[c.ID] || (c.Kind != "compile" && c.Kind != "runtime") {
			t.Fatalf("invalid or duplicate V14 case %q (%s)", c.ID, c.Kind)
		}
		seen[c.ID], groups[c.Group] = true, groups[c.Group]+1
		counts[c.Kind]++
		for api := range c.Unobservable {
			if api != "67.0" || c.Kind != "runtime" || !v14EnforcedRuntime(c.ID) {
				t.Fatalf("%s: unobservability outside the captured API67 enforced-query rejection", c.ID)
			}
		}
		for api, carry := range c.Carry {
			if (api != "59.0" && api != "67.0") || carry.Reason == "" || strings.ContainsAny(carry.Reason, "\r\n") ||
				(carry.Owner != "field and input rendering" && carry.Owner != "V12") {
				t.Fatalf("%s: invalid carry at API %s", c.ID, api)
			}
			if c.Kind == "compile" && len(carry.DiagnosticDifferences) == 0 {
				t.Fatalf("%s: compile carry requires exact recorded diagnostic differences at API %s", c.ID, api)
			}
			for _, difference := range carry.DiagnosticDifferences {
				if c.Kind != "compile" || difference.Native.Resource != "ApexClass" ||
					difference.Local.Resource != difference.Native.Resource || difference.Local.Type != difference.Native.Type ||
					difference.Native.Message == "" || difference.Local.Message == "" || difference.Native == difference.Local {
					t.Fatalf("%s: invalid diagnostic text carry at API %s", c.ID, api)
				}
				found := false
				for _, native := range c.Diagnostics[api] {
					found = found || native == difference.Native
				}
				if !found {
					t.Fatalf("%s: carried diagnostic is absent from the native answer at API %s", c.ID, api)
				}
			}
			if c.Kind == "runtime" {
				if len(carry.DOMDifferences) == 0 {
					t.Fatalf("%s: runtime carry requires exact recorded DOM differences", c.ID)
				}
				var native map[string]json.RawMessage
				if err := json.Unmarshal([]byte(strings.TrimPrefix(c.Expected[api], "DOM|")), &native); err != nil {
					t.Fatal(err)
				}
				fields := map[string]bool{}
				for _, difference := range carry.DOMDifferences {
					value, present := native[difference.Field]
					if difference.Field == "" || fields[difference.Field] || present != difference.NativePresent ||
						v04Encode(t, value) != v04Encode(t, difference.Native) ||
						(!difference.NativePresent && len(difference.Native) != 0) ||
						(difference.LocalPresent && !json.Valid(difference.Local)) ||
						(!difference.LocalPresent && len(difference.Local) != 0) ||
						(difference.NativePresent == difference.LocalPresent && v04Encode(t, difference.Native) == v04Encode(t, difference.Local)) {
						t.Fatalf("%s: invalid recorded DOM difference at API %s", c.ID, api)
					}
					if (carry.Owner == "field and input rendering" && difference.Field != "text") ||
						(carry.Owner == "V12" && difference.Field != "fields") {
						t.Fatalf("%s: DOM carry exempts fields outside %s ownership", c.ID, carry.Owner)
					}
					fields[difference.Field] = true
				}
			} else if len(carry.DOMDifferences) != 0 {
				t.Fatalf("%s: compile carry cannot exempt DOM fields", c.ID)
			}
		}
		for _, api := range v14APIVersions {
			if omission, omitted := c.Unobservable[api]; omitted {
				compiled := byID[omission.CompileID]
				_, hasInput := c.Inputs[api]
				_, hasAnswer := c.Expected[api]
				_, hasDiagnostics := c.Diagnostics[api]
				if omission.CompileID != "c_"+strings.TrimPrefix(c.ID, "r_") || omission.Reason == "" ||
					strings.ContainsAny(omission.Reason, "\r\n") || hasInput || hasAnswer || hasDiagnostics ||
					compiled.Kind != "compile" || compiled.Expected[api] != "COMPILE_ERROR" || len(omission.Diagnostics) == 0 ||
					v04Encode(t, omission.Diagnostics) != v04Encode(t, compiled.Diagnostics[api]) {
					t.Fatalf("%s: runtime omission lacks its exact native compile rejection at API %s", c.ID, api)
				}
				continue
			}
			input := c.Inputs[api]
			if input.Name == "" || len(input.Files) < 3 || input.Files["sfdx-project.json"] == "" ||
				input.Files["force-app/main/default/pages/"+input.Name+".page"] == "" ||
				input.Files["force-app/main/default/pages/"+input.Name+".page-meta.xml"] == "" {
				t.Fatalf("%s: incomplete captured input at API %s", c.ID, api)
			}
			answer, exists := c.Expected[api]
			diagnostics, diagnosticExists := c.Diagnostics[api]
			if !exists || !diagnosticExists || diagnostics == nil {
				t.Fatalf("%s: missing native answer/diagnostics at API %s", c.ID, api)
			}
			if c.Kind == "compile" {
				if (answer != "COMPILE_OK" && answer != "COMPILE_ERROR") || (answer == "COMPILE_ERROR") != (len(diagnostics) > 0) {
					t.Fatalf("%s: inconsistent native compile answer at API %s", c.ID, api)
				}
				for _, d := range diagnostics {
					if d.Resource == "" || d.Type == "" || d.Message == "" {
						t.Fatalf("%s: incomplete native diagnostic at API %s", c.ID, api)
					}
				}
			} else {
				var native map[string]json.RawMessage
				if !strings.HasPrefix(answer, "DOM|") || json.Unmarshal([]byte(strings.TrimPrefix(answer, "DOM|")), &native) != nil || native["rootPresent"] == nil || len(diagnostics) != 0 {
					t.Fatalf("%s: invalid native DOM answer at API %s", c.ID, api)
				}
				if string(native["rootPresent"]) == "true" && (native["text"] == nil || (!c.Observation.ValueOnly && native["fields"] == nil)) {
					t.Fatalf("%s: incomplete native text/fields at API %s", c.ID, api)
				}
			}
		}
	}
	if counts["compile"] != 240 || counts["runtime"] != 172 || len(groups) != 4 ||
		groups["principal-binding"] != 47 || groups["authorized-record-projection"] != 144 ||
		groups["page-class-access"] != 40 || groups["headers-cookies-redirects"] != 181 ||
		len(v14ObservedCases(table.Cases, "59.0")) != 412 || len(v14ObservedCases(table.Cases, "67.0")) != 409 {
		t.Fatalf("V14 requires all original and supplemental native rows, got %v, %v", counts, groups)
	}

	if batch := os.Getenv("GLADE_V14_BATCH"); batch != "" {
		parts := strings.Split(batch, ":")
		if len(parts) != 3 || (parts[0] != "59.0" && parts[0] != "67.0") {
			t.Fatalf("invalid V14 batch %q", batch)
		}
		first, firstErr := strconv.Atoi(parts[1])
		last, lastErr := strconv.Atoi(parts[2])
		cases := v14ObservedCases(table.Cases, parts[0])
		if firstErr != nil || lastErr != nil || first < 0 || first >= last || last > len(cases) || last-first > v14BatchSize {
			t.Fatalf("invalid V14 batch range %q", batch)
		}
		v14RunBatch(t, parts[0], cases[first:last], table.FixtureAccount)
		return
	}

	var report strings.Builder
	var output io.Writer = &report
	if path := os.Getenv("GLADE_V14_REPORT"); path != "" {
		file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := file.Close(); err != nil {
				t.Error(err)
			}
		})
		output = file
	}
	tsv := csv.NewWriter(output)
	tsv.Comma = '\t'
	writeRow := func(t *testing.T, fields ...string) {
		if err := tsv.Write(fields); err != nil {
			t.Fatal(err)
		}
		tsv.Flush()
		if err := tsv.Error(); err != nil {
			t.Fatal(err)
		}
	}
	writeRow(t, "api", "id", "status", "actual", "expected", "group", "diagnostic", "owner", "reason", "acceptance_status", "diagnostic_status", "expected_diagnostic")
	matches, compileMatches, acceptanceMatches, diagnosticMatches, domMatches := 0, 0, 0, 0, 0
	totalObserved, totalDOM, totalCompile := 0, 0, 0
	for _, api := range v14APIVersions {
		// One subtest per API so a lane can run a single API shard with
		// -run '^TestV14SalesforceConformance$/^59\.0$'; every row is still
		// asserted exactly once and the per-API totals are unchanged.
		t.Run(api, func(t *testing.T) {
			cases := v14ObservedCases(table.Cases, api)
			apiCounts := map[string]int{}
			for _, c := range cases {
				apiCounts[c.Kind]++
			}
			for _, c := range table.Cases {
				if omission, omitted := c.Unobservable[api]; omitted {
					t.Logf("API %s %s unobservable: %s (%s)", api, c.ID, omission.Reason, omission.CompileID)
					writeRow(t, api, c.ID, "UNOBSERVABLE", "", "", c.Group, "", "", omission.Reason, "NOT_APPLICABLE", "NOT_APPLICABLE", v04Encode(t, omission.Diagnostics))
				}
			}
			apiMatches, apiCompileMatches, apiAcceptanceMatches, apiDiagnosticMatches, apiDOMMatches := 0, 0, 0, 0, 0
			originalMatches := 0
			requestMatches := 0
			queryMatches := 0
			reviewMatches := 0
			headerMatches := 0
			independentMatches := 0
			for first := 0; first < len(cases); first += v14BatchSize {
				last := min(first+v14BatchSize, len(cases))
				results, batchErr := v14ExecuteBatch(t, api, first, last)
				for i, result := range results {
					c := cases[first+i]
					if result.ID != c.ID {
						t.Fatalf("API %s: batch row %d has ID %q, expected %q", api, first+i, result.ID, c.ID)
					}
					got, diagnostics := result.Actual, result.Diagnostics
					want := c.Expected[api]
					actualDiagnostic, expectedDiagnostic := v04Encode(t, diagnostics), v04Encode(t, c.Diagnostics[api])
					acceptanceMatch := got == want
					diagnosticMatch := c.Kind != "compile" || actualDiagnostic == expectedDiagnostic
					status, acceptanceStatus, diagnosticStatus := "MISMATCH", "MISMATCH", "NOT_APPLICABLE"
					if acceptanceMatch {
						acceptanceStatus = "MATCH"
						if c.Kind == "compile" {
							apiAcceptanceMatches++
						}
					}
					if c.Kind == "compile" {
						diagnosticStatus = "MISMATCH"
						if diagnosticMatch {
							diagnosticStatus = "MATCH"
							apiDiagnosticMatches++
						}
					}
					if acceptanceMatch && diagnosticMatch {
						status = "MATCH"
						apiMatches++
						if first+i < originalCount {
							originalMatches++
						} else if first+i < originalCount+len(controls.Cases) {
							requestMatches++
						} else if first+i < originalCount+len(controls.Cases)+len(queryControls.Cases) {
							queryMatches++
						} else if first+i < originalCount+len(controls.Cases)+len(queryControls.Cases)+len(reviewControls.Cases) {
							reviewMatches++
						} else if first+i < originalCount+len(controls.Cases)+len(queryControls.Cases)+len(reviewControls.Cases)+len(headerControls.Cases) {
							headerMatches++
						} else {
							independentMatches++
						}
						if c.Kind == "compile" {
							apiCompileMatches++
						} else {
							apiDOMMatches++
						}
					}
					carry := v14Carry{}
					if !acceptanceMatch || !diagnosticMatch {
						carry = c.Carry[api]
						if len(carry.DiagnosticDifferences) != 0 && (!acceptanceMatch ||
							!v14CarriedDiagnosticsMatch(t, diagnostics, c.Diagnostics[api], carry.DiagnosticDifferences)) {
							// A text carry cannot exempt acceptance, diagnostic shape,
							// dependent-page rejection or an unrecorded text difference.
							carry = v14Carry{}
						}
						if len(carry.DOMDifferences) != 0 &&
							!v14CarriedDOMMatches(t, got, want, carry.DOMDifferences) {
							carry = v14Carry{}
						}
						if carry.Owner != "" {
							status = "CARRY"
							t.Logf("API %s %s carry %s: %s", api, c.ID, carry.Owner, carry.Reason)
						}
					}
					// Matching rows always remain exact assertions, including rows
					// whose earlier carry has been resolved by a parent change.
					// Only temporary report paths are hidden after the comparison.
					writeRow(t, api, c.ID, status, got, want, c.Group, strings.ReplaceAll(actualDiagnostic, result.Root, "<fixture>"), carry.Owner, carry.Reason, acceptanceStatus, diagnosticStatus, expectedDiagnostic)
					if !capture && carry.Owner == "" && (!acceptanceMatch || !diagnosticMatch) {
						t.Errorf("API %s %s: actual <%s> expected <%s>; diagnostics <%s> expected <%s>", api, c.ID, got, want, actualDiagnostic, expectedDiagnostic)
					}
				}
				if batchErr != nil {
					t.Fatal(batchErr)
				}
				t.Logf("V14 API %s completed %d/%d rows; exact %d", api, last, len(cases), apiMatches)
			}
			writeRow(t, "ORIGINAL_API_TOTAL", api, fmt.Sprintf("%d/%d", originalMatches, originalCount))
			writeRow(t, "CONTROL_API_TOTAL", api, fmt.Sprintf("%d/%d", requestMatches, len(controls.Cases)))
			writeRow(t, "QUERY_CONTROL_API_TOTAL", api, fmt.Sprintf("%d/%d", queryMatches, len(queryControls.Cases)))
			writeRow(t, "REVIEW_CONTROL_API_TOTAL", api, fmt.Sprintf("%d/%d", reviewMatches, len(reviewControls.Cases)))
			writeRow(t, "HEADER_WRITE_CONTROL_API_TOTAL", api, fmt.Sprintf("%d/%d", headerMatches, len(headerControls.Cases)))
			independentCount := len(v14ObservedCases(independentControls.Cases, api))
			writeRow(t, "INDEPENDENT_REVIEW_API_TOTAL", api, fmt.Sprintf("%d/%d", independentMatches, independentCount))
			writeRow(t, "API_TOTAL", api, fmt.Sprintf("%d/%d", apiMatches, len(cases)))
			writeRow(t, "COMPILE_API_TOTAL", api, fmt.Sprintf("%d/%d", apiCompileMatches, apiCounts["compile"]))
			writeRow(t, "COMPILE_ACCEPTANCE_API_TOTAL", api, fmt.Sprintf("%d/%d", apiAcceptanceMatches, apiCounts["compile"]))
			writeRow(t, "DIAGNOSTIC_API_TOTAL", api, fmt.Sprintf("%d/%d", apiDiagnosticMatches, apiCounts["compile"]))
			writeRow(t, "DOM_API_TOTAL", api, fmt.Sprintf("%d/%d", apiDOMMatches, apiCounts["runtime"]))
			t.Logf("V14 API %s original exact %d/293; request controls %d/24; query controls %d/15; self-review controls %d/36; header-write controls %d/4; independent-review controls %d/%d; combined %d/%d; compile %d/%d, acceptance %d/%d, diagnostics %d/%d; DOM %d/%d", api, originalMatches, requestMatches, queryMatches, reviewMatches, headerMatches, independentMatches, independentCount, apiMatches, len(cases), apiCompileMatches, apiCounts["compile"], apiAcceptanceMatches, apiCounts["compile"], apiDiagnosticMatches, apiCounts["compile"], apiDOMMatches, apiCounts["runtime"])
			matches += apiMatches
			compileMatches += apiCompileMatches
			acceptanceMatches += apiAcceptanceMatches
			diagnosticMatches += apiDiagnosticMatches
			domMatches += apiDOMMatches
			totalObserved += len(cases)
			totalDOM += apiCounts["runtime"]
			totalCompile += apiCounts["compile"]
		})
	}
	writeRow(t, "COMPILE_TOTAL", fmt.Sprintf("%d/%d", compileMatches, totalCompile))
	writeRow(t, "COMPILE_ACCEPTANCE_TOTAL", fmt.Sprintf("%d/%d", acceptanceMatches, totalCompile))
	writeRow(t, "DIAGNOSTIC_TOTAL", fmt.Sprintf("%d/%d", diagnosticMatches, totalCompile))
	writeRow(t, "DOM_TOTAL", fmt.Sprintf("%d/%d", domMatches, totalDOM))
	writeRow(t, "TOTAL", fmt.Sprintf("%d/%d", matches, totalObserved))
	t.Logf("V14 exact matches %d/%d; compile %d/%d; DOM %d/%d", matches, totalObserved, compileMatches, totalCompile, domMatches, totalDOM)
	tsv.Flush()
	if err := tsv.Error(); err != nil {
		t.Fatal(err)
	}
}

func v14EnforcedRuntime(id string) bool {
	return id == "r_projection_with_enforced_count" || id == "r_projection_without_enforced_count" || id == "r_projection_inherited_enforced_count"
}

func v14ObservedCases(cases []v14Case, api string) []v14Case {
	observed := make([]v14Case, 0, len(cases))
	for _, c := range cases {
		if _, omitted := c.Unobservable[api]; !omitted {
			observed = append(observed, c)
		}
	}
	return observed
}

func v14CarriedDOMMatches(t *testing.T, actualText, nativeText string, differences []v14DOMDifference) bool {
	var actual, native map[string]json.RawMessage
	if !strings.HasPrefix(actualText, "DOM|") ||
		json.Unmarshal([]byte(strings.TrimPrefix(actualText, "DOM|")), &actual) != nil ||
		json.Unmarshal([]byte(strings.TrimPrefix(nativeText, "DOM|")), &native) != nil {
		return false
	}
	for _, difference := range differences {
		value, present := actual[difference.Field]
		if present != difference.LocalPresent || v04Encode(t, value) != v04Encode(t, difference.Local) {
			return false
		}
		if difference.NativePresent {
			actual[difference.Field] = difference.Native
		} else {
			delete(actual, difference.Field)
		}
	}
	return v04Encode(t, actual) == v04Encode(t, native)
}

func v14CarriedDiagnosticsMatch(t *testing.T, actual, native []v04Diagnostic, differences []v14DiagnosticDifference) bool {
	t.Helper()
	comparison := make([]v04Diagnostic, len(actual))
	copy(comparison, actual)
	for i, observed := range comparison {
		for _, difference := range differences {
			if observed == difference.Local {
				comparison[i] = difference.Native
				break
			}
		}
	}
	v04SortDiagnostics(comparison)
	return v04Encode(t, comparison) == v04Encode(t, native)
}

// Only child processes execute fixtures. They keep each row's project and
// request state private and publish raw observations without comparing them.
// The parent applies the original exact comparisons to every returned row.
func v14RunBatch(t *testing.T, api string, cases []v14Case, accountFields map[string]string) {
	t.Helper()
	path := os.Getenv("GLADE_V14_BATCH_RESULT")
	if path == "" {
		t.Fatal("V14 batch result path missing")
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Error(err)
		}
	}()
	encoder := json.NewEncoder(file)
	encoder.SetEscapeHTML(false)
	for _, c := range cases {
		if !t.Run(c.ID, func(t *testing.T) {
			root := t.TempDir()
			input := c.Inputs[api]
			for path, source := range input.Files {
				if filepath.IsAbs(path) || filepath.Clean(path) != path || strings.HasPrefix(path, "..") {
					t.Fatalf("%s: invalid fixture path %q", c.ID, path)
				}
				fullPath := filepath.Join(root, path)
				if err := os.MkdirAll(filepath.Dir(fullPath), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(fullPath, []byte(source), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			got, diagnostics := "", []v04Diagnostic{}
			if c.Kind == "compile" {
				got, diagnostics = v14Compile(t, root, input.Name)
			} else {
				got = v14Render(t, root, input.Name, c, accountFields)
			}
			if err := encoder.Encode(v14Result{API: api, ID: c.ID, Actual: got, Diagnostics: diagnostics, Root: root}); err != nil {
				t.Fatal(err)
			}
		}) {
			return
		}
	}
}

func v14Compile(t *testing.T, root, name string) (string, []v04Diagnostic) {
	t.Helper()
	v14CompileSchema(t, root)
	got, diagnostics := v04Execute(t, root, name, v04Case{Kind: "compile"})
	p, err := project.Load(root)
	if err != nil {
		return got, diagnostics
	}
	for _, d := range diagnostics {
		if d.Resource != "ApexClass" {
			continue
		}
		// Error rows use the product's Metadata API presentation. The shared
		// adapter still performs compilation and dependent-page validation.
		compiled, err := visualforce.ControllerCompileDiagnostics(p)
		if err != nil {
			t.Fatal(err)
		}
		kept := make([]v04Diagnostic, 0, len(diagnostics))
		for _, existing := range diagnostics {
			if existing.Resource != "ApexClass" {
				kept = append(kept, existing)
			}
		}
		for _, observed := range compiled {
			if observed.Severity == diagnostic.Error {
				kept = append(kept, v04Diagnostic{Resource: "ApexClass", Type: "Error", Message: observed.Message})
			}
		}
		diagnostics = kept
		v04SortDiagnostics(diagnostics)
		break
	}
	var accessError *visualforce.VisualforceAccessMetadataError
	if errors.As(visualforce.ValidateVisualforceAccessMetadata(p), &accessError) {
		for i := range diagnostics {
			if diagnostics[i].Resource == "ApexPage" && diagnostics[i].Message == accessError.Error() {
				diagnostics[i].Resource = "PermissionSet"
			}
		}
		v04SortDiagnostics(diagnostics)
	}
	return got, diagnostics
}

// Compile against the same independently defined standard objects as the
// runtime's administrator org. The native query controls validate its Account
// field types; no expected answer contributes to the schema or compilation.
func v14CompileSchema(t *testing.T, root string) {
	t.Helper()
	org, _ := v04Org()
	var snapshot schema.Schema
	for name, object := range org.Objects {
		metadata := schema.Object{Name: name}
		for fieldName, field := range object.Definition.Fields {
			fieldType := string(field.Type)
			if field.DisplayType != "" {
				fieldType = field.DisplayType
			}
			if field.Type == storage.FieldReference {
				fieldType = "Lookup"
			}
			metadata.Fields = append(metadata.Fields, schema.Field{Name: fieldName, Type: fieldType,
				ReferenceTo: append([]string(nil), field.ReferenceTo...), RelationshipName: field.RelationshipName,
				ChildRelationshipName: field.ChildRelationshipName})
		}
		sort.Slice(metadata.Fields, func(i, j int) bool { return metadata.Fields[i].Name < metadata.Fields[j].Name })
		snapshot.Objects = append(snapshot.Objects, metadata)
	}
	sort.Slice(snapshot.Objects, func(i, j int) bool { return snapshot.Objects[i].Name < snapshot.Objects[j].Name })
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "v14-schema.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	// The loaders accept schema.Schema JSON and project snapshot bindings;
	// glade.yml has no root version key. Validate both loaders before compiling
	// so a malformed fixture config cannot appear as a native compile mismatch.
	config := fmt.Sprintf("project:\n  schemaSnapshot: v14-schema.json\n  schemaSnapshotSHA256: %x\n", sha256.Sum256(data))
	if err := os.WriteFile(filepath.Join(root, "glade.yml"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatalf("load V14 compile schema config: %v", err)
	}
	if p.SchemaSnapshot != filepath.Join(root, "v14-schema.json") || p.SchemaSnapshotSHA256 != fmt.Sprintf("%x", sha256.Sum256(data)) {
		t.Fatal("V14 compile schema binding was not loaded")
	}
	loaded, err := schema.LoadProject(p)
	if err != nil {
		t.Fatalf("load V14 compile schema snapshot: %v", err)
	}
	if len(loaded.Objects) < len(snapshot.Objects) {
		t.Fatalf("V14 compile schema loaded %d objects; snapshot has %d", len(loaded.Objects), len(snapshot.Objects))
	}
}

func v14ExecuteBatch(t *testing.T, api string, first, last int) ([]v14Result, error) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(t.TempDir(), "observed.jsonl")
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, "-test.run=^TestV14SalesforceConformance$", "-test.timeout=9m", "-test.v")
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("GLADE_V14_BATCH=%s:%d:%d", api, first, last),
		"GLADE_V14_BATCH_RESULT="+path)
	output, runErr := cmd.CombinedOutput()
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("V14 API %s rows %d:%d: %v; %v\n%s", api, first, last, err, runErr, output)
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	results := make([]v14Result, 0, last-first)
	for {
		var result v14Result
		if err := decoder.Decode(&result); err != nil {
			if err != io.EOF {
				return results, fmt.Errorf("V14 API %s rows %d:%d: decode: %v; %v\n%s", api, first, last, err, runErr, output)
			}
			break
		}
		if len(results) == last-first || result.API != api || result.ID == "" || result.Root == "" || result.Diagnostics == nil {
			return results, fmt.Errorf("V14 API %s rows %d:%d: invalid batch observation", api, first, last)
		}
		results = append(results, result)
	}
	if runErr != nil || len(results) != last-first {
		return results, fmt.Errorf("V14 API %s rows %d:%d: received %d observations; %v\n%s", api, first, last, len(results), runErr, output)
	}
	return results, nil
}

// Reuse V04's positive administrator profile, with synthetic local identities.
// The Account values are capture inputs, independent of every expected answer.
func v14Org(accountFields map[string]string) (storage.OrgState, storage.ID) {
	org, user := v04Org()
	storage.EnsureDeterministicPlatformData(&org)
	account := org.Objects["Account"]
	const recordID storage.ID = "001000000000004AAA"
	record := account.Records[recordID]
	for field, value := range accountFields {
		record.Fields[field] = storage.StringValue(value)
	}
	account.Records[recordID] = record
	org.Objects["Account"] = account
	return org, user.ID
}

func v14Render(t *testing.T, root, name string, c v14Case, accountFields map[string]string) string {
	t.Helper()
	p, err := project.Load(root)
	if err != nil {
		return v14LocalError(t, err)
	}
	sch, err := schema.LoadProject(p)
	if err != nil {
		return v14LocalError(t, err)
	}
	source, err := server.NewSourceMetadataFromProject(p)
	if err != nil {
		return v14LocalError(t, err)
	}
	org, principal := v14Org(accountFields)
	handler := server.NewWithSource(&org, source)
	handler.VisualforceHTMLUserID = principal
	handler.SetProjectIndex(typesys.Build(p, sch))
	query := "?principal=" + url.QueryEscape(string(principal)) + "&ownedId=001000000000004AAA" + strings.ReplaceAll(c.Observation.Query, "__OWNED_RECORD__", "001000000000004AAA")
	get := httptest.NewRequest(http.MethodGet, "https://v14.example.test/apex/"+name+query, nil)
	for key, value := range c.Observation.RequestHeaders {
		get.Header.Set(key, value)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, get)
	doc, err := html.Parse(strings.NewReader(response.Body.String()))
	if err != nil {
		return v14LocalError(t, err)
	}
	if c.Observation.Interaction != "" {
		button := v04Find(doc, func(n *html.Node) bool { return n.Data == "input" && v04Attr(n, "value") == "V14_SUBMIT" })
		var form *html.Node
		for parent := button; parent != nil; parent = parent.Parent {
			if parent.Data == "form" {
				form = parent
				break
			}
		}
		if button == nil || form == nil {
			return v14LocalError(t, fmt.Errorf("rendered submit control/form missing"))
		}
		values := url.Values{}
		v04Walk(form, func(n *html.Node) {
			if n.Type != html.ElementNode || (n.Data != "input" && n.Data != "textarea" && n.Data != "select" && n.Data != "button") || v04HasAttr(n, "disabled") {
				return
			}
			kind := v04Attr(n, "type")
			if ((kind == "checkbox" || kind == "radio") && !v04HasAttr(n, "checked")) ||
				(c.Observation.Interaction != "ajax" && (kind == "submit" || kind == "button" || kind == "reset") && n != button) {
				return
			}
			if name := v04Attr(n, "name"); name != "" {
				value := v04ControlValue(n)
				if n.Data == "textarea" {
					value = strings.ReplaceAll(strings.ReplaceAll(value, "\r\n", "\n"), "\r", "\n")
					value = strings.ReplaceAll(value, "\n", "\r\n")
				}
				values.Add(name, value)
			}
		})
		values.Set(visualforce.ViewStateActionFieldName(), v04Attr(button, "data-action"))
		if c.Observation.Interaction == "ajax" {
			hook := v14AjaxHook.FindStringSubmatch(v04Attr(button, "onclick"))
			if len(hook) != 2 {
				return v14LocalError(t, fmt.Errorf("rendered AJAX submit hook missing"))
			}
			values.Set("__vf_ajax", "1")
			values.Set("__vf_rerender", hook[1])
		}
		postURL, err := get.URL.Parse(v04Attr(form, "action"))
		if err != nil {
			return v14LocalError(t, err)
		}
		post := httptest.NewRequest(http.MethodPost, postURL.String(), strings.NewReader(values.Encode()))
		post.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		for key, value := range c.Observation.RequestHeaders {
			post.Header.Set(key, value)
		}
		posted := httptest.NewRecorder()
		handler.ServeHTTP(posted, post)
		if c.Observation.ActionResponse {
			// The self-review capture observes the completed interaction's
			// response; accepted earlier observers retain their initial GET.
			response = posted
		}
		if c.Observation.Interaction == "ajax" {
			var partial visualforce.PartialResponse
			if err := json.Unmarshal(posted.Body.Bytes(), &partial); err != nil {
				return v14LocalError(t, err)
			}
			for id, markup := range partial.Targets {
				target := v04Find(doc, func(n *html.Node) bool { return v04Attr(n, "id") == id })
				fragment, err := html.Parse(strings.NewReader(markup))
				if err != nil {
					return v14LocalError(t, err)
				}
				replacement := v04Find(fragment, func(n *html.Node) bool { return v04Attr(n, "id") == id })
				if target == nil || target.Parent == nil || replacement == nil {
					return v14LocalError(t, fmt.Errorf("rendered partial target %q missing", id))
				}
				replacement.Parent.RemoveChild(replacement)
				target.Parent.InsertBefore(replacement, target)
				target.Parent.RemoveChild(target)
			}
		} else {
			response = posted
			doc, err = html.Parse(strings.NewReader(posted.Body.String()))
			if err != nil {
				return v14LocalError(t, err)
			}
		}
	}
	return v14ObserveDOM(t, doc, response, get, c)
}

var v14AjaxHook = regexp.MustCompile(`GLADEVF\.submit\([^,]+,'[^']*','([^']*)'`)
var v14ExceptionType = regexp.MustCompile(`\b(?:System\.)?[A-Za-z]*Exception\b`)
var v14CSRFMention = regexp.MustCompile(`(?i)\bCSRF\b|cross[- ]site request forgery`)

func v14ObserveDOM(t *testing.T, doc *html.Node, response *httptest.ResponseRecorder, request *http.Request, c v14Case) string {
	t.Helper()
	root := v04Find(doc, func(n *html.Node) bool { return v04Attr(n, "id") == "v14-root" })
	observed := map[string]any{"rootPresent": root != nil}
	if root == nil {
		title := v04Find(doc, func(n *html.Node) bool { return n.Data == "title" })
		body := v04Find(doc, func(n *html.Node) bool { return n.Data == "body" })
		if body == nil {
			body = doc
		}
		text := v04Text(body)
		types, seen := []string{}, map[string]bool{}
		for _, name := range v14ExceptionType.FindAllString(text, -1) {
			if !seen[name] {
				types = append(types, name)
				seen[name] = true
			}
		}
		sort.Strings(types)
		observed["titleHasError"], observed["exceptionTypes"] = strings.Contains(strings.ToLower(v04Text(title)), "error"), types
		// The captured observer retained these facts for a completed protected
		// GET without a controller root. They come only from the actual response.
		if c.Observation.ProtectedGET {
			observed["httpStatus"] = response.Code
			observed["csrfMention"] = v14CSRFMention.MatchString(text)
			observed["invalidSession"] = strings.Contains(strings.ToLower(text), "page you submitted was invalid for your session")
		}
	} else {
		value := v04Find(root, func(n *html.Node) bool { return v04Attr(n, "id") == "v14-value" })
		if value == nil {
			value = root
		}
		observed["text"] = v04Text(value)
		fields := []string{}
		v04Walk(root, func(n *html.Node) {
			if n.Type != html.ElementNode || (n.Data != "input" && n.Data != "select" && n.Data != "textarea") {
				return
			}
			kind := v04Attr(n, "type")
			if n.Data == "input" && (kind == "hidden" || kind == "submit") {
				return
			}
			fields = append(fields, v04ControlValue(n))
		})
		observed["fields"] = fields
	}
	if c.Observation.ValueOnly && root != nil {
		observed = map[string]any{"rootPresent": true, "text": observed["text"]}
	}
	if key := c.Observation.ResponseHeader; key != "" {
		_, present := response.Header()[http.CanonicalHeaderKey(key)]
		var value any
		if present {
			value = response.Header().Get(key)
		}
		switch c.Observation.HeaderReduction {
		case "presence":
			value = present
		case "owned-or-presence":
			value = map[string]bool{"present": present, "owned": value == "default-src 'self'"}
		case "csp-keywords":
			if present {
				value = v14CSPKeywords(response.Header().Get(key))
			}
		}
		observed["responseHeader"] = value
	}
	if c.Observation.ObserveCookie {
		cookies := []any{}
		for _, cookie := range response.Result().Cookies() {
			if cookie.Name != "apex__V14Owned" || cookie.MaxAge < 0 || (!cookie.Expires.IsZero() && cookie.Expires.Before(time.Now())) {
				continue
			}
			path := cookie.Path
			if path == "" {
				path = request.URL.Path[:strings.LastIndex(request.URL.Path, "/")]
				if path == "" {
					path = "/"
				}
			}
			observedCookie := map[string]any{"name": "V14Owned", "value": cookie.Value, "path": path, "secure": cookie.Secure, "httpOnly": cookie.HttpOnly}
			if c.Observation.CookieLifetime {
				expiry := "session"
				if cookie.MaxAge > 0 || !cookie.Expires.IsZero() {
					expiry = "persistent"
				}
				observedCookie["expiry"] = expiry
			}
			cookies = append(cookies, observedCookie)
		}
		observed["cookies"] = cookies
	}
	if c.Observation.ObserveScheme {
		observed["https"] = request.URL.Scheme == "https"
	}
	return "DOM|" + v04Encode(t, observed)
}

var v14CSPNonce = regexp.MustCompile(`^'nonce-[^']+'$`)

// Mirror the native control observer's semicolon/whitespace parsing. Preserve
// its literal directive names, including combined-header commas, and raw null
// for an absent header. No Salesforce hosts or nonce values seed this observer.
func v14CSPKeywords(header string) map[string]map[string]bool {
	policy := map[string]map[string]bool{}
	for _, directive := range strings.Split(header, ";") {
		words := strings.Fields(directive)
		if len(words) == 0 {
			continue
		}
		facts := map[string]bool{"self": false, "none": false, "unsafeInline": false, "unsafeEval": false, "nonce": false, "https": false, "data": false}
		for _, source := range words[1:] {
			switch source {
			case "'self'":
				facts["self"] = true
			case "'none'":
				facts["none"] = true
			case "'unsafe-inline'":
				facts["unsafeInline"] = true
			case "'unsafe-eval'":
				facts["unsafeEval"] = true
			case "data:":
				facts["data"] = true
			}
			facts["nonce"] = facts["nonce"] || v14CSPNonce.MatchString(source)
			facts["https"] = facts["https"] || strings.HasPrefix(source, "https:")
		}
		policy[strings.ToLower(words[0])] = facts
	}
	return policy
}

func v14LocalError(t *testing.T, err error) string {
	t.Helper()
	return "DOM|" + v04Encode(t, map[string]string{"localError": err.Error()})
}
