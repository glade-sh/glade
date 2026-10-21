package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// Exact sources retain native line numbers and independent route/API answers.
// Capture-desk acceptance is separate from matching the exported observations.
func TestHierarchyAndExceptionNativeConformance(t *testing.T) {
	var data struct {
		APIVersions  []string          `json:"apiVersions"`
		Routes       []string          `json:"routes"`
		Groups       []string          `json:"groups"`
		Helper       string            `json:"helper"`
		HelperSHA256 string            `json:"helperSHA256"`
		Objects      map[string]string `json:"objects"`
		ObjectSHA256 map[string]string `json:"objectSHA256"`
		Cells        []struct {
			ID                 string          `json:"id"`
			Group              string          `json:"group"`
			API                string          `json:"api"`
			Route              string          `json:"route"`
			Source             string          `json:"source"`
			SourceSHA256       string          `json:"sourceSHA256"`
			NativeMarkerSHA256 string          `json:"nativeMarkerSHA256"`
			Native             json.RawMessage `json:"native"`
		} `json:"cells"`
	}
	raw, err := os.ReadFile("testdata/conformance/hierarchy_exception_native.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	checkHash := func(source, want string) {
		t.Helper()
		sum := sha256.Sum256([]byte(source))
		if hex.EncodeToString(sum[:]) != want {
			t.Fatal("captured source bytes changed")
		}
	}
	checkHash(data.Helper, data.HelperSHA256)
	for name, source := range data.Objects {
		checkHash(source, data.ObjectSHA256[name])
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" || strings.Join(data.Routes, ",") != "anonymous,istest" || strings.Join(data.Groups, ",") != "C03,C02" || len(data.Cells) != 84 {
		t.Fatal("incomplete native matrix")
	}
	seen := map[string]bool{}
	for _, cell := range data.Cells {
		key := cell.API + "/" + cell.Group + "/" + cell.ID + "/" + cell.Route
		if seen[key] || len(cell.NativeMarkerSHA256) != 64 {
			t.Fatal("duplicate/unbound native cell")
		}
		seen[key] = true
		checkHash(cell.Source, cell.SourceSHA256)
		t.Run(key, func(t *testing.T) {
			classes := map[string]string{"TraceEmitter": data.Helper}
			if cell.Route == "istest" {
				classes["P3Observation"] = cell.Source
			}
			index := nativeBoundaryIndex(t, cell.API, classes, data.Objects)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native setup accepted; diagnostics %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = cell.API
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			machine := runner.newMachine()
			identity, err := vm.CompileAnonymous("String captureActor=UserInfo.getUserId();")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.Execute(identity); err != nil {
				t.Fatal(err)
			}
			actor := machine.Globals["captureActor"].Text
			var execErr error
			if cell.Route == "anonymous" {
				program, err := vm.CompileAnonymousWithOptions(cell.Source, vm.CompileOptions{APIVersion: cell.API})
				if err != nil {
					t.Fatal(err)
				}
				_, execErr = machine.Execute(program)
			} else {
				cases := Discover(index, Options{SelectedClasses: []string{"P3Observation"}, SelectedMethod: "observe"})
				if len(cases) != 1 {
					t.Fatalf("named discovery: %d", len(cases))
				}
				methods, methodErrors := compileTestMethods(cases)
				programs, programErrors := compileTestInvokePrograms(cases)
				key := testCaseKey(cases[0])
				if methodErrors[key] != nil || programErrors[key] != nil {
					t.Fatalf("named compilation: %v %v", methodErrors[key], programErrors[key])
				}
				if err := machine.RegisterMethod(methods[key]); err != nil {
					t.Fatal(err)
				}
				machine.EnableTestContext()
				_, execErr = machine.ExecuteInClass(programs[key], cases[0].ClassName)
			}
			var runtimeErr *vm.RuntimeError
			if !errors.As(execErr, &runtimeErr) || !strings.HasSuffix(runtimeErr.Type, "AssertException") {
				t.Fatalf("missing assertion transport: %v", execErr)
			}
			_, payload, found := strings.Cut(runtimeErr.ExceptionMessage(), "C126_RESULT|"+cell.ID+"|")
			if !found {
				t.Fatalf("missing marker: %v", execErr)
			}
			// Only the capture desk's redacted current-user identity is projected.
			// All fields, frame text, frame counts and other values compare exactly.
			if actor == "" {
				t.Fatal("missing local current user")
			}
			payload = strings.ReplaceAll(payload, `"`+actor+`"`, `"[USER_ID]"`)
			var got, want any
			if err := json.Unmarshal([]byte(payload), &got); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(cell.Native, &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("native %s; Glade %s", cell.Native, payload)
			}
		})
	}
	for _, group := range data.Groups {
		for n := 1; n <= 7; n++ {
			id := fmt.Sprintf("%s-%02d", group, n)
			for _, api := range data.APIVersions {
				for _, route := range data.Routes {
					if !seen[api+"/"+group+"/"+id+"/"+route] {
						t.Fatalf("missing %s/%s/%s", api, id, route)
					}
				}
			}
		}
	}
}
