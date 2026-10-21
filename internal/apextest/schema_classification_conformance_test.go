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

func TestSchemaClassificationNativeConformance(t *testing.T) {
	var data struct {
		APIVersions, Routes    []string
		NativeObservationCount int
		Fixtures               map[string]struct {
			ClassName                          string
			Classes, SourceSHA256              map[string]string
			ClassMetadata, ClassMetadataSHA256 string
		}
		Cells []struct {
			API, TransportAPI, Route, Fixture, Method, Marker                string
			DeploymentID, Status, Cleanup, RawDeploymentSHA256, RawRowSHA256 string
			Done, CheckOnly                                                  bool
			ProjectedPayload, ProjectedPayloadSHA256                         string
			Native                                                           map[string]any
		}
		Carried map[string]string
	}
	raw, err := os.ReadFile("testdata/conformance/schema_classification_boundaries.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIVersions, ",") != "62.0,65.0,67.0" || strings.Join(data.Routes, ",") != "istest" || data.NativeObservationCount != 8 || len(data.Cells) != 8 || len(data.Fixtures) != 6 {
		t.Fatal("incomplete named classification observations")
	}
	hash := func(s string) string { sum := sha256.Sum256([]byte(s)); return hex.EncodeToString(sum[:]) }
	for _, key := range []string{"count", "populated", "ids", "anonymous", "api68", "otherTemplates"} {
		if data.Carried[key] == "" {
			t.Fatal("carry lacks reason", key)
		}
		t.Logf("carried %s: %s", key, data.Carried[key])
	}
	seen := map[string]bool{}
	for _, cell := range data.Cells {
		key := cell.Fixture + "/" + cell.Method
		if seen[key] || cell.Route != "istest" || cell.API != cell.TransportAPI || !cell.Done || !cell.CheckOnly || cell.Status != "Failed" || cell.DeploymentID == "" || cell.Cleanup == "" {
			t.Fatal("invalid provenance", key)
		}
		seen[key] = true
		for _, digest := range []string{cell.RawDeploymentSHA256, cell.RawRowSHA256} {
			if b, e := hex.DecodeString(digest); e != nil || len(b) != sha256.Size {
				t.Fatal("invalid raw capture digest", key)
			}
		}
		var projected map[string]any
		if hash(cell.ProjectedPayload) != cell.ProjectedPayloadSHA256 || json.Unmarshal([]byte(cell.ProjectedPayload), &projected) != nil || !reflect.DeepEqual(projected, cell.Native) {
			t.Fatal("native payload changed", key)
		}
		t.Run(key, func(t *testing.T) {
			fixture, ok := data.Fixtures[cell.Fixture]
			if !ok || !strings.HasPrefix(cell.Fixture, cell.API+"/") || hash(fixture.ClassMetadata) != fixture.ClassMetadataSHA256 || !strings.Contains(fixture.ClassMetadata, "<apiVersion>"+cell.API+"</apiVersion>") {
				t.Fatal("invalid source API fixture")
			}
			for name, source := range fixture.Classes {
				if hash(source) != fixture.SourceSHA256[name] {
					t.Fatal("source digest changed", name)
				}
			}
			index := nativeBoundaryIndex(t, cell.API, fixture.Classes, nil)
			if index.HasErrors() || sema.Analyze(index).HasErrors() {
				t.Fatal("native declarations rejected")
			}
			cases := Discover(index, Options{SelectedClasses: []string{fixture.ClassName}})
			var selected []TestCase
			for _, candidate := range cases {
				if candidate.MethodName == cell.Method {
					selected = append(selected, candidate)
				}
			}
			if len(selected) != 1 {
				t.Fatal("missing native method", cell.Method)
			}
			compiled, methodErrors := compileTestMethodsForIndex(&index, selected)
			programs, programErrors := compileTestInvokePrograms(selected)
			test := selected[0]
			methodKey := testCaseKey(test)
			if methodErrors[methodKey] != nil || programErrors[methodKey] != nil {
				t.Fatalf("fixture compile: %v %v", methodErrors[methodKey], programErrors[methodKey])
			}
			org := orgFromIndex(index)
			org.APIVersion = cell.API
			base := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			machine := base.newMachine()
			machine.EnableTestContext()
			if err := machine.RegisterMethod(compiled[methodKey]); err != nil {
				t.Fatal(err)
			}
			_, execErr := machine.ExecuteInClass(programs[methodKey], test.ClassName)
			var fault *vm.RuntimeError
			if !errors.As(execErr, &fault) || !strings.HasSuffix(fault.Type, "AssertException") {
				t.Fatal("missing native observation transport", execErr)
			}
			_, payload, found := strings.Cut(fault.ExceptionMessage(), "Expected: "+cell.Marker+", Actual: ")
			var got map[string]any
			if !found || json.Unmarshal([]byte(payload), &got) != nil {
				t.Fatal("invalid observation payload", execErr)
			}
			switch cell.Method {
			case "lists":
				for _, field := range []string{"list", "set", "singleton", "double", "typed", "element"} {
					if !reflect.DeepEqual(got[field], cell.Native[field]) {
						t.Fatalf("%s: native=%v local=%v", field, cell.Native[field], got[field])
					}
				}
			case "observe":
				for key, placeholder := range map[string]string{"stored": "[STORED_ID]", "user": "[USER_ID]"} {
					row, ok := got[key].(map[string]any)
					if !ok {
						t.Fatal("missing Id control", key)
					}
					if id, ok := row["id"].(string); !ok || (len(id) != 15 && len(id) != 18) {
						t.Fatal("missing generated Id", key)
					}
					row["id"] = placeholder
				}
				if !reflect.DeepEqual(got, cell.Native) {
					t.Fatalf("native=%v local=%v", cell.Native, got)
				}
			case "templateId":
				if !reflect.DeepEqual(got, cell.Native) {
					t.Fatalf("native=%v local=%v", cell.Native, got)
				}
			default:
				t.Fatal("unhandled native observation", cell.Method)
			}
		})
	}
}
