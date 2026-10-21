package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestLongTextBoundaryObservationsConformance(t *testing.T) {
	var data struct {
		APIVersions            []string          `json:"apiVersions"`
		ShapeCount             int               `json:"shapeCount"`
		NativeObservationCount int               `json:"nativeObservationCount"`
		Objects                map[string]string `json:"objects"`
		Fixtures               map[string]struct {
			Classes      map[string]string `json:"classes"`
			SourceSHA256 map[string]string `json:"sourceSHA256"`
			AccessMode   string            `json:"accessMode"`
		} `json:"fixtures"`
		Cells []struct {
			API, Row, Method, Route, AccessMode, RawRow, RawRowSHA256 string
			Native                                                    map[string]any `json:"native"`
		} `json:"cells"`
		Carries []struct {
			Scope, Reason  string
			ExpectedNative any
		} `json:"carries"`
	}
	raw, err := os.ReadFile("testdata/conformance/longtext_boundary_observations.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" || data.ShapeCount != 6 || data.NativeObservationCount != 72 || len(data.Cells) != 72 || len(data.Fixtures) != 12 {
		t.Fatal("incomplete named long text native matrix")
	}
	hash := func(value string) string { sum := sha256.Sum256([]byte(value)); return hex.EncodeToString(sum[:]) }
	seen := map[string]bool{}
	for _, cell := range data.Cells {
		key := cell.API + "/" + cell.Row + "/" + cell.Method
		mode := "system"
		if cell.API == "62.0" {
			mode = "default"
		}
		if seen[key] || cell.Route != "istest" || cell.AccessMode != mode || hash(cell.RawRow) != cell.RawRowSHA256 {
			t.Fatalf("invalid native cell %s", key)
		}
		seen[key] = true
		id, payload, found := strings.Cut(strings.TrimSuffix(cell.RawRow, "\n"), "\t")
		var observed map[string]any
		if !found || id != cell.Row+"-"+strings.TrimPrefix(cell.Method, "shape") || json.Unmarshal([]byte(payload), &observed) != nil || !reflect.DeepEqual(observed, cell.Native) {
			t.Fatalf("native raw row mismatch %s", key)
		}
	}
	for _, carry := range data.Carries {
		if carry.Scope == "" || carry.Reason == "" || carry.ExpectedNative != nil {
			t.Fatal("invalid pending native carry")
		}
		t.Logf("carried %s: %s", carry.Scope, carry.Reason)
	}
	for _, api := range data.APIVersions {
		for _, row := range []string{"L01", "L02", "L03", "L04"} {
			t.Run(api+"/"+row, func(t *testing.T) {
				fixture, ok := data.Fixtures[api+"/"+row]
				if !ok {
					t.Fatal("missing source fixture")
				}
				for name, source := range fixture.Classes {
					if hash(source) != fixture.SourceSHA256[name] {
						t.Fatalf("source digest %s", name)
					}
				}
				index := nativeBoundaryIndex(t, api, fixture.Classes, data.Objects)
				analysis := sema.Analyze(index)
				if index.HasErrors() || analysis.HasErrors() {
					t.Fatalf("native declarations accepted: %v %v", index.Diagnostics, analysis.Diagnostics)
				}
				cases := Discover(index, Options{SelectedClasses: []string{"LongTextBoundaryNamedProbe"}})
				if len(cases) != 6 {
					t.Fatalf("named discovery: %d", len(cases))
				}
				compiled, methodErrors := compileTestMethodsForIndex(&index, cases)
				programs, programErrors := compileTestInvokePrograms(cases)
				org := orgFromIndex(index)
				org.APIVersion = api
				base := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				counts := newConformanceCountLog(t, "LongTextBoundary/"+row+"/"+fixture.AccessMode, api)
				for _, test := range cases {
					key := testCaseKey(test)
					if methodErrors[key] != nil || programErrors[key] != nil {
						t.Fatalf("native executes: %v %v", methodErrors[key], programErrors[key])
					}
					matched := false
					for _, cell := range data.Cells {
						if cell.API != api || cell.Row != row || cell.Method != test.MethodName {
							continue
						}
						matched = true
						counts.Run(t, "isTest", cell.Method, cell.Method, func(t *testing.T) {
							machine := base.newMachine()
							machine.EnableTestContext()
							if err := machine.RegisterMethod(compiled[key]); err != nil {
								t.Fatal(err)
							}
							_, execErr := machine.ExecuteInClass(programs[key], test.ClassName)
							var fault *vm.RuntimeError
							if !errors.As(execErr, &fault) || !strings.HasSuffix(fault.Type, "AssertException") {
								t.Fatalf("missing observation: %v", execErr)
							}
							_, payload, found := strings.Cut(fault.ExceptionMessage(), "Expected: LONGTEXT_NATIVE, Actual: ")
							var got map[string]any
							if !found || json.Unmarshal([]byte(payload), &got) != nil {
								t.Fatalf("invalid observation: %v", execErr)
							}
							if !reflect.DeepEqual(got, cell.Native) {
								want, _ := json.Marshal(cell.Native)
								t.Fatalf("native=%s; local=%s", want, payload)
							}
						})
					}
					if !matched {
						t.Fatalf("missing native method %s", test.MethodName)
					}
				}
			})
		}
	}
}
