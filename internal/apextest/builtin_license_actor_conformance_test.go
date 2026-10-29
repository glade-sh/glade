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

// Built-in profile identity and runAs Account facets are independent captured
// cells. The named actor source creates no PermissionSet or grant records.
func TestBuiltinLicenseActorOrgConformance(t *testing.T) {
	var data struct {
		APIVersions            []string                       `json:"apiVersions"`
		NativeObservationCount int                            `json:"nativeObservationCount"`
		SourcesByAPI           map[string]nativeBoundaryRoute `json:"sourcesByAPI"`
		NativeFilesSHA256      map[string]string              `json:"nativeFilesSha256"`
		Rows                   []struct {
			ID                  string                                          `json:"id"`
			TestMethod          string                                          `json:"testMethod"`
			Routes              []string                                        `json:"routes"`
			ExpectedByAPI       map[string]map[string]nativeObservationExpected `json:"expectedByAPI"`
			EvidenceSHA256ByAPI map[string]map[string]string                    `json:"evidenceSha256ByAPI"`
		} `json:"rows"`
		Carries []struct {
			Scope       string          `json:"scope"`
			Reason      string          `json:"reason"`
			NativeValue json.RawMessage `json:"nativeValue"`
		} `json:"carries"`
	}
	raw, err := os.ReadFile("testdata/conformance/builtin_license_actor.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIVersions, ",") != "62.0,64.0,67.0" || len(data.Rows) != 2 || data.NativeObservationCount != 9 || len(data.Carries) != 3 {
		t.Fatal("incomplete built-in license actor matrix")
	}
	for name, sum := range data.NativeFilesSHA256 {
		if name == "" || len(sum) != 64 {
			t.Fatal("missing raw evidence provenance")
		}
	}
	for _, carry := range data.Carries {
		if carry.Scope == "" || carry.Reason == "" || len(carry.NativeValue) == 0 || !json.Valid(carry.NativeValue) {
			t.Fatal("invalid unanswered-cell carry")
		}
		t.Logf("carried %s: %s; native value=%s", carry.Scope, carry.Reason, carry.NativeValue)
	}
	seen := map[string]bool{}
	for _, row := range data.Rows {
		if row.ID == "" || seen[row.ID] || row.TestMethod == "" || len(row.ExpectedByAPI) != 3 || len(row.EvidenceSHA256ByAPI) != 3 {
			t.Fatal("invalid or repeated captured row")
		}
		seen[row.ID] = true
		if row.ID == "PROFILE" && strings.Join(row.Routes, ",") != "anonymous,isTest" || row.ID == "ACTOR" && strings.Join(row.Routes, ",") != "isTest" {
			t.Fatal("captured route boundary changed")
		}
	}
	if !seen["PROFILE"] || !seen["ACTOR"] {
		t.Fatal("missing independent profile or actor row")
	}
	for _, api := range data.APIVersions {
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			source, ok := data.SourcesByAPI[api]
			if !ok {
				t.Fatal("missing captured API source")
			}
			nativeBoundaryCheckSources(t, source)
			counts := newConformanceCountLog(t, "BuiltinLicenseActor", api)
			for _, row := range data.Rows {
				t.Run(row.ID, func(t *testing.T) {
					for _, route := range row.Routes {
						t.Run(route, func(t *testing.T) {
							want, ok := row.ExpectedByAPI[api][route]
							if !ok || want.Outcome != "runtime" || len(row.EvidenceSHA256ByAPI[api][route]) != 64 {
								t.Fatal("missing native runtime cell")
							}
							counts.TrackKind(t, route, "exact", 1)
							source.ID, source.TestMethod = row.ID, row.TestMethod
							if route == "anonymous" {
								builtinLicenseActorAnonymous(t, api, source, want)
							} else {
								residualObservationObserve(t, api, route, source, want)
							}
						})
					}
				})
			}
		})
	}
}

func builtinLicenseActorAnonymous(t *testing.T, api string, source nativeBoundaryRoute, want nativeObservationExpected) {
	t.Helper()
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
	marker := "NATIVE_RESULT|" + source.ID + "|"
	observation := ""
	for _, line := range result.Debug {
		if strings.HasPrefix(line, marker) {
			if observation != "" {
				t.Fatal("repeated captured observation")
			}
			observation = strings.TrimPrefix(line, marker)
		}
	}
	var expected, actual map[string]any
	if observation == "" || json.Unmarshal(want.Value, &expected) != nil || json.Unmarshal([]byte(observation), &actual) != nil {
		t.Fatal("missing or invalid native/local observation")
	}
	if !reflect.DeepEqual(expected, actual) {
		t.Fatalf("native=%s; Glade=%s", want.Value, observation)
	}
}
