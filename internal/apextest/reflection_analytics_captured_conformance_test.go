package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

type reflectionAnalyticsCell struct {
	ID, API, Route, Context, Marker, Outcome, Primary, ExceptionType, Message, CarryReason string
	HashedEnvelope                                                                         bool
	Source                                                                                 struct {
		Anonymous string
		Classes   map[string]string
		Tests     []string
	}
	SourceSHA256 map[string]string
	Value        json.RawMessage
	Native       json.RawMessage
}

func runReflectionAnalyticsCaptured(t *testing.T, family string, matched, carried int) {
	t.Helper()
	var data struct {
		CellCount, MatchedCount, CarriedCount int
		Cells                                 []reflectionAnalyticsCell
	}
	raw, err := os.ReadFile(filepath.Join("testdata/conformance/reflection_analytics", family+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data.CellCount != matched+carried || len(data.Cells) != data.CellCount || data.MatchedCount != matched || data.CarriedCount != carried {
		t.Fatal("incomplete captured matrix")
	}
	seen := map[string]bool{}
	actualMatched, actualCarried := 0, 0
	for _, cell := range data.Cells {
		key := cell.Context + "/" + cell.API + "/" + cell.Route + "/" + cell.ID
		if cell.ID == "" || seen[key] || !json.Valid(cell.Native) {
			t.Fatalf("invalid native cell: %s", key)
		}
		seen[key] = true
		values := map[string]string{}
		if cell.Source.Anonymous != "" {
			values["anonymous"] = cell.Source.Anonymous
		}
		for name, source := range cell.Source.Classes {
			values["class:"+name] = source
		}
		if len(values) == 0 || len(values) != len(cell.SourceSHA256) {
			t.Fatal("missing captured source")
		}
		for name, source := range values {
			sum := sha256.Sum256([]byte(source))
			if hex.EncodeToString(sum[:]) != cell.SourceSHA256[name] {
				t.Fatalf("source changed: %s", key)
			}
		}
		if cell.CarryReason != "" {
			actualCarried++
		} else {
			actualMatched++
		}
		t.Run(key, func(t *testing.T) {
			counts := newConformanceCountLog(t, "ReflectionAnalytics-"+family, cell.API)
			if cell.CarryReason != "" {
				counts.TrackKind(t, cell.Route, "carried", 1)
				t.Logf("carried: %s; native=%s", cell.CarryReason, cell.Native)
				return
			}
			kind := "exact"
			if cell.Outcome == "reject" {
				kind = "category"
			}
			counts.TrackKind(t, cell.Route, kind, 1)
			index := reflectionAnalyticsIndex(t, cell)
			org := orgFromIndex(index)
			org.APIVersion = cell.API
			analysis := sema.Analyze(index)
			diagnostics := append(append([]diagnostic.Diagnostic{}, index.Diagnostics...), analysis.Diagnostics...)
			want := listSObjectCastExpected{Outcome: cell.Outcome, Primary: cell.Primary}
			var execErr error
			if cell.Route == "anonymous" {
				anonymous := sema.AnalyzeAnonymous(index, cell.Source.Anonymous, cell.API)
				diagnostics = append(diagnostics, anonymous.Diagnostics...)
				program, compileErr := vm.CompileAnonymousWithOptions(cell.Source.Anonymous, vm.CompileOptions{APIVersion: cell.API})
				listSObjectCastCompile(t, want, diagnostics, compileErr)
				if cell.Outcome == "reject" {
					return
				}
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				_, execErr = runner.execute(program)
			} else if cell.Route == "istest" {
				listSObjectCastCompile(t, want, diagnostics, nil)
				if cell.Outcome == "reject" {
					return
				}
				cases := Discover(index, Options{SelectedClasses: cell.Source.Tests, SelectedMethod: "observe"})
				if len(cases) != 1 {
					t.Fatalf("named observation discovery: %d", len(cases))
				}
				methods, methodErrors := compileTestMethodsForIndex(&index, cases)
				programs, programErrors := compileTestInvokePrograms(cases)
				key := testCaseKey(cases[0])
				if methodErrors[key] != nil || programErrors[key] != nil {
					t.Fatalf("named compile: %v %v", methodErrors[key], programErrors[key])
				}
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				machine := runner.newMachine()
				if err := machine.RegisterMethod(methods[key]); err != nil {
					t.Fatal(err)
				}
				machine.EnableTestContext()
				_, execErr = machine.ExecuteInClass(programs[key], cases[0].ClassName)
			} else {
				t.Fatalf("invalid route: %s", cell.Route)
			}
			reflectionAnalyticsValue(t, cell, execErr)
		})
	}
	if actualMatched != matched || actualCarried != carried {
		t.Fatal("captured dispositions differ")
	}
}

func reflectionAnalyticsIndex(t *testing.T, cell reflectionAnalyticsCell) typesys.Index {
	t.Helper()
	namespace := ""
	if cell.Context == "sameNamespace" {
		namespace = "sample"
	} else if cell.Context != "unpackaged" {
		t.Fatalf("invalid context: %s", cell.Context)
	}
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"namespace":%q,"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, namespace, cell.API))
	for name, source := range cell.Source.Classes {
		path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
		writeFile(t, path, source)
		writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+cell.API+"</apiVersion></ApexClass>")
	}
	return loadTestIndex(t, root)
}

func reflectionAnalyticsValue(t *testing.T, cell reflectionAnalyticsCell, execErr error) {
	t.Helper()
	var runtimeErr *vm.RuntimeError
	if !errors.As(execErr, &runtimeErr) {
		t.Fatalf("missing native observation: %v", execErr)
	}
	message := runtimeErr.ExceptionMessage()
	if cell.Outcome == "uncaught" {
		if listSObjectCastExceptionType(runtimeErr.Type) != cell.ExceptionType || message != cell.Message {
			t.Fatalf("native <%s: %s>; Glade <%s: %s>", cell.ExceptionType, cell.Message, runtimeErr.Type, message)
		}
		return
	}
	if cell.Outcome != "runtime" || listSObjectCastExceptionType(runtimeErr.Type) != "System.AssertException" {
		t.Fatalf("missing assertion envelope: %v", execErr)
	}
	_, payload, ok := strings.Cut(message, cell.Marker+cell.ID+"|")
	if !ok || strings.Count(message, cell.Marker) != 1 {
		t.Fatalf("invalid observation envelope: %s", message)
	}
	if cell.HashedEnvelope {
		digest, rest, ok := strings.Cut(payload, "|")
		sum := sha256.Sum256([]byte(rest))
		if !ok || digest != hex.EncodeToString(sum[:]) {
			t.Fatal("observation payload digest differs")
		}
		payload = rest
	}
	var got, want map[string]any
	if err := json.Unmarshal([]byte(payload), &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(cell.Value, &want); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"exceptionType", "caught"} {
		if name, ok := got[key].(string); ok {
			got[key] = listSObjectCastExceptionType(name)
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("native <%s>; Glade <%s>", cell.Value, payload)
	}
}

func TestReflectedListTokenCapturedConformance(t *testing.T) {
	runReflectionAnalyticsCaptured(t, "reflection", 140, 6)
}
func TestAppAnalyticsCapturedConformance(t *testing.T) {
	runReflectionAnalyticsCaptured(t, "analytics", 158, 0)
}
func TestReflectionAnalyticsTransportControlsConformance(t *testing.T) {
	runReflectionAnalyticsCaptured(t, "controls", 52, 0)
}
