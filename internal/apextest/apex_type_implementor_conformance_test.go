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

type apexImplementorNativeCapture struct {
	Context, SourceAPI, HelperAPI, Route, ProjectAPI, TransportAPI, NamespaceContext, TestClass string
	SourceManifestRaw, SourceManifestSHA256 string; OrgRun string `json:"runMetadata"`
	SourceManifest                                                                              struct {
		Phase                   string
		HelperSetupDeploymentID string
		Files                   map[string]string
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

func TestApexTypeImplementorNativeConformance(t *testing.T) {
	var data struct {
		SourceAPIVersions, Routes, PendingCoverage               []string
		NamespaceContext, Marker, OrgIDSHA256, ReceiptPathPolicy string
		KnownRows                                                int
		Captures                                                 []apexImplementorNativeCapture
		HelperSetup                                              struct {
			DeploymentID, Status, SourceManifestRaw, SourceManifestSHA256 string; OrgRun string `json:"runMetadata"`
			Done, CheckOnly                                                       bool
			NumberComponentErrors, NumberTestErrors                               int
			RawSHA256, SourceDigests                                              map[string]string
		}
		CarriedObservations []struct {
			apexImplementorNativeCapture
			Reason   string
			Response struct {
				Status  int
				Name    string
				Message string
				Data    struct {
					Success, Compiled                bool
					CompileProblem, ExceptionMessage string
				}
			}
		}
	}
	raw, err := os.ReadFile("testdata/conformance/apex_type_implementor.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(data.SourceAPIVersions, []string{"61.0", "62.0", "67.0"}) || !reflect.DeepEqual(data.Routes, []string{"isTest", "anonymous"}) || data.NamespaceContext != "none" || data.Marker != "IMPLEMENTOR_OBSERVATION|" || data.KnownRows != 6 || len(data.Captures) != 6 || len(data.PendingCoverage) != 6 || data.ReceiptPathPolicy == "" {
		t.Fatal("native implementor scope differs")
	}
	setup := data.HelperSetup
	if setup.DeploymentID == "" || setup.Status != "Succeeded" || !setup.Done || setup.CheckOnly || setup.NumberComponentErrors != 0 || setup.NumberTestErrors != 0 || !strings.Contains(setup.OrgRun, "orgIdSha256="+data.OrgIDSHA256) {
		t.Fatal("native helper setup did not succeed")
	}
	setupSum := sha256.Sum256([]byte(setup.SourceManifestRaw))
	var setupManifest struct {
		Phase string
		Files map[string]string
	}
	if hex.EncodeToString(setupSum[:]) != setup.SourceManifestSHA256 || setup.RawSHA256["source-digests.json"] != setup.SourceManifestSHA256 || json.Unmarshal([]byte(setup.SourceManifestRaw), &setupManifest) != nil || setupManifest.Phase != "before-native-call" || !reflect.DeepEqual(setupManifest.Files, setup.SourceDigests) || len(setup.SourceDigests) != 11 {
		t.Fatal("native helper setup source manifest differs")
	}
	for _, name := range []string{"deploy.json", "source-digests.json", "ORG-RUN.txt"} {
		assertImplementorDigest(t, setup.RawSHA256[name])
	}
	want := map[string]string{"named61": "61.0", "named62": "62.0", "named67": "67.0", "anonymous61": "61.0", "anonymous62": "62.0", "anonymous67": "67.0"}
	count := 0
	for _, capture := range data.Captures {
		t.Run(capture.Context, func(t *testing.T) {
			if want[capture.Context] != capture.SourceAPI || capture.HelperAPI != "67.0" || capture.NamespaceContext != "none" {
				t.Fatal("native caller/helper scope differs")
			}
			delete(want, capture.Context)
			root := validateImplementorFrozenSources(t, capture, data.OrgIDSHA256, setup.SourceDigests)
			actual := replayImplementorNative(t, capture, root, data.Marker, setup.DeploymentID)
			if len(capture.Rows) != 1 || capture.Rows[0].ID != "observe" || actual == nil || !reflect.DeepEqual(actual, capture.Rows[0].Native) {
				t.Errorf("native=%v; Glade=%v", capture.Rows, actual)
			}
			count++
		})
	}
	if count != data.KnownRows || len(want) != 0 {
		t.Fatal("native row coverage differs")
	}
	// Retain the independent access-level rejection without asserting Glade parity
	// for that unimplemented compile boundary or dropping the original observation.
	if len(data.CarriedObservations) != 1 {
		t.Fatal("native access-level carry absent")
	}
	carry := data.CarriedObservations[0]
	validateImplementorFrozenSources(t, carry.apexImplementorNativeCapture, data.OrgIDSHA256, setup.SourceDigests)
	if carry.Context != "anonymous67_system_mode_rejected" || carry.SourceAPI != "67.0" || carry.TransportAPI != "67.0" || carry.HelperAPI != "67.0" || carry.Route != "anonymous" || carry.ProjectAPI != "62.0" || carry.NamespaceContext != "none" || carry.SourceManifest.HelperSetupDeploymentID != setup.DeploymentID || carry.Reason == "" || len(carry.Rows) != 0 || carry.Response.Status != 1 || carry.Response.Name != "executeCompileFailure" || carry.Response.Data.Compiled || carry.Response.Data.Success || carry.Response.Data.CompileProblem != "Cannot use SYSTEM_MODE access level in anonymous execution of Apex." || !strings.Contains(carry.Sources["source.apex"].Source, "WITH SYSTEM_MODE") {
		t.Fatal("native access-level carry differs")
	}
	assertImplementorDigest(t, carry.RawSHA256["run.json"])
}

func assertImplementorDigest(t *testing.T, value string) {
	t.Helper()
	digest, err := hex.DecodeString(value)
	if err != nil || len(digest) != sha256.Size {
		t.Fatal("native raw digest absent or malformed")
	}
}

func validateImplementorFrozenSources(t *testing.T, capture apexImplementorNativeCapture, orgHash string, helperDigests map[string]string) string {
	t.Helper()
	if !strings.Contains(capture.OrgRun, "orgIdSha256="+orgHash) || !strings.Contains(capture.OrgRun, "sourceApi="+capture.SourceAPI) || !strings.Contains(capture.OrgRun, "transportApi="+capture.TransportAPI) {
		t.Fatal("D7 source/org pins differ")
	}
	for _, name := range []string{"org.tsv", "errors.json", "ORG-RUN.txt", "source-digests.json"} {
		assertImplementorDigest(t, capture.RawSHA256[name])
	}
	if len(capture.Command.Command) == 0 || capture.Command.Command[0] != "sf" || !strings.Contains(strings.Join(capture.Command.Command, " "), "--api-version "+capture.TransportAPI) {
		t.Fatal("command transport pin differs")
	}
	manifestSum := sha256.Sum256([]byte(capture.SourceManifestRaw))
	var manifest struct {
		Phase string
		Files map[string]string
	}
	if capture.SourceManifest.Phase != "before-native-call" || hex.EncodeToString(manifestSum[:]) != capture.SourceManifestSHA256 || capture.RawSHA256["source-digests.json"] != capture.SourceManifestSHA256 || json.Unmarshal([]byte(capture.SourceManifestRaw), &manifest) != nil || manifest.Phase != capture.SourceManifest.Phase || !reflect.DeepEqual(manifest.Files, capture.SourceManifest.Files) || len(capture.Sources) != len(manifest.Files) {
		t.Fatal("original pre-call source manifest differs")
	}
	wantFiles := 13
	if capture.Route == "anonymous" {
		wantFiles = 12
	} else if capture.SourceAPI == "67.0" {
		wantFiles = 11
	}
	if len(capture.Sources) != wantFiles {
		t.Fatal("frozen source count differs")
	}
	root := t.TempDir()
	for name, source := range capture.Sources {
		sum := sha256.Sum256([]byte(source.Source))
		if filepath.IsAbs(name) || strings.Contains(name, "..") || source.Source == "" || hex.EncodeToString(sum[:]) != source.SHA256 || manifest.Files[name] != source.SHA256 {
			t.Fatal("frozen source digest differs")
		}
		writeFile(t, filepath.Join(root, name), source.Source)
	}
	for name, digest := range helperDigests {
		if capture.Sources[name].SHA256 != digest {
			t.Fatal("helper differs from installed synthetic source")
		}
	}
	for _, class := range []string{"AtlasContract", "AtlasProvider", "AtlasPlain", "GlyphFixtures", "SlateFixtures"} {
		var metadata struct {
			API string `xml:"apiVersion"`
		}
		path := "force-app/main/default/classes/" + class + ".cls-meta.xml"
		if xml.Unmarshal([]byte(capture.Sources[path].Source), &metadata) != nil || metadata.API != capture.HelperAPI {
			t.Fatal("helper metadata API differs")
		}
	}
	var project struct {
		API string `json:"sourceApiVersion"`
	}
	if json.Unmarshal([]byte(capture.Sources["sfdx-project.json"].Source), &project) != nil || project.API != capture.ProjectAPI {
		t.Fatal("project metadata API differs")
	}
	return root
}

func replayImplementorNative(t *testing.T, capture apexImplementorNativeCapture, root, marker, setupID string) map[string]any {
	t.Helper()
	anonymous := capture.Route == "anonymous"
	if anonymous {
		e := capture.Execution
		if capture.ProjectAPI != "62.0" || capture.TransportAPI != capture.SourceAPI || capture.TestClass != "" || capture.SourceManifest.HelperSetupDeploymentID != setupID || e.Status != 0 || !e.Compiled || !e.Success || e.CompileProblem != "" || e.ExceptionMessage != "" || capture.Errors.ExitCode != 0 || !capture.Errors.Compiled || !capture.Errors.Success || capture.Errors.CompileProblem != "" || capture.Errors.ExceptionMessage != "" || strings.Contains(capture.Sources["source.apex"].Source, "WITH SYSTEM_MODE") {
			t.Fatal("anonymous native execution incomplete")
		}
		assertImplementorDigest(t, capture.RawSHA256["run.json"])
	} else {
		d := capture.Deployment
		wantClass := "ImplementorObservation"
		if capture.SourceAPI == "67.0" {
			wantClass = "GlyphFixtures"
		}
		if capture.Route != "isTest" || capture.ProjectAPI != "62.0" || capture.TransportAPI != "62.0" || capture.TestClass != wantClass || d.ID == "" || !d.Done || !d.CheckOnly || d.Status != "Failed" || d.NumberComponentErrors != 0 || d.NumberTestErrors != 1 || d.NumberTestsCompleted != 0 || d.NumberTestsTotal != 1 || len(capture.Errors.ComponentFailures) != 0 || len(capture.Errors.TransportOrSetupErrors) != 0 {
			t.Fatal("named check-only observation incomplete")
		}
		var metadata struct {
			API string `xml:"apiVersion"`
		}
		if xml.Unmarshal([]byte(capture.Sources["force-app/main/default/classes/"+wantClass+".cls-meta.xml"].Source), &metadata) != nil || metadata.API != capture.SourceAPI {
			t.Fatal("observer metadata API differs")
		}
		assertImplementorDigest(t, capture.RawSHA256["deploy.json"])
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
	if len(capture.Rows) != 1 || !reflect.DeepEqual(decode(capture.Rows[0].NativeFailure.Message), capture.Rows[0].Native) {
		t.Fatal("native assertion payload differs")
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
