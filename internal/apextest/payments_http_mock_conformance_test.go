package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

type paymentsMockNativeCell struct {
	API              string `json:"api"`
	Route            string `json:"route"`
	Native           string `json:"native"`
	NativeSHA256     string `json:"nativeSHA256"`
	RawReceiptSHA256 string `json:"rawReceiptSHA256"`
}

// MT01 has named captures only. The null request is a distinct native platform
// fault, retained as a carry; anonymous request execution was not observed.
func TestPaymentsHTTPRegisteredMockOrgConformance(t *testing.T) {
	var data struct {
		APIVersions              []string                     `json:"apiVersions"`
		Routes                   []string                     `json:"routes"`
		NativeObservationCount   int                          `json:"nativeObservationCount"`
		AssertedObservationCount int                          `json:"assertedObservationCount"`
		CarriedObservationCount  int                          `json:"carriedObservationCount"`
		Route                    nativeBoundaryRoute          `json:"route"`
		ExpectedNative           string                       `json:"expectedNative"`
		Cells                    []paymentsMockNativeCell     `json:"cells"`
		ClassMetadataByAPI       map[string]map[string]string `json:"classMetadataByAPI"`
		ClassMetadataSHA256      map[string]map[string]string `json:"classMetadataSHA256"`
		Anonymous                struct {
			ExpectedNative *string `json:"expectedNative"`
			Reason         string  `json:"reason"`
		} `json:"anonymous"`
		NullRequestCarry struct {
			Comparison string                   `json:"comparison"`
			Reason     string                   `json:"reason"`
			Source     nativeBoundaryRoute      `json:"source"`
			Cells      []paymentsMockNativeCell `json:"cells"`
		} `json:"nullRequestCarry"`
	}
	raw, err := os.ReadFile("testdata/conformance/payments_http_registered_mock.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" || strings.Join(data.Routes, ",") != "isTest" || data.NativeObservationCount != 6 || data.AssertedObservationCount != 3 || data.CarriedObservationCount != 3 || len(data.Cells) != 3 || len(data.NullRequestCarry.Cells) != 3 || data.ExpectedNative == "" {
		t.Fatal("incomplete named mock native matrix")
	}
	if data.Anonymous.ExpectedNative != nil || data.Anonymous.Reason == "" || data.NullRequestCarry.Comparison != "carried" || data.NullRequestCarry.Reason == "" {
		t.Fatal("lost unsupported request boundary")
	}
	for _, source := range []nativeBoundaryRoute{data.Route, data.NullRequestCarry.Source} {
		nativeBoundaryCheckSources(t, source)
		if source.ID != "MT01T" || source.Stage != "runtime" || len(source.Classes) != 2 || len(source.Tests) != 1 || source.Tests[0] != "MockedTransportSpec" || len(source.FixedClassAPIVersions) != 1 || source.FixedClassAPIVersions["MockedTransportCaller"] != "60.0" {
			t.Fatal("incorrect captured caller metadata")
		}
	}
	checkHash := func(source, want string) {
		t.Helper()
		sum := sha256.Sum256([]byte(source))
		if hex.EncodeToString(sum[:]) != want {
			t.Fatal("captured bytes changed")
		}
	}
	for _, cells := range [][]paymentsMockNativeCell{data.Cells, data.NullRequestCarry.Cells} {
		seen := map[string]bool{}
		for _, cell := range cells {
			if cell.Route != "isTest" || seen[cell.API] || !strings.Contains(",62.0,67.0,68.0,", ","+cell.API+",") {
				t.Fatal("unknown or repeated named cell")
			}
			seen[cell.API] = true
			checkHash(cell.Native, cell.NativeSHA256)
			if digest, err := hex.DecodeString(cell.RawReceiptSHA256); err != nil || len(digest) != sha256.Size {
				t.Fatal("invalid native receipt digest")
			}
		}
	}
	for _, cell := range data.NullRequestCarry.Cells {
		if !strings.HasPrefix(cell.Native, "System.UnexpectedException:") {
			t.Fatal("changed native null fault carry")
		}
	}
	if len(data.ClassMetadataByAPI) != 3 || len(data.ClassMetadataSHA256) != 3 {
		t.Fatal("missing captured source metadata")
	}
	for _, cell := range data.Cells {
		t.Run(cell.API, func(t *testing.T) {
			if cell.Native != data.ExpectedNative {
				t.Fatal("native answer mismatch")
			}
			metadata := data.ClassMetadataByAPI[cell.API]
			hashes := data.ClassMetadataSHA256[cell.API]
			if len(metadata) != 2 || len(hashes) != 2 {
				t.Fatal("incomplete class API capture")
			}
			for name := range data.Route.Classes {
				api := cell.API
				if fixed := data.Route.FixedClassAPIVersions[name]; fixed != "" {
					api = fixed
				}
				if !strings.Contains(metadata[name], "<apiVersion>"+api+"</apiVersion>") {
					t.Fatal("source API mismatch")
				}
				checkHash(metadata[name], hashes[name])
			}
			source := data.Route
			index := nativeBoundaryIndexWithClassAPIs(t, cell.API, source.Classes, nil, source.FixedClassAPIVersions)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native accepted; local rejected: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			cases := Discover(index, Options{SelectedClasses: source.Tests, SelectedMethod: "observe"})
			if len(cases) != 1 {
				t.Fatalf("expected one captured test, got %d", len(cases))
			}
			methods, methodErrors := compileTestMethods(cases)
			programs, programErrors := compileTestInvokePrograms(cases)
			key := testCaseKey(cases[0])
			if methodErrors[key] != nil || programErrors[key] != nil {
				t.Fatalf("compile: %v %v", methodErrors[key], programErrors[key])
			}
			org := orgFromIndex(index)
			org.APIVersion = cell.API
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			machine := runner.newMachine()
			if err := machine.RegisterMethod(methods[key]); err != nil {
				t.Fatal(err)
			}
			machine.EnableTestContext()
			_, execErr := machine.ExecuteInClass(programs[key], cases[0].ClassName)
			nativeBoundaryValue(t, source.ID, nativeBoundaryExpected{Outcome: "runtime", Value: data.ExpectedNative}, execErr)
		})
	}
}
