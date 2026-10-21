package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// These owned observations cover only the captured named tests and API versions.
// The assertion is the observation transport, not a native test failure claim.
func TestBatchFinishLifecycleNativeConformance(t *testing.T) {
	var data struct {
		APIVersions, Routes, Variants      []string
		ShapeCount, NativeObservationCount int
		Capture                            struct{ Transport, AnonymousBoundary string }
		Fixtures                           map[string]struct {
			Classes, SourceSHA256    map[string]string
			Metadata, MetadataSHA256 map[string]map[string]string
		}
		CarriedPhases map[string]string
		Cells         []struct {
			API, Route, Variant, Method, RawRow, RawRowSHA256 string
			Native                                            []any
		}
	}
	raw, err := os.ReadFile("testdata/conformance/batch_finish_lifecycle.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	methods := []string{"directScheduleAbortChild", "directScheduleKeepChild", "finishAbortChild", "finishKeepChild", "finishScheduleOnly", "guardedEmpty", "guardedOne", "plainEmpty"}
	if strings.Join(data.APIVersions, ",") != "58.0,62.0,67.0,68.0" || strings.Join(data.Routes, ",") != "istest" || data.ShapeCount != len(methods) || strings.Join(data.Variants, ",") != "default,explicit100" || data.NativeObservationCount != 64 || len(data.Cells) != 64 || len(data.Fixtures) != 2 {
		t.Fatal("incomplete native batch finish matrix")
	}
	if data.Capture.Transport == "" || data.Capture.AnonymousBoundary == "" || len(data.CarriedPhases) != 2 || data.CarriedPhases["00"] == "" || data.CarriedPhases["01"] == "" {
		t.Fatal("missing observation boundary or worker-phase carry reason")
	}
	t.Logf("anonymous boundary: %s", data.Capture.AnonymousBoundary)
	checkHash := func(value, want string) {
		t.Helper()
		sum := sha256.Sum256([]byte(value))
		if value == "" || hex.EncodeToString(sum[:]) != want {
			t.Fatal("changed owned source or raw native row")
		}
	}
	for _, variant := range data.Variants {
		fixture := data.Fixtures[variant]
		if len(fixture.Classes) != 2 || fixture.Classes["CyclePhaseBatch"] == "" || fixture.Classes["CyclePhaseObservation"] == "" {
			t.Fatal("missing owned declarations")
		}
		for name, source := range fixture.Classes {
			checkHash(source, fixture.SourceSHA256[name])
			for _, api := range data.APIVersions {
				metadata := fixture.Metadata[api][name]
				checkHash(metadata, fixture.MetadataSHA256[api][name])
				var declaration struct {
					APIVersion string `xml:"apiVersion"`
				}
				if xml.Unmarshal([]byte(metadata), &declaration) != nil || declaration.APIVersion != api {
					t.Fatalf("native source metadata differs: %s/%s/%s", variant, api, name)
				}
			}
		}
	}
	seen := map[string]bool{}
	firstAPI := map[string][]any{}
	for _, cell := range data.Cells {
		key := cell.API + "/" + cell.Route + "/" + cell.Variant + "/" + cell.Method
		if seen[key] || cell.Route != "istest" {
			t.Fatalf("duplicate or unexpected native cell: %s", key)
		}
		seen[key] = true
		checkHash(cell.RawRow, cell.RawRowSHA256)
		method, payload, found := strings.Cut(strings.TrimSuffix(cell.RawRow, "\n"), "\t")
		var fromRaw []any
		if !found || method != cell.Method || json.Unmarshal([]byte(payload), &fromRaw) != nil || !reflect.DeepEqual(fromRaw, cell.Native) {
			t.Fatalf("native row differs from raw observation: %s", key)
		}
		if previous, ok := firstAPI[cell.Variant+"/"+cell.Method]; ok {
			if !reflect.DeepEqual(previous, cell.Native) {
				t.Fatalf("captured API cells differ: %s", key)
			}
		} else {
			firstAPI[cell.Variant+"/"+cell.Method] = cell.Native
		}
	}
	for _, api := range data.APIVersions {
		for _, variant := range data.Variants {
			for _, method := range methods {
				if !seen[api+"/istest/"+variant+"/"+method] {
					t.Fatalf("missing native cell: %s/%s/%s", api, variant, method)
				}
			}
		}
	}
	// Replace only the job snapshot of carried phases. Their position, occurrence,
	// shape, cron states and pending counts remain in the comparison.
	project := func(t *testing.T, native []any) []any {
		t.Helper()
		if len(native) != 5 {
			t.Fatalf("invalid observation shape: %v", native)
		}
		phases, ok := native[4].([]any)
		if !ok {
			t.Fatalf("invalid phase list: %v", native[4])
		}
		result := append([]any(nil), native...)
		projected := make([]any, len(phases))
		for i, value := range phases {
			phase, ok := value.([]any)
			if !ok || len(phase) != 4 {
				t.Fatalf("invalid phase snapshot: %v", value)
			}
			name, ok := phase[0].(string)
			if !ok {
				t.Fatalf("invalid phase name: %v", phase[0])
			}
			copy := append([]any(nil), phase...)
			if _, carried := data.CarriedPhases[name]; carried {
				copy[1] = "<carried-job-snapshot>"
			}
			projected[i] = copy
		}
		result[4] = projected
		return result
	}
	for _, api := range data.APIVersions {
		for _, variant := range data.Variants {
			t.Run(api+"/"+variant, func(t *testing.T) {
				counts := newConformanceCountLog(t, "BatchFinishLifecycle/"+variant, api)
				index := nativeBoundaryIndex(t, api, data.Fixtures[variant].Classes, nil)
				analysis := sema.Analyze(index)
				if index.HasErrors() || analysis.HasErrors() {
					t.Fatalf("native declarations accepted: %v %v", index.Diagnostics, analysis.Diagnostics)
				}
				cases := Discover(index, Options{SelectedClasses: []string{"CyclePhaseObservation"}})
				if len(cases) != data.ShapeCount {
					t.Fatalf("named discovery: %d rows", len(cases))
				}
				compiled, methodErrors := compileTestMethodsForIndex(&index, cases)
				programs, programErrors := compileTestInvokePrograms(cases)
				org := orgFromIndex(index)
				org.APIVersion = api
				base := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				for _, cell := range data.Cells {
					if cell.API != api || cell.Variant != variant {
						continue
					}
					kind := "exact"
					want := project(t, cell.Native)
					if !reflect.DeepEqual(want, cell.Native) {
						kind = "partial"
					}
					counts.RunKind(t, "isTest", cell.Method, cell.Method, kind, func(t *testing.T) {
						for _, value := range cell.Native[4].([]any) {
							phase := value.([]any)
							if reason := data.CarriedPhases[phase[0].(string)]; reason != "" {
								payload, _ := json.Marshal(phase)
								t.Logf("carried phase %s: %s native=%s", phase[0], reason, payload)
							}
						}
						machine := base.newMachine()
						machine.EnableTestContext()
						var execErr error
						matched := 0
						for _, test := range cases {
							if test.MethodName != cell.Method {
								continue
							}
							matched++
							key := testCaseKey(test)
							if methodErrors[key] != nil || programErrors[key] != nil {
								t.Fatalf("native executes: method=%v program=%v", methodErrors[key], programErrors[key])
							}
							if err := machine.RegisterMethod(compiled[key]); err != nil {
								t.Fatal(err)
							}
							_, execErr = machine.ExecuteInClass(programs[key], test.ClassName)
						}
						if matched != 1 {
							t.Fatalf("expected one named entry, got %d", matched)
						}
						var fault *vm.RuntimeError
						if !errors.As(execErr, &fault) || !strings.HasSuffix(fault.Type, "AssertException") || !strings.Contains(fault.ExceptionMessage(), "NATIVE_OBSERVATION") {
							t.Fatalf("missing assertion observation: %v", execErr)
						}
						_, payload, found := strings.Cut(fault.ExceptionMessage(), "Actual: ")
						var got []any
						if !found || json.Unmarshal([]byte(payload), &got) != nil {
							t.Fatalf("invalid observation payload: %v", execErr)
						}
						if !reflect.DeepEqual(project(t, got), want) {
							native, _ := json.Marshal(cell.Native)
							t.Fatalf("native=%s; local=%s", native, payload)
						}
					})
				}
			})
		}
	}
}
