package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

type emailFieldCaseCell struct {
	APIVersion, Route, Stage, Anonymous, TestClass string
	RequestFiles, SourceSHA256                     map[string]string
	NativeEnvelopeJSON, NativeEnvelopeSHA256       string
	StdoutSHA256, StderrSHA256                     string
	Observation                                    struct {
		Outcome, ExceptionType, ExceptionMessage string
		Value                                    json.RawMessage
		CaseTableRow                             string `json:"case_table_row"`
		API                                      string `json:"api"`
		InvocationRoute                          string `json:"invocation_route"`
	}
}

// These native rows retain the caller, stored record, and before-trigger views
// separately. B07's two wrappers both escape with AssertException natively;
// keeping them active exposes any local catchability mismatch.
func TestEmailFieldCaseOrgConformance(t *testing.T) {
	var data struct {
		APIVersions, Routes                []string
		ShapeCount, NativeObservationCount int
		FixtureSourcesByAPI                map[string]map[string]string
		FixtureSourceSHA256ByAPI           map[string]map[string]string
		Rows                               []struct {
			ID    string
			Cells []emailFieldCaseCell
		}
	}
	raw, err := os.ReadFile("testdata/conformance/email_field_case.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" || strings.Join(data.Routes, ",") != "anonymous,isTest" ||
		data.ShapeCount != 9 || len(data.Rows) != 9 || data.NativeObservationCount != 54 {
		t.Fatal("incomplete native Email/Text matrix")
	}
	wantIDs := map[string]bool{"B01": true, "B02": true, "B03": true, "B04": true, "B05": true, "B06": true, "B07_RAW": true, "B07_CATCH": true, "B08": true}
	for _, row := range data.Rows {
		if !wantIDs[row.ID] || len(row.Cells) != 6 {
			t.Fatalf("invalid native row %s", row.ID)
		}
		delete(wantIDs, row.ID)
		seen := make(map[string]bool)
		for _, cell := range row.Cells {
			key := cell.APIVersion + "/" + cell.Route
			if seen[key] || cell.Observation.CaseTableRow != row.ID || cell.Observation.API != cell.APIVersion ||
				!strings.EqualFold(cell.Observation.InvocationRoute, cell.Route) {
				t.Fatalf("invalid/repeated native cell %s/%s", row.ID, key)
			}
			seen[key] = true
			wantUncaught := "uncaught_anonymous"
			if cell.Route == "isTest" {
				wantUncaught = "uncaught_istest"
			}
			if cell.Observation.Outcome != "runtime" && cell.Observation.Outcome != wantUncaught {
				t.Fatalf("unsupported/misrouted native outcome %q: %s/%s", cell.Observation.Outcome, row.ID, key)
			}
			if cell.Stage != "observed" {
				t.Fatalf("all captured Email/Text cells must stay active: %s/%s", row.ID, key)
			}
			if cell.Observation.Outcome == wantUncaught && (cell.Observation.ExceptionType != "System.AssertException" || cell.Observation.ExceptionMessage == "") {
				t.Fatalf("uncaught native assertion type/message missing: %s/%s", row.ID, key)
			}
			emailFieldCaseCheckHashes(t, cell.RequestFiles, cell.SourceSHA256, cell.Anonymous)
			emailFieldCaseCheckHash(t, cell.NativeEnvelopeJSON, cell.NativeEnvelopeSHA256)
			var envelope struct {
				StdoutSHA256 string `json:"stdout_sha256"`
				StderrSHA256 string `json:"stderr_sha256"`
			}
			if err := json.Unmarshal([]byte(cell.NativeEnvelopeJSON), &envelope); err != nil ||
				envelope.StdoutSHA256 != cell.StdoutSHA256 || envelope.StderrSHA256 != cell.StderrSHA256 {
				t.Fatalf("native transport hashes changed: %s/%s", row.ID, key)
			}
			emailFieldCaseVerifyNativeObservation(t, row.ID, cell)
		}
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			fixture := data.FixtureSourcesByAPI[api]
			if len(fixture) != 7 {
				t.Fatal("captured Email helper/object/two-trigger source matrix incomplete")
			}
			emailFieldCaseCheckHashes(t, fixture, data.FixtureSourceSHA256ByAPI[api], "")
			counts := newConformanceCountLog(t, "EmailFieldCase", api)
			for _, row := range data.Rows {
				t.Run(row.ID, func(t *testing.T) {
					for _, route := range data.Routes {
						t.Run(route, func(t *testing.T) {
							var selected *emailFieldCaseCell
							for i := range row.Cells {
								if cell := &row.Cells[i]; cell.APIVersion == api && cell.Route == route {
									selected = cell
									break
								}
							}
							if selected == nil {
								t.Fatal("missing native cell")
							}
							counts.TrackKind(t, route, "exact", 1)
							emailFieldCaseExecute(t, row.ID, *selected, fixture)
						})
					}
				})
			}
		})
	}
}

func emailFieldCaseCheckHashes(t *testing.T, files, hashes map[string]string, anonymous string) {
	t.Helper()
	wantHashes := len(files)
	if anonymous != "" {
		wantHashes++
	}
	if wantHashes != len(hashes) {
		t.Fatal("captured source hashes incomplete")
	}
	for name, source := range files {
		emailFieldCaseCheckHash(t, source, hashes[name])
	}
	if anonymous != "" {
		emailFieldCaseCheckHash(t, anonymous, hashes["anonymous"])
	}
}

func emailFieldCaseCheckHash(t *testing.T, text, want string) {
	t.Helper()
	sum := sha256.Sum256([]byte(text))
	if hex.EncodeToString(sum[:]) != want {
		t.Fatal("captured source/native bytes changed")
	}
}

func emailFieldCaseVerifyNativeObservation(t *testing.T, id string, cell emailFieldCaseCell) {
	t.Helper()
	var envelope struct {
		Payload struct {
			Data   struct{ ExceptionMessage string }
			Result struct {
				Details struct {
					RunTestResult struct {
						Failures []struct{ Name, MethodName, Message string }
					}
				}
			}
		}
	}
	if err := json.Unmarshal([]byte(cell.NativeEnvelopeJSON), &envelope); err != nil {
		t.Fatal(err)
	}
	message := envelope.Payload.Data.ExceptionMessage
	if cell.Route == "isTest" {
		failures := envelope.Payload.Result.Details.RunTestResult.Failures
		if len(failures) != 1 || failures[0].Name != cell.TestClass || failures[0].MethodName != "observe" {
			t.Fatal("native named envelope must identify the captured singleton test")
		}
		message = failures[0].Message
	}
	marker := "C126_RESULT|" + id + "|"
	if cell.Observation.Outcome != "runtime" {
		if strings.Contains(message, "C126_RESULT|") || message != cell.Observation.ExceptionMessage || !strings.HasPrefix(message, cell.Observation.ExceptionType+": ") {
			t.Fatal("exported uncaught expectation differs from exact native envelope")
		}
		return
	}
	_, tail, found := strings.Cut(message, marker)
	if !found || strings.Count(message, "C126_RESULT|") != 1 || !strings.HasPrefix(message, "System.AssertException: ") {
		t.Fatal("native envelope has no unique assertion transport marker")
	}
	var native, expected map[string]any
	if err := json.Unmarshal([]byte(tail), &native); err != nil || len(native) == 0 {
		t.Fatal("native marker JSON is malformed or empty")
	}
	if err := json.Unmarshal(cell.Observation.Value, &expected); err != nil || !reflect.DeepEqual(native, expected) {
		t.Fatal("exported expectation differs from exact native marker JSON")
	}
}

func emailFieldCaseExecute(t *testing.T, id string, cell emailFieldCaseCell, fixture map[string]string) {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, cell.APIVersion))
	for _, files := range []map[string]string{fixture, cell.RequestFiles} {
		for name, source := range files {
			writeFile(t, filepath.Join(root, "force-app", name), source)
		}
	}
	index := loadTestIndex(t, root)
	if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("native compiles; local setup diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
	}
	runtime, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(runtime.Triggers) != 4 {
		t.Fatalf("captured two before-insert/update triggers require four event registrations; got %d", len(runtime.Triggers))
	}
	org := orgFromIndex(index)
	org.APIVersion = cell.APIVersion
	runner := newConformanceRunner(t, index, conformanceRunnerOptions{RequestRuntime: &runtime, Org: &org})
	machine := runner.newMachine()
	var execErr error
	if cell.Route == "anonymous" {
		if analysis := sema.AnalyzeAnonymous(index, cell.Anonymous, cell.APIVersion); analysis.HasErrors() {
			t.Fatalf("native compiles; local anonymous diagnostics=%v", analysis.Diagnostics)
		}
		program, err := vm.CompileAnonymousWithOptions(cell.Anonymous, vm.CompileOptions{APIVersion: cell.APIVersion})
		if err != nil {
			t.Fatal(err)
		}
		_, execErr = machine.Execute(program)
	} else {
		cases := Discover(index, Options{SelectedClasses: []string{cell.TestClass}, SelectedMethod: "observe"})
		if len(cases) != 1 {
			t.Fatalf("captured singleton named test discovery=%d", len(cases))
		}
		methods, methodErrors := compileTestMethods(cases)
		programs, programErrors := compileTestInvokePrograms(cases)
		key := testCaseKey(cases[0])
		if methodErrors[key] != nil || programErrors[key] != nil {
			t.Fatalf("named compile: %v %v", methodErrors[key], programErrors[key])
		}
		if err := machine.RegisterMethod(methods[key]); err != nil {
			t.Fatal(err)
		}
		machine.EnableTestContext()
		_, execErr = machine.ExecuteInClass(programs[key], cell.TestClass)
	}
	var runtimeErr *vm.RuntimeError
	if execErr == nil || !errors.As(execErr, &runtimeErr) {
		t.Fatalf("native exception/transport missing: %v", execErr)
	}
	if cell.Observation.Outcome != "runtime" {
		if strings.Contains(runtimeErr.ExceptionMessage(), "C126_RESULT|") || !nativeObservationFailureMatches(runtimeErr, cell.Observation.ExceptionType, cell.Observation.ExceptionMessage) {
			t.Fatalf("native uncaught %s; local %v", cell.Observation.ExceptionMessage, execErr)
		}
		return
	}
	marker := "C126_RESULT|" + id + "|"
	_, tail, found := strings.Cut(runtimeErr.ExceptionMessage(), marker)
	if runtimeErr.Type != "System.AssertException" || !found || strings.Count(runtimeErr.ExceptionMessage(), "C126_RESULT|") != 1 {
		t.Fatalf("wrong native observation transport: %v", execErr)
	}
	var observed, expected map[string]any
	if err := json.Unmarshal([]byte(tail), &observed); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(cell.Observation.Value, &expected); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(observed, expected) {
		t.Fatalf("native value %s; local value %s", cell.Observation.Value, tail)
	}
}
