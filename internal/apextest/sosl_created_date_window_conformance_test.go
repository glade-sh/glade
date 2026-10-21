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

func TestSOSLCreatedDateWindowNativeConformance(t *testing.T) {
	var data struct {
		RowCount, ScalarCells                   int
		OrgIDSHA256, OrgTSVSHA256, OrgRunSHA256 string
		TestClass, Source, SourceSHA256         string
		OrgContext                              struct {
			RequestedEdition, OtherFeatureContext string
			FeaturesRequested                     []string
			SettingsChanged                       *bool
		}
		Captures []struct {
			API, TransportAPI, Route, Metadata, MetadataSHA256 string
			DeploymentID, DeploymentStatus, TestOutcome        string
			DeploymentReceiptSHA256, ReceiptSHA256             string
			CheckOnly                                          bool
			Observations                                       map[string]map[string]any
		}
		Uncaptured []string
	}
	raw, err := os.ReadFile("testdata/conformance/sosl_created_date_window.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	checkDigest := func(value string) {
		t.Helper()
		if bytes, err := hex.DecodeString(value); err != nil || len(bytes) != sha256.Size {
			t.Fatal("missing native artifact binding")
		}
	}
	checkHash := func(source, want string) {
		t.Helper()
		digest := sha256.Sum256([]byte(source))
		if source == "" || hex.EncodeToString(digest[:]) != want {
			t.Fatal("captured source hash changed")
		}
	}
	if data.RowCount != 9 || data.ScalarCells != 45 || len(data.Captures) != 3 || len(data.Uncaptured) != 3 {
		t.Fatal("incomplete native matrix")
	}
	if data.OrgContext.RequestedEdition != "Developer" || data.OrgContext.OtherFeatureContext != "UNCAPTURED" || data.OrgContext.FeaturesRequested == nil || len(data.OrgContext.FeaturesRequested) != 0 || data.OrgContext.SettingsChanged == nil || *data.OrgContext.SettingsChanged {
		t.Fatal("native org context changed")
	}
	for _, hash := range []string{data.OrgIDSHA256, data.OrgTSVSHA256, data.OrgRunSHA256} {
		checkDigest(hash)
	}
	checkHash(data.Source, data.SourceSHA256)
	seenAPI := map[string]bool{}
	rows, scalars := 0, 0
	for _, capture := range data.Captures {
		if seenAPI[capture.API] || capture.API != capture.TransportAPI || capture.Route != "istest" || capture.CheckOnly || capture.DeploymentID == "" || capture.DeploymentStatus != "Succeeded" || capture.TestOutcome != "Failed" || len(capture.Observations) != 3 {
			t.Fatal("invalid named source/transport capture")
		}
		seenAPI[capture.API] = true
		checkDigest(capture.ReceiptSHA256)
		checkDigest(capture.DeploymentReceiptSHA256)
		checkHash(capture.Metadata, capture.MetadataSHA256)
		if !strings.Contains(capture.Metadata, "<apiVersion>"+capture.API+"</apiVersion>") {
			t.Fatal("native source API changed")
		}
		for _, id := range []string{"SW01", "SW02", "SW03"} {
			want, ok := capture.Observations[id]
			if !ok || len(want) != 5 {
				t.Fatal("missing native row", id)
			}
			rows++
			scalars += len(want)
			t.Run(capture.API+"/istest/"+id, func(t *testing.T) {
				index := nativeBoundaryIndex(t, capture.API, map[string]string{data.TestClass: data.Source}, nil)
				analysis := sema.Analyze(index)
				if index.HasErrors() || analysis.HasErrors() {
					t.Fatalf("native accepted; local diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
				}
				cases := Discover(index, Options{SelectedClasses: []string{data.TestClass}, SelectedMethod: strings.ToLower(id)})
				if len(cases) != 1 {
					t.Fatal("named observation selection changed")
				}
				methods, methodErrors := compileTestMethods(cases)
				programs, programErrors := compileTestInvokePrograms(cases)
				key := testCaseKey(cases[0])
				if methodErrors[key] != nil || programErrors[key] != nil {
					t.Fatalf("named compile: %v %v", methodErrors[key], programErrors[key])
				}
				org := orgFromIndex(index)
				org.APIVersion = capture.TransportAPI
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				machine := runner.newMachine()
				if err := machine.RegisterMethod(methods[key]); err != nil {
					t.Fatal(err)
				}
				machine.EnableTestContext()
				_, execErr := machine.ExecuteInClass(programs[key], cases[0].ClassName)
				var observed *vm.RuntimeError
				if !errors.As(execErr, &observed) || observed.Type != "System.AssertException" {
					t.Fatalf("missing intentional observation assertion: %v", execErr)
				}
				_, payload, ok := strings.Cut(observed.ExceptionMessage(), "SEARCH_WINDOW|"+id+"|")
				var actual map[string]any
				if !ok || json.Unmarshal([]byte(payload), &actual) != nil {
					t.Fatal("invalid observation frame")
				}
				if !reflect.DeepEqual(want, actual) {
					t.Fatalf("native=%v; local=%v", want, actual)
				}
			})
		}
	}
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		if !seenAPI[api] {
			t.Fatal("missing native API", api)
		}
	}
	if rows != data.RowCount || scalars != data.ScalarCells {
		t.Fatalf("native rows=%d scalar cells=%d", rows, scalars)
	}
	for _, scope := range data.Uncaptured {
		t.Log("UNCAPTURED", scope)
	}
}
