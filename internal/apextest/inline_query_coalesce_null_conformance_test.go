package apextest

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

type queryNullCompileAnswer struct {
	Outcome     string                     `json:"outcome"`
	Diagnostics []nativeBoundaryDiagnostic `json:"diagnostics,omitempty"`
	CarryReason string                     `json:"carryReason,omitempty"`
}

type queryNullRuntimeRow struct {
	ID            string                                          `json:"id"`
	TestMethod    string                                          `json:"testMethod"`
	ExpectedByAPI map[string]map[string]nativeObservationExpected `json:"expectedByAPI"`
}

// The anonymous batch and named methods retain the captured source. Native
// deployment acceptance is distinct from the twelve evaluated query results.
func TestInlineQueryCoalesceNullOrgConformance(t *testing.T) {
	var data struct {
		APIVersions                   []string                       `json:"apiVersions"`
		NativeCompileObservationCount int                            `json:"nativeCompileObservationCount"`
		NativeRuntimeObservationCount int                            `json:"nativeRuntimeObservationCount"`
		SourcesByAPI                  map[string]nativeBoundaryRoute `json:"sourcesByAPI"`
		Carries                       []struct {
			APIVersion string `json:"apiVersion"`
			Reason     string `json:"reason"`
		} `json:"carries"`
		CompileRows []struct {
			SourceRef      string                            `json:"sourceRef,omitempty"`
			ObservedMethod string                            `json:"observedMethod,omitempty"`
			ID             string                            `json:"id"`
			SourcesByAPI   map[string]nativeBoundaryRoute    `json:"sourcesByAPI"`
			ExpectedByAPI  map[string]queryNullCompileAnswer `json:"expectedByAPI"`
		} `json:"compileRows"`
		RuntimeRows []queryNullRuntimeRow `json:"runtimeRows"`
	}
	raw, err := os.ReadFile("testdata/conformance/inline_query_coalesce_null.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIVersions, ",") != "62.0,64.0,67.0" || len(data.CompileRows) != 7 || len(data.RuntimeRows) != 12 || data.NativeCompileObservationCount != 21 || data.NativeRuntimeObservationCount != 72 {
		t.Fatal("incomplete inline-query null-coalescing matrix")
	}
	for _, api := range data.APIVersions {
		source, ok := data.SourcesByAPI[api]
		if !ok {
			t.Fatalf("missing captured source for API %s", api)
		}
		nativeBoundaryCheckSources(t, source)
	}
	for _, carry := range data.Carries {
		if carry.APIVersion == "" || carry.Reason == "" {
			t.Fatal("invalid uncaptured API carry")
		}
		t.Logf("carried API %s: %s", carry.APIVersion, carry.Reason)
	}
	seen := map[string]bool{}
	for _, row := range data.CompileRows {
		if row.ID == "" || seen[row.ID] || len(row.ExpectedByAPI) != len(data.APIVersions) {
			t.Fatalf("invalid compile row: %s", row.ID)
		}
		seen[row.ID] = true
		for _, api := range data.APIVersions {
			source, ok := row.SourcesByAPI[api]
			if row.SourceRef == "matrix" {
				source, ok = data.SourcesByAPI[api]
				foundMethod := false
				for _, text := range source.Classes {
					foundMethod = foundMethod || row.ObservedMethod != "" && strings.Contains(text, " "+row.ObservedMethod+"(")
				}
				if !foundMethod {
					t.Fatalf("missing captured compile method: %s/%s", row.ID, api)
				}
			}
			if !ok {
				t.Fatalf("missing compile source for %s/%s", row.ID, api)
			}
			nativeBoundaryCheckSources(t, source)
		}
	}
	for _, row := range data.RuntimeRows {
		if row.ID == "" || seen[row.ID] || row.TestMethod == "" || len(row.ExpectedByAPI) != len(data.APIVersions) {
			t.Fatalf("invalid runtime row: %s", row.ID)
		}
		seen[row.ID] = true
	}
	for _, api := range data.APIVersions {
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			for _, row := range data.CompileRows {
				t.Run(row.ID, func(t *testing.T) {
					want, ok := row.ExpectedByAPI[api]
					if !ok {
						t.Fatal("missing native compile cell")
					}
					if want.Outcome == "unknown" {
						if want.CarryReason == "" {
							t.Fatal("unknown compile cell needs a carry reason")
						}
						t.Logf("carried native compile observation: %s", want.CarryReason)
						return
					}
					source := row.SourcesByAPI[api]
					if row.SourceRef == "matrix" {
						source = data.SourcesByAPI[api]
					}
					index := nativeBoundaryIndex(t, api, source.Classes, nil)
					analysis := sema.Analyze(index)
					rejected := index.HasErrors() || analysis.HasErrors()
					switch want.Outcome {
					case "accepted":
						if rejected {
							t.Fatalf("native accepted class; Glade diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
						}
					case "rejected":
						if !rejected || len(want.Diagnostics) == 0 {
							t.Fatalf("native rejected class; Glade diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
						}
						// This row asserts rejection, not unobserved diagnostic parity.
						t.Logf("native compile rejection retained: %v; diagnostic text is not asserted", want.Diagnostics)
					default:
						t.Fatalf("invalid compile outcome: %s", want.Outcome)
					}
				})
			}
			counts := newConformanceCountLog(t, "InlineQueryCoalesceNull", api)
			t.Run("anonymous", func(t *testing.T) {
				source := data.SourcesByAPI[api]
				index := nativeBoundaryIndex(t, api, source.Classes, nil)
				analysis := sema.Analyze(index)
				anonymous := sema.AnalyzeAnonymous(index, source.Anonymous, api)
				if index.HasErrors() || analysis.HasErrors() || anonymous.HasErrors() {
					t.Fatalf("native executes; Glade diagnostics=%v %v %v", index.Diagnostics, analysis.Diagnostics, anonymous.Diagnostics)
				}
				program, err := vm.CompileAnonymousWithOptions(source.Anonymous, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				org.APIVersion = api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				result, err := runner.execute(program)
				if err != nil {
					t.Fatal(err)
				}
				observations := map[string]json.RawMessage{}
				for _, line := range result.Debug {
					if !strings.HasPrefix(line, "NATIVE_RESULT|") {
						continue
					}
					id, value, ok := strings.Cut(strings.TrimPrefix(line, "NATIVE_RESULT|"), "|")
					if !ok || observations[id] != nil {
						t.Fatalf("invalid or repeated observation: %s", line)
					}
					observations[id] = json.RawMessage(value)
				}
				if len(observations) != len(data.RuntimeRows) {
					t.Fatalf("anonymous observations=%d, want %d", len(observations), len(data.RuntimeRows))
				}
				for _, row := range data.RuntimeRows {
					t.Run(row.ID, func(t *testing.T) {
						counts.TrackKind(t, "anonymous", "exact", 1)
						want := row.ExpectedByAPI[api]["anonymous"]
						var actual, expected map[string]any
						if want.Outcome != "runtime" || json.Unmarshal(want.Value, &expected) != nil || len(expected) == 0 || json.Unmarshal(observations[row.ID], &actual) != nil {
							t.Fatal("missing native or local runtime JSON")
						}
						if !reflect.DeepEqual(expected, actual) {
							t.Fatalf("native=%s; Glade=%s", want.Value, observations[row.ID])
						}
					})
				}
			})
			t.Run("isTest", func(t *testing.T) {
				for _, row := range data.RuntimeRows {
					t.Run(row.ID, func(t *testing.T) {
						counts.TrackKind(t, "isTest", "exact", 1)
						source := data.SourcesByAPI[api]
						source.ID, source.TestMethod = row.ID, row.TestMethod
						residualObservationObserve(t, api, "isTest", source, row.ExpectedByAPI[api]["isTest"])
					})
				}
			})
		})
	}
}
