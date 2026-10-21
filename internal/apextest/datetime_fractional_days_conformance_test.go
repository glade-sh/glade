package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

type datetimeFractionalDaysContext struct {
	SourceAPI    string   `json:"sourceApi"`
	TransportAPI string   `json:"transportApi"`
	Route        string   `json:"route"`
	Values       []string `json:"values"`
}

// Literal, Decimal, Double and Integer operands retain their independent native
// results in each source API and execution route.
func TestDatetimeFractionalDaysConformance(t *testing.T) {
	const fixture = "testdata/conformance/datetime_fractional_days"
	var data struct {
		Files []struct {
			Path   string `json:"path"`
			SHA256 string `json:"sha256"`
		} `json:"files"`
		Contexts []datetimeFractionalDaysContext `json:"contexts"`
	}
	raw, err := os.ReadFile(filepath.Join(fixture, "observations.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Files) != 2 || len(data.Contexts) != 8 {
		t.Fatal("incomplete fractional-day observations")
	}
	sources := map[string]string{}
	for _, file := range data.Files {
		if (file.Path != "setup/FractionalDayObservation.cls" && file.Path != "setup/FractionalDayObservationTest.cls") || sources[file.Path] != "" {
			t.Fatalf("invalid or repeated fractional-day source: %s", file.Path)
		}
		body, err := os.ReadFile(filepath.Join(fixture, filepath.FromSlash(file.Path)))
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(body)
		if len(body) == 0 || hex.EncodeToString(digest[:]) != file.SHA256 {
			t.Fatalf("captured fractional-day source changed: %s", file.Path)
		}
		sources[file.Path] = string(body)
	}
	expected := map[string]bool{}
	for _, api := range []string{"62.0", "66.0", "67.0", "68.0"} {
		for _, route := range []string{"anonymous", "istest"} {
			expected[api+"/"+route] = true
		}
	}
	seen := map[string]bool{}
	for _, context := range data.Contexts {
		key := context.SourceAPI + "/" + context.Route
		if !expected[key] || seen[key] || context.TransportAPI != context.SourceAPI || len(context.Values) != 8 {
			t.Fatalf("invalid or repeated fractional-day context: %+v", context)
		}
		seen[key] = true
		for _, value := range context.Values {
			if _, err := time.Parse(time.RFC3339Nano, value); err != nil {
				t.Fatalf("invalid native fractional-day timestamp %q: %v", value, err)
			}
		}
	}
	for _, context := range data.Contexts {
		t.Run(context.SourceAPI+"/"+context.Route, func(t *testing.T) {
			datetimeFractionalDaysReplay(t, context, sources)
		})
	}
}

func datetimeFractionalDaysReplay(t *testing.T, context datetimeFractionalDaysContext, sources map[string]string) {
	t.Helper()
	classes := map[string]string{}
	for path, source := range sources {
		classes[strings.TrimSuffix(filepath.Base(path), ".cls")] = source
	}
	index := nativeBoundaryIndex(t, context.SourceAPI, classes, nil)
	for _, typ := range index.Types {
		if _, captured := classes[typ.Name]; captured && typ.EffectiveAPIVersion != context.SourceAPI {
			t.Fatalf("changed fractional-day source pin: %s=%s", typ.Name, typ.EffectiveAPIVersion)
		}
	}
	analysis := sema.Analyze(index)
	if index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("native fractional-day source accepted; local diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
	}
	org := orgFromIndex(index)
	org.APIVersion = context.TransportAPI
	runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
	const marker = "FRACTIONAL_DAYS:"
	var execErr error
	if context.Route == "anonymous" {
		body := "System.assert(false,'" + marker + "'+JSON.serialize(FractionalDayObservation.observe()));"
		checked := sema.AnalyzeAnonymous(index, body, context.SourceAPI)
		program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: context.SourceAPI})
		if checked.HasErrors() || err != nil {
			t.Fatalf("native anonymous fractional-day source accepted; local diagnostics=%v compile=%v", checked.Diagnostics, err)
		}
		_, execErr = runner.execute(program)
	} else {
		cases := Discover(index, Options{SelectedClasses: []string{"FractionalDayObservationTest"}, SelectedMethod: "capture"})
		if len(cases) != 1 {
			t.Fatalf("named fractional-day discovery: %d", len(cases))
		}
		methods, methodErrors := compileTestMethodsForIndex(&index, cases)
		programs, programErrors := compileTestInvokePrograms(cases)
		key := testCaseKey(cases[0])
		if methodErrors[key] != nil || programErrors[key] != nil {
			t.Fatalf("named fractional-day compilation: %v %v", methodErrors[key], programErrors[key])
		}
		machine := runner.newMachine()
		if err := machine.RegisterMethod(methods[key]); err != nil {
			t.Fatal(err)
		}
		machine.EnableTestContext()
		_, execErr = machine.ExecuteInClass(programs[key], cases[0].ClassName)
	}
	var runtimeErr *vm.RuntimeError
	if !errors.As(execErr, &runtimeErr) || runtimeErr.Type != "System.AssertException" {
		t.Fatalf("missing fractional-day observation: %v", execErr)
	}
	message := runtimeErr.ExceptionMessage()
	_, payload, found := strings.Cut(message, marker)
	if !found || strings.Count(message, marker) != 1 {
		t.Fatalf("invalid fractional-day marker: %v", execErr)
	}
	var observed []string
	if err := json.Unmarshal([]byte(payload), &observed); err != nil || len(observed) != 8 {
		t.Fatalf("incomplete fractional-day payload: %s", payload)
	}
	if !reflect.DeepEqual(observed, context.Values) {
		t.Fatalf("native fractional days=%v; local=%v", context.Values, observed)
	}
}
