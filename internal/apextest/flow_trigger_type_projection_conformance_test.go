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

// Captured sources remain intact; the explicit local projection carries
// record-triggered discovery and existing empty-caller mutation separately.
func TestFlowTriggerTypeProjectionNativeConformance(t *testing.T) {
	var data struct {
		APIVersions, Routes                             []string
		NativeObservationCount, CarriedObservationCount int
		Fixtures                                        map[string]struct {
			Files, FileSHA256, Classes, SourceSHA256 map[string]string
			CapturedClasses, CapturedSourceSHA256    map[string]string
			ClassMetadata, ClassMetadataSHA256       string
		}
		Cells []struct {
			API, TransportAPI, Route, Method, DeploymentID, Status, Cleanup string
			Done, CheckOnly                                                 bool
			RawDeploymentSHA256, RawRowSHA256, Payload, PayloadSHA256       string
			Native                                                          map[string]any
		}
		Partial struct {
			API, TransportAPI, Route, DeploymentID, Status, Source, SourceSHA256, RawDeploymentSHA256, NativeError, Reason, Cleanup string
			Done, CheckOnly                                                                                                         bool
		}
		Carried map[string]string
	}
	raw, err := os.ReadFile("testdata/conformance/flow_trigger_type_projection.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIVersions, ",") != "62.0,65.0,67.0" || strings.Join(data.Routes, ",") != "istest" || data.NativeObservationCount != 3 || len(data.Cells) != 3 || len(data.Fixtures) != 3 || data.CarriedObservationCount != 1 {
		t.Fatal("incomplete native view/copy observations")
	}
	hash := func(s string) string { sum := sha256.Sum256([]byte(s)); return hex.EncodeToString(sum[:]) }
	digest := func(s string) bool { b, err := hex.DecodeString(s); return err == nil && len(b) == sha256.Size }
	partial := data.Partial
	if partial.API != "67.0" || partial.TransportAPI != partial.API || partial.Route != "istest" || !partial.Done || !partial.CheckOnly || partial.Status != "Failed" || partial.DeploymentID == "" || partial.Cleanup == "" || hash(partial.Source) != partial.SourceSHA256 || !digest(partial.RawDeploymentSHA256) || !strings.HasPrefix(partial.NativeError, "System.DmlException: Operation failed due to fields being inaccessible on Sobject CatalogNote__c") || partial.Reason == "" {
		t.Fatal("missing partial-copy native evidence")
	}
	t.Logf("carried API67 default-DML copy: %s; native=%s", partial.Reason, partial.NativeError)
	for _, key := range []string{"api67Copy", "anonymous", "api68", "otherMetadata", "recordTriggeredDiscovery", "emptyCaller"} {
		if data.Carried[key] == "" {
			t.Fatal("carry lacks reason", key)
		}
		t.Logf("carried %s: %s", key, data.Carried[key])
	}
	seen := map[string]bool{}
	for _, cell := range data.Cells {
		if seen[cell.API] || cell.API != cell.TransportAPI || cell.Route != "istest" || cell.Method != "observe" || !cell.Done || !cell.CheckOnly || cell.Status != "Failed" || cell.DeploymentID == "" || cell.Cleanup == "" || !digest(cell.RawDeploymentSHA256) || !digest(cell.RawRowSHA256) || hash(cell.Payload) != cell.PayloadSHA256 {
			t.Fatal("invalid native provenance", cell.API)
		}
		seen[cell.API] = true
		var native map[string]any
		if json.Unmarshal([]byte(cell.Payload), &native) != nil || !reflect.DeepEqual(native, cell.Native) {
			t.Fatal("native payload changed", cell.API)
		}
		t.Run(cell.API, func(t *testing.T) {
			fixture, ok := data.Fixtures[cell.API]
			if !ok || hash(fixture.ClassMetadata) != fixture.ClassMetadataSHA256 || !strings.Contains(fixture.ClassMetadata, "<apiVersion>"+cell.API+"</apiVersion>") {
				t.Fatal("invalid class API pin")
			}
			for name, source := range fixture.Files {
				if hash(source) != fixture.FileSHA256[name] {
					t.Fatal("metadata digest changed", name)
				}
			}
			for name, source := range fixture.CapturedClasses {
				if hash(source) != fixture.CapturedSourceSHA256[name] {
					t.Fatal("captured source digest changed", name)
				}
				projected := strings.ReplaceAll(source, "  FlowDefinitionView triggered=[SELECT TriggerType FROM FlowDefinitionView WHERE ApiName='QuietCatalogRecordFlow' AND IsActive=true];\n", "")
				projected = strings.ReplaceAll(projected, "Kind__c=triggered.TriggerType", "Kind__c='RecordBeforeSave'")
				projected = strings.ReplaceAll(projected, "'triggered'=>triggered.TriggerType,", "")
				projected = strings.ReplaceAll(projected, ",'triggered'=>triggered.TriggerType", "")
				if projected != fixture.Classes[name] {
					t.Fatal("unexpected source projection", name)
				}
			}
			for name, source := range fixture.Classes {
				if hash(source) != fixture.SourceSHA256[name] {
					t.Fatal("source digest changed", name)
				}
			}
			index := flowCoreIndex(t, cell.API, fixture.Classes, fixture.Files)
			if index.HasErrors() || sema.Analyze(index).HasErrors() {
				t.Fatal("native declarations rejected")
			}
			tests := Discover(index, Options{SelectedClasses: []string{"FlowViewObservation"}, SelectedMethod: "observe"})
			if len(tests) != 1 {
				t.Fatal("missing native observer")
			}
			methods, methodErrors := compileTestMethodsForIndex(&index, tests)
			programs, programErrors := compileTestInvokePrograms(tests)
			test := tests[0]
			key := testCaseKey(test)
			if methodErrors[key] != nil || programErrors[key] != nil {
				t.Fatalf("fixture compile: %v %v", methodErrors[key], programErrors[key])
			}
			org := orgFromIndex(index)
			org.APIVersion = cell.API
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			machine := runner.newMachine()
			machine.EnableTestContext()
			if err := machine.RegisterMethod(methods[key]); err != nil {
				t.Fatal(err)
			}
			_, execErr := machine.ExecuteInClass(programs[key], test.ClassName)
			var fault *vm.RuntimeError
			if !errors.As(execErr, &fault) || !strings.HasSuffix(fault.Type, "AssertException") {
				t.Fatal("missing observation transport", execErr)
			}
			_, payload, found := strings.Cut(fault.ExceptionMessage(), "Expected: FLOW_VIEW_NATIVE, Actual: ")
			var got map[string]any
			if !found || json.Unmarshal([]byte(payload), &got) != nil {
				t.Fatal("invalid observation payload", execErr)
			}
			expected := make(map[string]any, len(cell.Native))
			for name, value := range cell.Native {
				if name != "triggered" && name != "emptyCaller" {
					expected[name] = value
				}
			}
			if nativeEmpty, ok := cell.Native["emptyCaller"]; ok {
				t.Logf("carried emptyCaller: native=%v local=%v", nativeEmpty, got["emptyCaller"])
				delete(got, "emptyCaller")
			}
			if !reflect.DeepEqual(got, expected) {
				t.Fatalf("native projection=%v local=%v", expected, got)
			}
		})
	}
}
