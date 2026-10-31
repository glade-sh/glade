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
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

type staticFactoryReceiverCapture struct {
	Context, SourceAPI, HelperAPI, Route, ProjectAPI, TransportAPI, NamespaceContext, TestClass string
	SourceManifestRaw, SourceManifestSHA256                                                     string
	OrgRun                                                                                      string `json:"runMetadata"`
	SourceManifest                                                                              struct {
		Phase, HelperSetupDeploymentID string
		Files                          map[string]string
	}
	Sources    map[string]struct{ Source, SHA256 string }
	RawSHA256  map[string]string
	Command    struct{ Command []string }
	Deployment struct {
		ID, Status                                                                      string
		Done, CheckOnly                                                                 bool
		NumberComponentErrors, NumberTestErrors, NumberTestsCompleted, NumberTestsTotal int
	}
	Execution struct {
		Status                           int
		Success, Compiled                bool
		CompileProblem, ExceptionMessage string
	}
	Errors struct {
		ComponentFailures, TransportOrSetupErrors []json.RawMessage
		ExitCode                                  int
		Success, Compiled                         bool
		CompileProblem, ExceptionMessage          string
	}
	Rows []struct {
		ID            string
		Native        map[string]any
		NativeFailure struct{ Message string }
	}
}

type staticFactoryReceiverSetup struct {
	DeploymentID, Status, SourceManifestRaw, SourceManifestSHA256 string
	Done, CheckOnly                                               bool
	NumberComponentErrors, NumberTestErrors                       int
	RawSHA256, SourceDigests                                      map[string]string
	OrgRun                                                        string `json:"runMetadata"`
	ReceiptDiscrepancy                                            string
	Command                                                       struct{ Command []string }
}

func TestStaticFactoryReceiverNativeConformance(t *testing.T) {
	var data struct {
		SourceAPIVersions, Routes, PendingCoverage               []string
		NamespaceContext, Marker, OrgIDSHA256, ReceiptPathPolicy string
		KnownRows                                                int
		Captures                                                 []staticFactoryReceiverCapture
		HelperSetups                                             map[string]staticFactoryReceiverSetup
		ExcludedSetupAttempts                                    []struct {
			SourceAPI, Stage, ErrorName, SourceManifestSHA256 string
			AcceptedDeploymentID                              *string
			ExitCode, ConflictingFiles, OwnedHelperClasses    int
			RawSHA256                                         map[string]string
		}
	}
	raw, err := os.ReadFile("testdata/conformance/static_factory_receiver.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(data.SourceAPIVersions, []string{"62.0", "67.0"}) || !reflect.DeepEqual(data.Routes, []string{"isTest", "anonymous"}) || data.NamespaceContext != "none" || data.Marker != "RECEIVER_PRECEDENCE_OBSERVATION|" || data.KnownRows != 4 || len(data.Captures) != 4 || len(data.HelperSetups) != 2 || len(data.PendingCoverage) == 0 || data.ReceiptPathPolicy == "" {
		t.Fatal("native receiver scope differs")
	}
	if len(data.ExcludedSetupAttempts) != 1 {
		t.Fatal("excluded setup attempt provenance differs")
	}
	excluded := data.ExcludedSetupAttempts[0]
	if excluded.SourceAPI != "62.0" || excluded.Stage != "CLI-source-tracking-preflight" || excluded.ErrorName != "SourceConflictError" || excluded.AcceptedDeploymentID != nil || excluded.ExitCode != 1 || excluded.ConflictingFiles != 6 || excluded.OwnedHelperClasses != 3 || excluded.RawSHA256["source-digests.json"] != excluded.SourceManifestSHA256 {
		t.Fatal("preflight setup failure was not excluded")
	}
	for _, name := range []string{"deploy.json", "deploy.command.json", "source-digests.json", "ORG-RUN.txt"} {
		assertStaticFactoryReceiverDigest(t, excluded.RawSHA256[name])
	}
	for _, api := range data.SourceAPIVersions {
		setup, ok := data.HelperSetups[api]
		if !ok || setup.DeploymentID == "" || setup.Status != "Succeeded" || !setup.Done || setup.CheckOnly || setup.NumberComponentErrors != 0 || setup.NumberTestErrors != 0 || !strings.Contains(setup.OrgRun, "orgIdSha256="+data.OrgIDSHA256) || !strings.Contains(setup.OrgRun, "helperApi="+api) || !strings.Contains(setup.OrgRun, "transportApi=62.0") {
			t.Fatal("native helper setup incomplete")
		}
		if len(setup.Command.Command) == 0 || setup.Command.Command[0] != "sf" || !strings.Contains(strings.Join(setup.Command.Command, " "), "--test-level NoTestRun") || !strings.Contains(strings.Join(setup.Command.Command, " "), "--api-version 62.0") {
			t.Fatal("native helper setup command differs")
		}
		if api == "62.0" && (!strings.Contains(strings.Join(setup.Command.Command, " "), "--ignore-conflicts") || setup.ReceiptDiscrepancy == "") {
			t.Fatal("successful owned-helper update command provenance differs")
		}
		manifestSum := sha256.Sum256([]byte(setup.SourceManifestRaw))
		if hex.EncodeToString(manifestSum[:]) != setup.SourceManifestSHA256 || setup.RawSHA256["source-digests.json"] != setup.SourceManifestSHA256 {
			t.Fatal("helper setup manifest digest differs")
		}
		var manifest struct {
			Phase string
			Files map[string]string
		}
		if json.Unmarshal([]byte(setup.SourceManifestRaw), &manifest) != nil || manifest.Phase != "before-native-call" || len(manifest.Files) != 7 || !reflect.DeepEqual(manifest.Files, setup.SourceDigests) {
			t.Fatal("helper setup source manifest differs")
		}
		for _, name := range []string{"deploy.json", "source-digests.json", "ORG-RUN.txt"} {
			assertStaticFactoryReceiverDigest(t, setup.RawSHA256[name])
		}
	}
	want := map[string]string{"named62": "62.0", "anonymous62": "62.0", "named67": "67.0", "anonymous67": "67.0"}
	count := 0
	for _, capture := range data.Captures {
		t.Run(capture.Context, func(t *testing.T) {
			if want[capture.Context] != capture.SourceAPI || capture.HelperAPI != capture.SourceAPI || capture.NamespaceContext != "none" {
				t.Fatal("native source scope differs")
			}
			delete(want, capture.Context)
			actual := replayStaticFactoryReceiverNative(t, capture, data.Marker, data.OrgIDSHA256, data.HelperSetups[capture.SourceAPI])
			if len(capture.Rows) != 1 || actual == nil || capture.Rows[0].ID != "observe" || !reflect.DeepEqual(actual, capture.Rows[0].Native) {
				t.Fatalf("native=%v; Glade=%v", capture.Rows, actual)
			}
			count++
		})
	}
	if count != data.KnownRows || len(want) != 0 {
		t.Fatal("native row coverage differs")
	}
}

func assertStaticFactoryReceiverDigest(t *testing.T, value string) {
	t.Helper()
	digest, err := hex.DecodeString(value)
	if err != nil || len(digest) != sha256.Size {
		t.Fatal("native raw digest absent or malformed")
	}
}

func replayStaticFactoryReceiverNative(t *testing.T, capture staticFactoryReceiverCapture, marker, orgHash string, setup staticFactoryReceiverSetup) map[string]any {
	t.Helper()
	if len(capture.Rows) != 1 || capture.Rows[0].ID != "observe" {
		t.Fatal("native observation row differs")
	}
	anonymous := capture.Route == "anonymous"
	resultName, sourceCount := "deploy.json", 9
	if anonymous {
		resultName, sourceCount = "run.json", 8
		e := capture.Execution
		if capture.ProjectAPI != "62.0" || capture.TransportAPI != capture.SourceAPI || capture.TestClass != "" || e.Status != 0 || !e.Compiled || !e.Success || e.CompileProblem != "" || e.ExceptionMessage != "" || capture.Errors.ExitCode != 0 || !capture.Errors.Compiled || !capture.Errors.Success || capture.Errors.CompileProblem != "" || capture.Errors.ExceptionMessage != "" || capture.SourceManifest.HelperSetupDeploymentID != setup.DeploymentID || !strings.Contains(capture.OrgRun, "helperSetupDeploymentId="+setup.DeploymentID) {
			t.Fatal("anonymous native execution incomplete")
		}
	} else {
		d := capture.Deployment
		if capture.Route != "isTest" || capture.ProjectAPI != "62.0" || capture.TransportAPI != "62.0" || capture.TestClass != "OwnedReceiverObservation" || d.ID == "" || !d.Done || !d.CheckOnly || d.Status != "Failed" || d.NumberComponentErrors != 0 || d.NumberTestErrors != 1 || d.NumberTestsCompleted != 0 || d.NumberTestsTotal != 1 || len(capture.Errors.ComponentFailures) != 0 || len(capture.Errors.TransportOrSetupErrors) != 0 {
			t.Fatal("named native check-only observation incomplete")
		}
	}
	if !strings.Contains(capture.OrgRun, "orgIdSha256="+orgHash) || !strings.Contains(capture.OrgRun, "sourceApi="+capture.SourceAPI) || !strings.Contains(capture.OrgRun, "transportApi="+capture.TransportAPI) {
		t.Fatal("D7 source/org pins differ")
	}
	for _, name := range []string{resultName, "org.tsv", "errors.json", "ORG-RUN.txt", "source-digests.json"} {
		assertStaticFactoryReceiverDigest(t, capture.RawSHA256[name])
	}
	if len(capture.Command.Command) == 0 || capture.Command.Command[0] != "sf" || !strings.Contains(strings.Join(capture.Command.Command, " "), "--api-version "+capture.TransportAPI) {
		t.Fatal("command transport pin differs")
	}
	manifestSum := sha256.Sum256([]byte(capture.SourceManifestRaw))
	if capture.SourceManifest.Phase != "before-native-call" || hex.EncodeToString(manifestSum[:]) != capture.SourceManifestSHA256 || capture.RawSHA256["source-digests.json"] != capture.SourceManifestSHA256 {
		t.Fatal("pre-call source manifest digest differs")
	}
	var manifest struct {
		Phase, HelperSetupDeploymentID string
		Files                          map[string]string
	}
	if json.Unmarshal([]byte(capture.SourceManifestRaw), &manifest) != nil || !reflect.DeepEqual(manifest, capture.SourceManifest) || len(manifest.Files) != sourceCount || len(capture.Sources) != sourceCount {
		t.Fatal("frozen source manifest payload differs")
	}
	root := t.TempDir()
	for name, source := range capture.Sources {
		sum := sha256.Sum256([]byte(source.Source))
		ownedPath := name == "sfdx-project.json" || anonymous && name == "source.apex" || strings.HasPrefix(name, "force-app/main/default/classes/")
		if !ownedPath || filepath.IsAbs(name) || strings.Contains(name, "..") || source.Source == "" || hex.EncodeToString(sum[:]) != source.SHA256 || manifest.Files[name] != source.SHA256 {
			t.Fatal("frozen source digest differs")
		}
		writeFile(t, filepath.Join(root, name), source.Source)
	}
	for _, class := range []string{"TokenFactory", "AtlasBase", "GlyphChild"} {
		path := "force-app/main/default/classes/" + class + ".cls-meta.xml"
		var metadata struct {
			API string `xml:"apiVersion"`
		}
		if xml.Unmarshal([]byte(capture.Sources[path].Source), &metadata) != nil || metadata.API != capture.HelperAPI {
			t.Fatal("helper metadata API differs")
		}
	}
	for name, digest := range setup.SourceDigests {
		if capture.Sources[name].SHA256 != digest {
			t.Fatal("helper source differs from native setup")
		}
	}
	var projectMetadata struct {
		API string `json:"sourceApiVersion"`
	}
	if json.Unmarshal([]byte(capture.Sources["sfdx-project.json"].Source), &projectMetadata) != nil || projectMetadata.API != capture.ProjectAPI {
		t.Fatal("project metadata API differs")
	}
	if !anonymous {
		var metadata struct {
			API string `xml:"apiVersion"`
		}
		if xml.Unmarshal([]byte(capture.Sources["force-app/main/default/classes/OwnedReceiverObservation.cls-meta.xml"].Source), &metadata) != nil || metadata.API != capture.SourceAPI {
			t.Fatal("observer metadata API differs")
		}
	}
	index := loadTestIndex(t, root)
	analysis := sema.Analyze(index)
	if index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("native source compiled; diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
	}
	org := orgFromIndex(index)
	org.APIVersion = capture.ProjectAPI
	decode := func(message string) map[string]any {
		t.Helper()
		_, payload, ok := strings.Cut(message, marker)
		if !ok {
			t.Fatal("observer marker absent")
		}
		var value map[string]any
		if err := json.Unmarshal([]byte(payload), &value); err != nil {
			t.Fatal(err)
		}
		return value
	}
	if anonymous {
		source := capture.Sources["source.apex"].Source
		if analysis := sema.AnalyzeAnonymous(index, source, capture.SourceAPI); analysis.HasErrors() {
			t.Fatalf("native anonymous compiled; diagnostics=%v", analysis.Diagnostics)
		}
		program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: capture.SourceAPI})
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
		var actual map[string]any
		for _, message := range result.Debug {
			if strings.HasPrefix(message, marker) {
				if actual != nil {
					t.Fatal("repeated anonymous observation")
				}
				actual = decode(message)
			}
		}
		return actual
	}
	runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
	cases := Discover(index, Options{SelectedClasses: []string{capture.TestClass}})
	if len(cases) != 1 || cases[0].MethodName != "observe" {
		t.Fatal("observer discovery differs")
	}
	methods, methodErrors := compileTestMethodsForIndex(&index, cases)
	programs, programErrors := compileTestInvokePrograms(cases)
	key := testCaseKey(cases[0])
	if methodErrors[key] != nil || programErrors[key] != nil {
		t.Fatal("observer compilation differs")
	}
	if !reflect.DeepEqual(decode(capture.Rows[0].NativeFailure.Message), capture.Rows[0].Native) {
		t.Fatal("native assertion payload differs")
	}
	machine := runner.newMachine()
	if err := machine.RegisterMethod(methods[key]); err != nil {
		t.Fatal(err)
	}
	machine.EnableTestContext()
	_, execErr := machine.ExecuteInClass(programs[key], capture.TestClass)
	var runtimeErr *vm.RuntimeError
	if !errors.As(execErr, &runtimeErr) || genericInterfaceCastExceptionType(runtimeErr.Type) != "System.AssertException" {
		t.Fatalf("observer assertion absent: %v", execErr)
	}
	return decode(runtimeErr.Message)
}
