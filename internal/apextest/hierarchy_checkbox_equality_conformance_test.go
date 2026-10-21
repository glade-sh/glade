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

func TestHierarchyCheckboxEqualityNativeConformance(t *testing.T) {
	var data struct {
		APIVersions, Routes    []string
		NativeObservationCount int
		Objects, ObjectSHA256  map[string]string
		Fixtures               map[string]struct {
			Classes, SourceSHA256              map[string]string
			ClassMetadata, ClassMetadataSHA256 string
		}
		Cells []struct {
			API, TransportAPI, Route, Syntax, Method, Fixture string
			DeploymentID, Status, Cleanup                     string
			RawDeploymentSHA256, RawRowSHA256                 string
			ProjectedPayload, ProjectedPayloadSHA256          string
			Done, CheckOnly                                   bool
			Native                                            map[string]any
		}
		Carried map[string]string
	}
	raw, err := os.ReadFile("testdata/conformance/hierarchy_checkbox_equality.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0" || strings.Join(data.Routes, ",") != "istest" || data.NativeObservationCount != 8 || len(data.Cells) != 8 || len(data.Fixtures) != 4 {
		t.Fatal("incomplete named hierarchy Checkbox observations")
	}
	hash := func(value string) string {
		sum := sha256.Sum256([]byte(value))
		return hex.EncodeToString(sum[:])
	}
	for name, source := range data.Objects {
		if hash(source) != data.ObjectSHA256[name] {
			t.Fatal("object source digest changed", name)
		}
	}
	seen := map[string]bool{}
	for _, cell := range data.Cells {
		key := cell.API + "/" + cell.Syntax + "/" + cell.Method
		if seen[key] || cell.Route != "istest" || cell.TransportAPI != cell.API || !cell.Done || !cell.CheckOnly || cell.Status != "Failed" || cell.DeploymentID == "" || cell.Cleanup == "" || cell.Fixture != cell.API+"/"+cell.Syntax {
			t.Fatal("invalid native provenance", key)
		}
		seen[key] = true
		for _, digest := range []string{cell.RawDeploymentSHA256, cell.RawRowSHA256} {
			if bytes, err := hex.DecodeString(digest); err != nil || len(bytes) != sha256.Size {
				t.Fatal("missing retained raw capture digest", key)
			}
		}
		var projected map[string]any
		if hash(cell.ProjectedPayload) != cell.ProjectedPayloadSHA256 || json.Unmarshal([]byte(cell.ProjectedPayload), &projected) != nil || !reflect.DeepEqual(projected, cell.Native) {
			t.Fatal("native projected payload changed", key)
		}
	}
	for _, field := range []string{"p", "j", "anonymous", "api68"} {
		if data.Carried[field] == "" {
			t.Fatal("carry lacks its reason", field)
		}
		t.Logf("carried %s: %s", field, data.Carried[field])
	}
	for _, api := range data.APIVersions {
		for _, syntax := range []string{"dynamic", "typed"} {
			t.Run(api+"/"+syntax, func(t *testing.T) {
				fixture := data.Fixtures[api+"/"+syntax]
				if hash(fixture.ClassMetadata) != fixture.ClassMetadataSHA256 || !strings.Contains(fixture.ClassMetadata, "<apiVersion>"+api+"</apiVersion>") {
					t.Fatal("class source API changed")
				}
				for name, source := range fixture.Classes {
					if hash(source) != fixture.SourceSHA256[name] {
						t.Fatal("class source digest changed", name)
					}
				}
				index := nativeBoundaryIndex(t, api, fixture.Classes, data.Objects)
				if index.HasErrors() || sema.Analyze(index).HasErrors() {
					t.Fatal("native declarations rejected")
				}
				cases := Discover(index, Options{SelectedClasses: []string{"HierarchyCheckboxObservation"}})
				if len(cases) != 2 {
					t.Fatalf("discovered %d cases, want 2", len(cases))
				}
				compiled, methodErrors := compileTestMethodsForIndex(&index, cases)
				programs, programErrors := compileTestInvokePrograms(cases)
				org := orgFromIndex(index)
				org.APIVersion = api
				base := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				for _, test := range cases {
					key := testCaseKey(test)
					if methodErrors[key] != nil || programErrors[key] != nil {
						t.Fatalf("native fixture compile: %v %v", methodErrors[key], programErrors[key])
					}
					matched := false
					for _, cell := range data.Cells {
						if cell.API != api || cell.Syntax != syntax || cell.Method != test.MethodName {
							continue
						}
						matched = true
						t.Run(cell.Method, func(t *testing.T) {
							machine := base.newMachine()
							machine.EnableTestContext()
							if err := machine.RegisterMethod(compiled[key]); err != nil {
								t.Fatal(err)
							}
							_, execErr := machine.ExecuteInClass(programs[key], test.ClassName)
							var fault *vm.RuntimeError
							if !errors.As(execErr, &fault) || !strings.HasSuffix(fault.Type, "AssertException") {
								t.Fatal("missing native observation transport", execErr)
							}
							_, payload, found := strings.Cut(fault.ExceptionMessage(), "Expected: HCE_NATIVE, Actual: ")
							var got map[string]any
							if !found || json.Unmarshal([]byte(payload), &got) != nil {
								t.Fatal("invalid observation payload", execErr)
							}
							for _, field := range []string{"shape", "checks", "l", "r"} {
								if !reflect.DeepEqual(got[field], cell.Native[field]) {
									t.Fatalf("%s: native=%v local=%v", field, cell.Native[field], got[field])
								}
							}
						})
					}
					if !matched {
						t.Fatal("missing native method", test.MethodName)
					}
				}
			})
		}
	}
}
