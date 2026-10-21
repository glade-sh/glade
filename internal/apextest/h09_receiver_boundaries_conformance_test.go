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

func TestH09ReceiverBoundaryNativeConformance(t *testing.T) {
	var data struct {
		SourceAPIVersions, Routes, PendingCoverage               []string
		NamespaceContext, Marker, OrgIDSHA256, ReceiptPathPolicy string
		KnownRows                                                int
		Captures                                                 []staticFactoryReceiverCapture
		HelperSetups                                             map[string]staticFactoryReceiverSetup
		ExcludedTruncatedObservations                            []struct {
			SourceAPI, Route, DeploymentID, Reason, SourceManifestSHA256 string
			CompleteRows                                                 int
			RawSHA256                                                    map[string]string
		}
	}
	raw, err := os.ReadFile("testdata/conformance/h09_receiver_boundaries.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(data.SourceAPIVersions, []string{"62.0", "67.0", "68.0"}) || !reflect.DeepEqual(data.Routes, []string{"isTest", "anonymous"}) || data.NamespaceContext != "none" || data.Marker != "RECEIVER_PRECEDENCE_OBSERVATION|" || data.KnownRows != 9 || len(data.Captures) != 6 || len(data.HelperSetups) != 3 || len(data.PendingCoverage) == 0 || data.ReceiptPathPolicy == "" {
		t.Fatal("native receiver scope differs")
	}
	if len(data.ExcludedTruncatedObservations) != 1 {
		t.Fatal("excluded truncated observation provenance differs")
	}
	excluded := data.ExcludedTruncatedObservations[0]
	if excluded.SourceAPI != "62.0" || excluded.Route != "isTest" || excluded.DeploymentID == "" || excluded.Reason == "" || excluded.CompleteRows != 0 || excluded.RawSHA256["source-digests.json"] != excluded.SourceManifestSHA256 {
		t.Fatal("truncated observation was not excluded")
	}
	for _, name := range []string{"deploy.json", "source-digests.json", "ORG-RUN.txt", "errors.json", "org.tsv"} {
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
		manifestSum := sha256.Sum256([]byte(setup.SourceManifestRaw))
		if hex.EncodeToString(manifestSum[:]) != setup.SourceManifestSHA256 || setup.RawSHA256["source-digests.json"] != setup.SourceManifestSHA256 {
			t.Fatal("helper setup manifest digest differs")
		}
		var manifest struct {
			Phase string
			Files map[string]string
		}
		if json.Unmarshal([]byte(setup.SourceManifestRaw), &manifest) != nil || manifest.Phase != "before-native-call" || len(manifest.Files) != 11 || !reflect.DeepEqual(manifest.Files, setup.SourceDigests) {
			t.Fatal("helper setup source manifest differs")
		}
		for _, name := range []string{"deploy.json", "source-digests.json", "ORG-RUN.txt"} {
			assertStaticFactoryReceiverDigest(t, setup.RawSHA256[name])
		}
	}
	want := map[string]string{"named62": "62.0", "anonymous62": "62.0", "named67": "67.0", "anonymous67": "67.0", "named68": "68.0", "anonymous68": "68.0"}
	count := 0
	for _, capture := range data.Captures {
		t.Run(capture.Context, func(t *testing.T) {
			if want[capture.Context] != capture.SourceAPI || capture.HelperAPI != capture.SourceAPI || capture.NamespaceContext != "none" {
				t.Fatal("native source scope differs")
			}
			delete(want, capture.Context)
			actual := replayH09ReceiverBoundaryNative(t, capture, data.Marker, data.OrgIDSHA256, data.HelperSetups[capture.SourceAPI])
			if len(actual) != len(capture.Rows) {
				t.Fatal("native/replay row count differs")
			}
			for _, row := range capture.Rows {
				if actual[row.ID] == nil || !reflect.DeepEqual(actual[row.ID], row.Native) {
					t.Errorf("%s: native=%v; Glade=%v", row.ID, row.Native, actual[row.ID])
				}
				count++
			}
		})
	}
	if count != data.KnownRows || len(want) != 0 {
		t.Fatal("native row coverage differs")
	}
}

func replayH09ReceiverBoundaryNative(t *testing.T, capture staticFactoryReceiverCapture, marker, orgHash string, setup staticFactoryReceiverSetup) map[string]map[string]any {
	t.Helper()
	expectedRows := 2
	if capture.Route == "anonymous" {
		expectedRows = 1
	}
	if len(capture.Rows) != expectedRows {
		t.Fatal("native observation row differs")
	}
	anonymous := capture.Route == "anonymous"
	resultName, sourceCount := "deploy.json", 13
	if anonymous {
		resultName, sourceCount = "run.json", 12
		e := capture.Execution
		if capture.ProjectAPI != "62.0" || capture.TransportAPI != capture.SourceAPI || capture.TestClass != "" || e.Status != 0 || !e.Compiled || !e.Success || e.CompileProblem != "" || e.ExceptionMessage != "" || capture.Errors.ExitCode != 0 || !capture.Errors.Compiled || !capture.Errors.Success || capture.Errors.CompileProblem != "" || capture.Errors.ExceptionMessage != "" || capture.SourceManifest.HelperSetupDeploymentID != setup.DeploymentID || !strings.Contains(capture.OrgRun, "helperSetupDeploymentId="+setup.DeploymentID) {
			t.Fatal("anonymous native execution incomplete")
		}
	} else {
		d := capture.Deployment
		if capture.Route != "isTest" || capture.ProjectAPI != "62.0" || capture.TransportAPI != "62.0" || capture.TestClass != "OwnedBoundaryObservation" || d.ID == "" || !d.Done || !d.CheckOnly || d.Status != "Failed" || d.NumberComponentErrors != 0 || d.NumberTestErrors != 2 || d.NumberTestsCompleted != 0 || d.NumberTestsTotal != 2 || len(capture.Errors.ComponentFailures) != 0 || len(capture.Errors.TransportOrSetupErrors) != 0 {
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
	for _, class := range []string{"TokenFactory", "AtlasBase", "GlyphChild", "AtlasFieldBase", "GlyphReceiver"} {
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
		if xml.Unmarshal([]byte(capture.Sources["force-app/main/default/classes/OwnedBoundaryObservation.cls-meta.xml"].Source), &metadata) != nil || metadata.API != capture.SourceAPI {
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
		return map[string]map[string]any{"observe": actual}
	}
	runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
	cases := Discover(index, Options{SelectedClasses: []string{capture.TestClass}})
	if len(cases) != 2 {
		t.Fatal("observer discovery differs")
	}
	methods, methodErrors := compileTestMethodsForIndex(&index, cases)
	programs, programErrors := compileTestInvokePrograms(cases)
	nativeRows := make(map[string]map[string]any)
	for _, row := range capture.Rows {
		if row.ID != "observe" && row.ID != "observeBoundaries" || nativeRows[row.ID] != nil {
			t.Fatal("native named method identity differs")
		}
		if !reflect.DeepEqual(decode(row.NativeFailure.Message), row.Native) {
			t.Fatal("native assertion payload differs")
		}
		nativeRows[row.ID] = row.Native
	}
	actual := make(map[string]map[string]any)
	for _, testCase := range cases {
		key := testCaseKey(testCase)
		if methodErrors[key] != nil || programErrors[key] != nil || nativeRows[testCase.MethodName] == nil {
			t.Fatal("observer compilation or method identity differs")
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
		actual[testCase.MethodName] = decode(runtimeErr.Message)
	}
	return actual
}
