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

func TestQualifiedStaticShadowNativeConformance(t *testing.T) {
	var data struct {
		APIVersions, Routes    []string
		NativeObservationCount int
		Fixtures               map[string]struct {
			Classes, ClassMetadata, SourceSHA256, MetadataSHA256 map[string]string
			Anonymous, AnonymousSHA256, TestClass                string
		}
		Cells []struct {
			API, TransportAPI, Route                                   string
			Native                                                     json.RawMessage
			NativeJSONSHA256, RawSHA256, DeploymentSHA256, OrgIdSHA256 string
			DeploymentID, SourceStatus                                 string
		}
	}
	raw, err := os.ReadFile("testdata/conformance/qualified_static_shadow.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(data.APIVersions, []string{"62.0", "67.0"}) ||
		!reflect.DeepEqual(data.Routes, []string{"anonymous", "istest"}) ||
		data.NativeObservationCount != 4 || len(data.Cells) != 4 || len(data.Fixtures) != 2 {
		t.Fatal("changed captured source/context matrix")
	}
	hash := func(source []byte) string {
		sum := sha256.Sum256(source)
		return hex.EncodeToString(sum[:])
	}
	checkReceiptDigest := func(value string) {
		t.Helper()
		decoded, err := hex.DecodeString(value)
		if err != nil || len(decoded) != sha256.Size {
			t.Fatalf("invalid native receipt digest %q", value)
		}
	}
	seen := map[string]bool{}
	for _, cell := range data.Cells {
		key := cell.API + "/" + cell.Route
		if seen[key] || (cell.API != "62.0" && cell.API != "67.0") ||
			(cell.Route != "anonymous" && cell.Route != "istest") || cell.TransportAPI != cell.API {
			t.Fatal("invalid or repeated native cell", key)
		}
		seen[key] = true
		t.Run(key, func(t *testing.T) {
			// Raw receipts remain separate from the owned replay packet. Validate
			// their provenance digests here; recompute embedded source/value hashes.
			checkReceiptDigest(cell.RawSHA256)
			checkReceiptDigest(cell.DeploymentSHA256)
			checkReceiptDigest(cell.OrgIdSHA256)
			if cell.DeploymentID == "" || (cell.Route == "istest" && cell.SourceStatus != "intentional assertion observation") ||
				(cell.Route == "anonymous" && cell.SourceStatus != "success/debug observation") {
				t.Fatal("missing native route provenance")
			}
			var want any
			if err := json.Unmarshal(cell.Native, &want); err != nil {
				t.Fatal(err)
			}
			canonical, err := json.Marshal(want)
			if err != nil || hash(canonical) != cell.NativeJSONSHA256 {
				t.Fatal("changed native observation digest")
			}
			fixture := data.Fixtures[cell.API]
			className := "CollectionScopeProbe" + strings.TrimSuffix(cell.API, ".0")
			if fixture.TestClass != className+"Test" || len(fixture.Classes) != 2 ||
				fixture.Classes[className] == "" || fixture.Classes[fixture.TestClass] == "" ||
				len(fixture.ClassMetadata) != 2 || len(fixture.SourceSHA256) != 2 || len(fixture.MetadataSHA256) != 2 ||
				hash([]byte(fixture.Anonymous)) != fixture.AnonymousSHA256 ||
				!strings.Contains(fixture.Anonymous, className+".observe()") {
				t.Fatal("incomplete captured source route")
			}
			for name, source := range fixture.Classes {
				if hash([]byte(source)) != fixture.SourceSHA256[name] ||
					hash([]byte(fixture.ClassMetadata[name])) != fixture.MetadataSHA256[name] ||
					!strings.Contains(fixture.ClassMetadata[name], "<apiVersion>"+cell.API+"</apiVersion>") {
					t.Fatal("changed captured source or source version", name)
				}
			}
			index := nativeBoundaryIndex(t, cell.API, fixture.Classes, nil)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native accepted fixture: index=%v analysis=%v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = cell.API
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			var observation string
			if cell.Route == "anonymous" {
				analysis := sema.AnalyzeAnonymous(index, fixture.Anonymous, cell.API)
				program, err := vm.CompileAnonymousWithOptions(fixture.Anonymous, vm.CompileOptions{APIVersion: cell.API})
				if analysis.HasErrors() || err != nil {
					t.Fatalf("native anonymous executes: analysis=%v compile=%v", analysis.Diagnostics, err)
				}
				result, err := runner.newMachine().Execute(program)
				if err != nil {
					t.Fatal(err)
				}
				for _, line := range result.Debug {
					if strings.Contains(line, "SCOPE_ROWS|") {
						if observation != "" {
							t.Fatal("duplicate anonymous observation")
						}
						observation = line
					}
				}
			} else {
				cases := Discover(index, Options{SelectedClasses: []string{fixture.TestClass}, SelectedMethod: "observe"})
				if len(cases) != 1 {
					t.Fatalf("named observation discovery = %d, want 1", len(cases))
				}
				methods, methodErrors := compileTestMethodsForIndex(&index, cases)
				programs, invokeErrors := compileTestInvokePrograms(cases)
				key := testCaseKey(cases[0])
				if methodErrors[key] != nil || invokeErrors[key] != nil {
					t.Fatalf("named compile: %v %v", methodErrors[key], invokeErrors[key])
				}
				machine := runner.newMachine()
				if err := machine.RegisterMethod(methods[key]); err != nil {
					t.Fatal(err)
				}
				machine.EnableTestContext()
				_, execErr := machine.ExecuteInClass(programs[key], cases[0].ClassName)
				var fault *vm.RuntimeError
				if !errors.As(execErr, &fault) || fault.Type != "System.AssertException" {
					t.Fatalf("missing named observation assertion: %v", execErr)
				}
				observation = fault.ExceptionMessage()
			}
			_, payload, found := strings.Cut(observation, "SCOPE_ROWS|")
			payload, _, ended := strings.Cut(payload, "|SCOPE_END")
			if !found || !ended || strings.Count(observation, "SCOPE_ROWS|") != 1 {
				t.Fatalf("missing exact observation boundary: %q", observation)
			}
			var got any
			if err := json.Unmarshal([]byte(payload), &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("native=%s\nlocal=%s", canonical, payload)
			}
		})
	}
}
