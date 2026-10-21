package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"html"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestPrimitiveConstructorNativeConformance(t *testing.T) {
	var data struct {
		APIs           []string `json:"apiVersions"`
		Transport      string   `json:"transportApi"`
		Route          string
		Namespace      string   `json:"namespaceContext"`
		Known          int      `json:"knownRows"`
		Pending        []string `json:"pendingCoverage"`
		KnownAnonymous int      `json:"knownAnonymousRows"`
		Anonymous      []struct {
			Context, SourceAPI, CallerAPI, TransportAPI, Route, NamespaceContext, Cell, Marker string
			Source                                                                             struct{ Source, SHA256 string }
			Execution                                                                          struct {
				Success, Compiled                                     bool
				CompileProblem, ExceptionMessage, ExceptionStackTrace string
			}
			Native        map[string]any
			NativeLogLine string
			RawSHA256     map[string]string
		} `json:"anonymousCaptures"`
		Setup []struct {
			Status, Cell, SourceAPI, CallerAPI, TransportAPI, Route, NamespaceContext, DeploymentID string
			CheckOnly                                                                               bool
			Observed                                                                                int                        `json:"observedRuntimeRows"`
			Failures                                                                                []struct{ Problem string } `json:"componentFailures"`
		} `json:"setupCarries"`
		Captures []struct {
			Context, SourceAPI, CallerAPI, TestClass, Marker string
			Deployment                                       struct {
				ID, Status                                                                      string
				Done, CheckOnly                                                                 bool
				NumberComponentErrors, NumberTestErrors, NumberTestsCompleted, NumberTestsTotal int
			}
			Sources   map[string]struct{ Source, SHA256 string }
			RawSHA256 map[string]string
			Rows      []struct {
				Method        string
				Native        map[string]any
				CarriedPaths  map[string]string
				NativeFailure struct{ Message string }
			}
		}
	}
	raw, err := os.ReadFile("testdata/conformance/primitive_constructor_native.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(data.APIs, []string{"62.0", "63.0", "67.0"}) || data.Transport != "62.0" || data.Route != "isTest" || data.Namespace != "none" || data.Known != 24 || data.KnownAnonymous != 12 || len(data.Anonymous) != 12 || len(data.Captures) != 3 || len(data.Pending) == 0 {
		t.Fatal("native scope differs")
	}
	if len(data.Setup) != 1 {
		t.Fatal("qualified primitive setup carry missing")
	}
	carry := data.Setup[0]
	if carry.Status != "observed-setup-boundary" || carry.Cell != "integerQualified" || carry.SourceAPI != "63.0" || carry.CallerAPI != "63.0" || carry.TransportAPI != "62.0" || carry.Route != "@IsTest" || carry.NamespaceContext != "none" || carry.DeploymentID == "" || !carry.CheckOnly || carry.Observed != 0 || len(carry.Failures) != 1 || carry.Failures[0].Problem != "Invalid type: System.Integer" {
		t.Fatal("qualified primitive setup boundary differs")
	}
	rows := 0
	expectedAPIs := map[string]string{"api62-runtime": "62.0", "api63-runtime": "63.0", "api67-runtime": "67.0"}
	for _, capture := range data.Captures {
		t.Run(capture.Context, func(t *testing.T) {
			if expectedAPIs[capture.Context] != capture.SourceAPI || capture.CallerAPI != capture.SourceAPI {
				t.Fatal("source/caller scope differs")
			}
			delete(expectedAPIs, capture.Context)
			d := capture.Deployment
			if d.ID == "" || !d.Done || !d.CheckOnly || d.Status != "Failed" || d.NumberComponentErrors != 0 || d.NumberTestsCompleted != 0 || d.NumberTestErrors != 8 || d.NumberTestsTotal != 8 || len(capture.Rows) != 8 {
				t.Fatal("native deployment incomplete")
			}
			for _, name := range []string{"org.tsv", "deploy.json", "errors.json", "ORG-RUN.txt"} {
				hash, err := hex.DecodeString(capture.RawSHA256[name])
				if err != nil || len(hash) != sha256.Size {
					t.Fatalf("raw digest missing %s", name)
				}
			}
			root := t.TempDir()
			for name, source := range capture.Sources {
				sum := sha256.Sum256([]byte(source.Source))
				if filepath.IsAbs(name) || strings.Contains(name, "..") || source.Source == "" || hex.EncodeToString(sum[:]) != source.SHA256 {
					t.Fatal("source digest differs")
				}
				writeFile(t, filepath.Join(root, name), source.Source)
			}
			var project struct {
				API string `json:"sourceApiVersion"`
			}
			if err := json.Unmarshal([]byte(capture.Sources["sfdx-project.json"].Source), &project); err != nil || project.API != capture.SourceAPI {
				t.Fatal("project source API differs")
			}
			for _, name := range []string{"OwnedPrimitiveReflection", capture.TestClass} {
				var metadata struct {
					API string `xml:"apiVersion"`
				}
				if err := xml.Unmarshal([]byte(capture.Sources["force-app/main/default/classes/"+name+".cls-meta.xml"].Source), &metadata); err != nil || metadata.API != capture.SourceAPI {
					t.Fatal("class source API differs")
				}
			}
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native compiled; Glade diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = capture.SourceAPI
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			cases := Discover(index, Options{SelectedClasses: []string{capture.TestClass}})
			if len(cases) != 8 {
				t.Fatal("named discovery differs")
			}
			methods, methodErrors := compileTestMethodsForIndex(&index, cases)
			programs, programErrors := compileTestInvokePrograms(cases)
			decode := func(message string) map[string]any {
				_, payload, ok := strings.Cut(message, ", Actual: ")
				if !ok {
					t.Fatalf("missing observation assertion: %s", message)
				}
				payload, ok = strings.CutPrefix(payload, capture.Marker+" ")
				if !ok {
					t.Fatal("observation marker differs")
				}
				var got map[string]any
				if err := json.Unmarshal([]byte(payload), &got); err != nil {
					t.Fatal(err)
				}
				return got
			}
			seen := map[string]bool{}
			for _, row := range capture.Rows {
				t.Run(row.Method, func(t *testing.T) {
					if seen[row.Method] {
						t.Fatal("duplicate native cell")
					}
					seen[row.Method] = true
					rows++
					if !reflect.DeepEqual(decode(row.NativeFailure.Message), row.Native) {
						t.Fatal("native envelope differs")
					}
					key := ""
					for _, c := range cases {
						if c.MethodName == row.Method {
							key = testCaseKey(c)
						}
					}
					if key == "" || methodErrors[key] != nil || programErrors[key] != nil {
						t.Fatalf("named compilation differs: %v %v", methodErrors[key], programErrors[key])
					}
					machine := runner.newMachine()
					if err := machine.RegisterMethod(methods[key]); err != nil {
						t.Fatal(err)
					}
					machine.EnableTestContext()
					_, execErr := machine.ExecuteInClass(programs[key], capture.TestClass)
					var runtimeErr *vm.RuntimeError
					if !errors.As(execErr, &runtimeErr) || genericInterfaceCastExceptionType(runtimeErr.Type) != "System.AssertException" {
						t.Fatalf("observation assertion missing: %v", execErr)
					}
					got := decode(runtimeErr.ExceptionMessage())
					if len(got) != len(row.Native) {
						t.Fatalf("observation fields differ: native=%v; Glade=%v", row.Native, got)
					}
					for field, want := range row.Native {
						actual, exists := got[field]
						if !exists {
							t.Fatalf("missing observation field %s", field)
						}
						if reason, carried := row.CarriedPaths[field]; carried {
							if reason == "" || field != "message" || (row.Method != "interfaceLiteral" && row.Method != "abstractLiteral") {
								t.Fatal("unbounded carry")
							}
							continue
						}
						if !reflect.DeepEqual(actual, want) {
							t.Fatalf("field %s: native=%v; Glade=%v", field, want, actual)
						}
					}
				})
			}
		})
	}
	if rows != data.Known || len(expectedAPIs) != 0 {
		t.Fatal("native cell coverage differs")
	}
	anonymousSeen := map[string]bool{}
	for _, capture := range data.Anonymous {
		t.Run(capture.Context, func(t *testing.T) {
			validAPI := capture.SourceAPI == "62.0" || capture.SourceAPI == "63.0" || capture.SourceAPI == "67.0"
			validCell := capture.Cell == "integerLiteral" || capture.Cell == "integerLookup" || capture.Cell == "stringLiteral" || capture.Cell == "listLiteral"
			key := capture.SourceAPI + "/" + capture.Cell
			if !validAPI || !validCell || anonymousSeen[key] || capture.CallerAPI != capture.SourceAPI || capture.TransportAPI != capture.SourceAPI || capture.Route != "anonymous" || capture.NamespaceContext != "none" || capture.Marker != "PRIMITIVE_REFLECTION_OBSERVATION|" {
				t.Fatal("anonymous observation scope differs")
			}
			anonymousSeen[key] = true
			if !capture.Execution.Compiled || !capture.Execution.Success || capture.Execution.CompileProblem != "" || capture.Execution.ExceptionMessage != "" || capture.Execution.ExceptionStackTrace != "" {
				t.Fatal("anonymous observation did not complete")
			}
			sum := sha256.Sum256([]byte(capture.Source.Source))
			if capture.Source.Source == "" || hex.EncodeToString(sum[:]) != capture.Source.SHA256 {
				t.Fatal("anonymous source digest differs")
			}
			for _, name := range []string{"org.tsv", "run.json", "source.apex", "errors.json", "ORG-RUN.txt"} {
				hash, err := hex.DecodeString(capture.RawSHA256[name])
				if err != nil || len(hash) != sha256.Size {
					t.Fatalf("anonymous raw digest missing: %s", name)
				}
			}
			decode := func(message string) map[string]any {
				_, payload, ok := strings.Cut(html.UnescapeString(message), capture.Marker)
				if !ok {
					t.Fatal("anonymous observation marker missing")
				}
				var value map[string]any
				if err := json.Unmarshal([]byte(payload), &value); err != nil {
					t.Fatal(err)
				}
				return value
			}
			if !reflect.DeepEqual(decode(capture.NativeLogLine), capture.Native) {
				t.Fatal("native anonymous envelope differs")
			}
			index, masked, problem := typeTextAnonymous(t, capture.SourceAPI, capture.Source.Source)
			if problem != "" {
				t.Fatal(problem)
			}
			program, err := vm.CompileAnonymousWithOptions(masked, vm.CompileOptions{APIVersion: capture.SourceAPI})
			if err != nil {
				t.Fatal(err)
			}
			org := orgFromIndex(index)
			org.APIVersion = capture.SourceAPI
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			result, err := runner.execute(program)
			if err != nil {
				t.Fatal(err)
			}
			observed := 0
			for _, line := range result.Debug {
				if !strings.Contains(line, capture.Marker) {
					continue
				}
				observed++
				if got := decode(line); !reflect.DeepEqual(got, capture.Native) {
					t.Fatalf("native anonymous=%v; Glade=%v", capture.Native, got)
				}
			}
			if observed != 1 {
				t.Fatal("anonymous observation count differs")
			}
		})
	}
	if len(anonymousSeen) != data.KnownAnonymous {
		t.Fatal("anonymous native cell coverage differs")
	}
}
