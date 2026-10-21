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

func TestPrivateTestReflectionNativeConformance(t *testing.T) {
	var data struct {
		RowCount, ScalarCells, EnforcedRows, EnforcedScalarCells, CarriedRows, CarriedScalarCells int
		OrgIDSHA256, OrgTSVSHA256, OrgRunSHA256, TestClass                                        string
		Sources, SourceSHA256, AnonymousSources, AnonymousSourceSHA256                            map[string]string
		OrgContext                                                                                struct {
			RequestedEdition, OtherFeatureContext string
			FeaturesRequested                     []string
			SettingsChanged                       *bool
		}
		Captures []struct {
			API, TransportAPI, Metadata, MetadataSHA256, DeploymentID, DeploymentStatus, DeploymentReceiptSHA256 string
			CheckOnly                                                                                            bool
		}
		Rows []struct {
			API, TransportAPI, Route, ID, ReceiptSHA256 string
			Native                                      map[string]any
		}
		Carry       struct{ ID, OwnerCommit, Reason string }
		Uncaptured  []string
		Diagnostics []struct {
			ID, Reason, ReceiptSHA256, SourceSHA256 string
			AdmittedObservations                    int
		}
	}
	raw, err := os.ReadFile("testdata/conformance/private_test_reflection.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	digest := func(value string) {
		t.Helper()
		if bytes, err := hex.DecodeString(value); err != nil || len(bytes) != sha256.Size {
			t.Fatal("missing native binding")
		}
	}
	checkHash := func(source, want string) {
		t.Helper()
		hash := sha256.Sum256([]byte(source))
		if source == "" || hex.EncodeToString(hash[:]) != want {
			t.Fatal("native source binding changed")
		}
	}
	if data.RowCount != 27 || data.ScalarCells != 81 || data.EnforcedRows != 21 || data.EnforcedScalarCells != 63 || data.CarriedRows != 6 || data.CarriedScalarCells != 18 || len(data.Rows) != 27 || len(data.Captures) != 3 || len(data.Sources) != 3 || len(data.AnonymousSources) != 4 {
		t.Fatal("incomplete native reflection matrix")
	}
	if data.OrgContext.RequestedEdition != "Developer" || data.OrgContext.OtherFeatureContext != "UNCAPTURED" || data.OrgContext.FeaturesRequested == nil || len(data.OrgContext.FeaturesRequested) != 0 || data.OrgContext.SettingsChanged == nil || *data.OrgContext.SettingsChanged {
		t.Fatal("native org context changed")
	}
	for _, value := range []string{data.OrgIDSHA256, data.OrgTSVSHA256, data.OrgRunSHA256} {
		digest(value)
	}
	for name, source := range data.Sources {
		checkHash(source, data.SourceSHA256[name])
	}
	for id, source := range data.AnonymousSources {
		checkHash(source, data.AnonymousSourceSHA256[id])
	}
	if len(data.Diagnostics) != 1 || data.Diagnostics[0].AdmittedObservations != 0 || data.Diagnostics[0].Reason == "" {
		t.Fatal("failed deployment diagnostic changed")
	}
	digest(data.Diagnostics[0].ReceiptSHA256)
	digest(data.Diagnostics[0].SourceSHA256)
	if data.Carry.ID != "H03" || data.Carry.OwnerCommit != "capture-revision-101178971230423374541195777434233254409522279888771267780935517270067878892000" || data.Carry.Reason == "" {
		t.Fatal("missing private-nested dependency carry")
	}
	seenAPI, seenRow := map[string]bool{}, map[string]bool{}
	enforced, carried := 0, 0
	for _, capture := range data.Captures {
		if seenAPI[capture.API] || capture.API != capture.TransportAPI || capture.CheckOnly || capture.DeploymentID == "" || capture.DeploymentStatus != "Succeeded" {
			t.Fatal("invalid native source/transport capture")
		}
		seenAPI[capture.API] = true
		checkHash(capture.Metadata, capture.MetadataSHA256)
		digest(capture.DeploymentReceiptSHA256)
		if !strings.Contains(capture.Metadata, "<apiVersion>"+capture.API+"</apiVersion>") {
			t.Fatal("native source API changed")
		}
		index := nativeBoundaryIndex(t, capture.API, data.Sources, nil)
		analysis := sema.Analyze(index)
		if index.HasErrors() || analysis.HasErrors() {
			t.Fatalf("native accepted; local diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
		}
		cases := Discover(index, Options{SelectedClasses: []string{data.TestClass}})
		if len(cases) != 5 {
			t.Fatal("named observation selection changed")
		}
		methods, methodErrors := compileTestMethods(cases)
		programs, programErrors := compileTestInvokePrograms(cases)
		for _, row := range data.Rows {
			if row.API != capture.API {
				continue
			}
			key := row.API + "/" + row.Route + "/" + row.ID
			if seenRow[key] || row.API != row.TransportAPI || (row.Route != "anonymous" && row.Route != "istest") || row.ID < "H01" || row.ID > "H05" || (row.Route == "anonymous" && row.ID == "H05") || len(row.Native) != 3 {
				t.Fatal("invalid native row", key)
			}
			for _, field := range []string{"single", "empty", "null"} {
				if _, ok := row.Native[field]; !ok {
					t.Fatal("missing overload", key, field)
				}
			}
			seenRow[key] = true
			digest(row.ReceiptSHA256)
			if row.ID == data.Carry.ID {
				carried++
			} else {
				enforced++
			}
			t.Run(key, func(t *testing.T) {
				org := orgFromIndex(index)
				org.APIVersion = row.TransportAPI
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				machine := runner.newMachine()
				var execErr error
				if row.Route == "anonymous" {
					program, err := vm.CompileAnonymousWithOptions(data.AnonymousSources[row.ID], vm.CompileOptions{APIVersion: row.API})
					if err != nil {
						t.Fatal(err)
					}
					_, execErr = machine.Execute(program)
				} else {
					var selected *TestCase
					for i := range cases {
						if strings.EqualFold(cases[i].MethodName, row.ID) {
							selected = &cases[i]
						}
					}
					if selected == nil {
						t.Fatal("missing native method", row.ID)
					}
					methodKey := testCaseKey(*selected)
					if methodErrors[methodKey] != nil || programErrors[methodKey] != nil {
						t.Fatalf("named compile: %v %v", methodErrors[methodKey], programErrors[methodKey])
					}
					if err := machine.RegisterMethod(methods[methodKey]); err != nil {
						t.Fatal(err)
					}
					machine.EnableTestContext()
					_, execErr = machine.ExecuteInClass(programs[methodKey], selected.ClassName)
				}
				var observed *vm.RuntimeError
				if !errors.As(execErr, &observed) || observed.Type != "System.AssertException" {
					t.Fatalf("missing intentional observation assertion: %v", execErr)
				}
				_, payload, ok := strings.Cut(observed.ExceptionMessage(), "REFLECT_VISIBILITY|"+row.ID+"|")
				var actual map[string]any
				if !ok || json.Unmarshal([]byte(payload), &actual) != nil {
					t.Fatal("invalid native observation frame")
				}
				if row.ID == data.Carry.ID {
					t.Logf("CARRY %s native=%v local=%v: %s (%s)", row.ID, row.Native, actual, data.Carry.Reason, data.Carry.OwnerCommit)
					return
				}
				if !reflect.DeepEqual(row.Native, actual) {
					t.Fatalf("native=%v local=%v", row.Native, actual)
				}
			})
		}
	}
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		if !seenAPI[api] {
			t.Fatal("missing native API", api)
		}
	}
	if len(seenRow) != data.RowCount || enforced != data.EnforcedRows || carried != data.CarriedRows {
		t.Fatalf("native row counts: total=%d enforced=%d carried=%d", len(seenRow), enforced, carried)
	}
	for _, scope := range data.Uncaptured {
		t.Log("UNCAPTURED", scope)
	}
}
