package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

type staticInterfaceNativeRow struct {
	ID            string
	Native        map[string]any
	NativeFailure struct{ Message string }
}

type staticInterfaceNativeCapture struct {
	Context, SourceAPI, CallerAPI, ProjectAPI, TransportAPI, Route, NamespaceContext, TestClass, Marker, DeploymentReceipt string
	SourceAPIs                                                                                                             map[string]string
	Deployment                                                                                                             struct {
		ID, Status                                                                      string
		Done, CheckOnly                                                                 bool
		NumberComponentErrors, NumberTestErrors, NumberTestsCompleted, NumberTestsTotal int
	}
	Execution struct {
		Status                           int
		Success, Compiled                bool
		CompileProblem, ExceptionMessage string
	}
	Sources                  map[string]struct{ Source, SHA256 string }
	Source                   struct{ Source, SHA256 string }
	SourceDigests, RawSHA256 map[string]string
	Rows                     []staticInterfaceNativeRow
}

func TestStaticInterfaceDispatchNativeConformance(t *testing.T) {
	var data struct {
		APIs                         []string `json:"apiVersions"`
		ProjectAPI, NamespaceContext string
		Routes                       []string
		Known                        int      `json:"knownRows"`
		Pending                      []string `json:"pendingCoverage"`
		Captures                     []staticInterfaceNativeCapture
	}

	raw, err := os.ReadFile("testdata/conformance/static_interface_dispatch_native.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(data.APIs, []string{"52.0", "62.0", "67.0"}) || data.ProjectAPI != "62.0" || !reflect.DeepEqual(data.Routes, []string{"isTest", "anonymous"}) || data.NamespaceContext != "none" || data.Known != 37 || len(data.Captures) != 7 || len(data.Pending) == 0 {
		t.Fatal("native static-interface scope differs")
	}
	expected := map[string]string{"api52": "52.0", "api52-controls": "52.0", "api62": "62.0", "api67": "67.0", "anonymous-api52": "52.0", "anonymous-api62": "62.0", "anonymous-api67": "67.0"}
	rows := 0
	for _, capture := range data.Captures {
		t.Run(capture.Context, func(t *testing.T) {
			if expected[capture.Context] != capture.SourceAPI || capture.CallerAPI != capture.SourceAPI || capture.ProjectAPI != data.ProjectAPI || capture.NamespaceContext != data.NamespaceContext {
				t.Fatal("capture source/caller/transport scope differs")
			}
			delete(expected, capture.Context)
			wantRows := 6
			if capture.Context == "api52" {
				wantRows = 1
			}
			anonymous := capture.Route == "anonymous"
			if anonymous {
				if capture.TransportAPI != capture.SourceAPI || capture.TestClass != "" || capture.Marker != "STATIC_ACTION_OBSERVATION|" {
					t.Fatal("anonymous observation route differs")
				}
				d := capture.Deployment
				e := capture.Execution
				if d.ID == "" || !d.Done || d.CheckOnly || d.Status != "Succeeded" || d.NumberComponentErrors != 0 || d.NumberTestErrors != 0 || d.NumberTestsTotal != 0 || d.NumberTestsCompleted != 0 || e.Status != 0 || !e.Success || !e.Compiled || e.CompileProblem != "" || e.ExceptionMessage != "" {
					t.Fatal("native helper deployment or anonymous execution incomplete")
				}
			} else {
				if capture.Route != "isTest" || capture.TransportAPI != "62.0" || capture.TestClass != "StaticActionObservation" || capture.Marker != "STATIC_ACTION_OBSERVATION " {
					t.Fatal("named observation route differs")
				}
				d := capture.Deployment
				if d.ID == "" || !d.Done || !d.CheckOnly || d.Status != "Failed" || d.NumberComponentErrors != 0 || d.NumberTestsCompleted != 0 || d.NumberTestErrors != wantRows || d.NumberTestsTotal != wantRows {
					t.Fatal("incomplete native observation deployment")
				}
			}
			wantReceipt := "deploy.json"
			if anonymous {
				wantReceipt = "setup-result.json"
			} else if capture.Context == "api67" {
				wantReceipt = "report-1.json"
			}
			if capture.DeploymentReceipt != wantReceipt {
				t.Fatal("terminal native deployment receipt differs")
			}
			rawNames := []string{wantReceipt, "org.tsv", "errors.json", "ORG-RUN.txt", "source-digests.json"}
			if anonymous {
				setupRequestReceipt := "deploy-retry.json"
				if capture.Context == "anonymous-api52" {
					setupRequestReceipt = "deploy.json"
				}
				rawNames = append(rawNames, setupRequestReceipt, "run.json", "observe.apex")
			} else {
				rawNames = append(rawNames, "deploy.json")
			}
			for _, name := range rawNames {
				sum, err := hex.DecodeString(capture.RawSHA256[name])
				if err != nil || len(sum) != sha256.Size {
					t.Fatalf("missing raw digest %s", name)
				}
			}

			root := t.TempDir()
			for name, source := range capture.Sources {
				sum := sha256.Sum256([]byte(source.Source))
				if filepath.IsAbs(name) || strings.Contains(name, "..") || source.Source == "" || hex.EncodeToString(sum[:]) != source.SHA256 || capture.SourceDigests[name] != source.SHA256 {
					t.Fatal("captured source/D7 digest differs")
				}
				writeFile(t, filepath.Join(root, name), source.Source)
			}
			projectRoot := root
			prefix := ""
			if anonymous {
				projectRoot = filepath.Join(root, "project")
				prefix = "project/"
			}
			var project struct {
				API string `json:"sourceApiVersion"`
			}
			if err := json.Unmarshal([]byte(capture.Sources[prefix+"sfdx-project.json"].Source), &project); err != nil || project.API != capture.ProjectAPI {
				t.Fatal("project API pin differs")
			}
			classes := []string{"OwnedAction", "OwnedStaticAction", "OwnedInstanceAction"}
			if !anonymous {
				classes = append(classes, "StaticActionObservation")
			}
			if wantRows == 6 {
				classes = append(classes, "OwnedOtherStaticAction", "OwnedUnrelatedAction")
			}
			if len(capture.SourceAPIs) != len(classes) || len(capture.Sources) != 2*len(classes)+1 {
				t.Fatal("captured class set differs")
			}
			for _, name := range classes {
				var metadata struct {
					API string `xml:"apiVersion"`
				}
				if capture.SourceAPIs[name] != capture.SourceAPI {
					t.Fatal("helper source API differs")
				}
				if err := xml.Unmarshal([]byte(capture.Sources[prefix+"force-app/main/default/classes/"+name+".cls-meta.xml"].Source), &metadata); err != nil || metadata.API != capture.SourceAPI {
					t.Fatal("class metadata pin differs")
				}
			}
			index := loadTestIndex(t, projectRoot)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native compiled; Glade diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = capture.ProjectAPI
			actualRows := map[string]map[string]any{}

			decode := func(message string) map[string]any {
				t.Helper()
				_, payload, ok := strings.Cut(message, capture.Marker)
				if !ok {
					t.Fatalf("native assertion marker missing: %s", message)
				}
				var result map[string]any
				if err := json.Unmarshal([]byte(payload), &result); err != nil {
					t.Fatal(err)
				}
				return result
			}
			if anonymous {
				sourceSum := sha256.Sum256([]byte(capture.Source.Source))
				if capture.Source.Source == "" || hex.EncodeToString(sourceSum[:]) != capture.Source.SHA256 || capture.SourceDigests["observe.apex"] != capture.Source.SHA256 || capture.RawSHA256["observe.apex"] != capture.Source.SHA256 {
					t.Fatal("anonymous body source/D7 digest differs")
				}
				if analysis := sema.AnalyzeAnonymous(index, capture.Source.Source, capture.CallerAPI); analysis.HasErrors() {
					t.Fatalf("native compiled; anonymous diagnostics=%v", analysis.Diagnostics)
				}
				program, err := vm.CompileAnonymousWithOptions(capture.Source.Source, vm.CompileOptions{APIVersion: capture.CallerAPI})
				if err != nil {
					t.Fatal(err)
				}
				runtime, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
				if err != nil {
					t.Fatal(err)
				}
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{RequestRuntime: &runtime, Org: &org})
				result, err := runner.execute(program)
				if err != nil {
					t.Fatal(err)
				}
				for _, message := range result.Debug {
					if strings.HasPrefix(message, capture.Marker) {
						actualRows[strconv.Itoa(len(actualRows))] = decode(message)
					}
				}
			} else {
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				cases := Discover(index, Options{SelectedClasses: []string{capture.TestClass}})
				if len(cases) != wantRows {
					t.Fatal("named observation discovery differs")
				}
				methods, methodErrors := compileTestMethodsForIndex(&index, cases)
				programs, programErrors := compileTestInvokePrograms(cases)
				for _, c := range cases {
					key := testCaseKey(c)
					if methodErrors[key] != nil || programErrors[key] != nil {
						t.Fatal("named observation compilation differs")
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
					actualRows[c.MethodName] = decode(runtimeErr.Message)
				}
			}
			if len(actualRows) != wantRows {
				t.Fatal("replay observation count differs")
			}
			seen := map[string]bool{}

			for _, row := range capture.Rows {
				t.Run(row.ID, func(t *testing.T) {
					if seen[row.ID] {
						t.Fatal("duplicate native method")
					}
					seen[row.ID] = true
					rows++
					if !anonymous && !reflect.DeepEqual(decode(row.NativeFailure.Message), row.Native) {
						t.Fatal("native payload differs from assertion")
					}
					actual, ok := actualRows[row.ID]
					if !ok {
						t.Fatal("native row missing from replay")
					}

					if !reflect.DeepEqual(actual, row.Native) {
						t.Errorf("payload differs: native=%v; Glade=%v", row.Native, actual)
					}
				})
			}
		})
	}
	if rows != data.Known || len(expected) != 0 {
		t.Fatal("native static-interface row coverage differs")
	}
}
