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

type annotatedMethodNativeCapture struct {
	Context, SourceAPI, HelperAPI, Route, ProjectAPI, TransportAPI, NamespaceContext, TestClass       string
	SourceManifestSHA256, SourceManifestRaw string; OrgRun string `json:"runMetadata"`; SourceManifestKind, OriginalSourceManifestSHA256 string
	OmittedNonSourceFiles                                                                             int
	SourceManifest                                                                                    struct {
		Phase string
		Files map[string]string
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

func TestAnnotatedMethodCommentsNativeConformance(t *testing.T) {
	var data struct {
		SourceAPIVersions, Routes, PendingCoverage               []string
		NamespaceContext, Marker, OrgIDSHA256, ReceiptPathPolicy string
		KnownRows                                                int
		Captures                                                 []annotatedMethodNativeCapture
		HelperSetup                                              struct {
			DeploymentID, Status, SourceManifestRaw, SourceManifestSHA256 string
			Done, CheckOnly                                               bool
			NumberComponentErrors, NumberTestErrors                       int
			RawSHA256, SourceDigests                                      map[string]string
			OrgRun string `json:"runMetadata"`
		}
	}
	raw, err := os.ReadFile("testdata/conformance/annotated_method_comments.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(data.SourceAPIVersions, []string{"61.0", "62.0", "67.0"}) || !reflect.DeepEqual(data.Routes, []string{"isTest", "anonymous"}) || data.NamespaceContext != "none" || data.Marker != "ANNOTATED_METHOD_OBSERVATION|" || data.KnownRows != 6 || len(data.Captures) != 6 || len(data.PendingCoverage) == 0 || data.ReceiptPathPolicy == "" {
		t.Fatal("native method-comment scope differs")
	}
	setup := data.HelperSetup
	if setup.DeploymentID == "" || setup.Status != "Succeeded" || !setup.Done || setup.CheckOnly || setup.NumberComponentErrors != 0 || setup.NumberTestErrors != 0 || !strings.Contains(setup.OrgRun, "orgIdSha256="+data.OrgIDSHA256) {
		t.Fatal("native helper setup did not succeed")
	}
	setupSum := sha256.Sum256([]byte(setup.SourceManifestRaw))
	if hex.EncodeToString(setupSum[:]) != setup.SourceManifestSHA256 || setup.RawSHA256["source-digests.json"] != setup.SourceManifestSHA256 {
		t.Fatal("helper setup source manifest differs")
	}
	var setupManifest struct {
		Phase string
		Files map[string]string
	}
	if json.Unmarshal([]byte(setup.SourceManifestRaw), &setupManifest) != nil || setupManifest.Phase != "before-native-call" || !reflect.DeepEqual(setupManifest.Files, setup.SourceDigests) || len(setup.SourceDigests) != 5 {
		t.Fatal("helper setup frozen sources differ")
	}
	for _, name := range []string{"deploy.json", "source-digests.json", "ORG-RUN.txt"} {
		assertAnnotatedMethodDigest(t, setup.RawSHA256[name])
	}
	want := map[string]string{"named61": "61.0", "named62": "62.0", "named67": "67.0", "anonymous61": "61.0", "anonymous62": "62.0", "anonymous67": "67.0"}
	count := 0
	for _, capture := range data.Captures {
		t.Run(capture.Context, func(t *testing.T) {
			if want[capture.Context] != capture.SourceAPI || capture.HelperAPI != "67.0" || capture.NamespaceContext != "none" {
				t.Fatal("native caller/helper scope differs")
			}
			delete(want, capture.Context)
			actual := replayAnnotatedMethodNative(t, capture, data.Marker, data.OrgIDSHA256, setup.SourceDigests)
			if len(capture.Rows) != 1 || actual == nil {
				t.Fatal("native/replay row count differs")
			}
			row := capture.Rows[0]
			if row.ID != "observe" || !reflect.DeepEqual(actual, row.Native) {
				t.Errorf("native=%v; Glade=%v", row.Native, actual)
			}
			count++
		})
	}
	if count != data.KnownRows || len(want) != 0 {
		t.Fatal("native row coverage differs")
	}
}

func assertAnnotatedMethodDigest(t *testing.T, value string) {
	t.Helper()
	digest, err := hex.DecodeString(value)
	if err != nil || len(digest) != sha256.Size {
		t.Fatal("native raw digest absent or malformed")
	}
}

func replayAnnotatedMethodNative(t *testing.T, capture annotatedMethodNativeCapture, marker, orgHash string, helperDigests map[string]string) map[string]any {
	t.Helper()
	anonymous := capture.Route == "anonymous"
	resultName := "deploy.json"
	if anonymous {
		resultName = "run.json"
		e := capture.Execution
		if capture.ProjectAPI != "62.0" || capture.TransportAPI != capture.SourceAPI || capture.TestClass != "" || e.Status != 0 || !e.Compiled || !e.Success || e.CompileProblem != "" || e.ExceptionMessage != "" || capture.Errors.ExitCode != 0 || !capture.Errors.Compiled || !capture.Errors.Success || capture.Errors.CompileProblem != "" || capture.Errors.ExceptionMessage != "" {
			t.Fatal("anonymous native execution incomplete")
		}
	} else {
		d := capture.Deployment
		if capture.Route != "isTest" || capture.ProjectAPI != "62.0" || capture.TransportAPI != "62.0" || capture.TestClass != "AnnotatedMethodObservation" || d.ID == "" || !d.Done || !d.CheckOnly || d.Status != "Failed" || d.NumberComponentErrors != 0 || d.NumberTestErrors != 1 || d.NumberTestsCompleted != 0 || d.NumberTestsTotal != 1 || len(capture.Errors.ComponentFailures) != 0 || len(capture.Errors.TransportOrSetupErrors) != 0 {
			t.Fatal("named check-only observation incomplete")
		}
	}
	if !strings.Contains(capture.OrgRun, "orgIdSha256="+orgHash) || !strings.Contains(capture.OrgRun, "sourceApi="+capture.SourceAPI) || !strings.Contains(capture.OrgRun, "transportApi="+capture.TransportAPI) {
		t.Fatal("D7 source/org pins differ")
	}
	for _, name := range []string{resultName, "org.tsv", "errors.json", "ORG-RUN.txt", "source-digests.json"} {
		assertAnnotatedMethodDigest(t, capture.RawSHA256[name])
	}
	if len(capture.Command.Command) == 0 || capture.Command.Command[0] != "sf" || !strings.Contains(strings.Join(capture.Command.Command, " "), "--api-version "+capture.TransportAPI) {
		t.Fatal("command transport pin differs")
	}
	manifestSum := sha256.Sum256([]byte(capture.SourceManifestRaw))
	wantPhase, wantKind, wantOmitted := "before-native-call", "original-precall", 0
	if anonymous {
		wantPhase, wantKind, wantOmitted = "owned-source-subset-of-before-native-call", "owned-source-subset-of-precall-manifest", 14
	}
	if capture.SourceManifest.Phase != wantPhase || capture.SourceManifestKind != wantKind || capture.OmittedNonSourceFiles != wantOmitted || hex.EncodeToString(manifestSum[:]) != capture.SourceManifestSHA256 || capture.RawSHA256["source-digests.json"] != capture.OriginalSourceManifestSHA256 || (!anonymous && capture.OriginalSourceManifestSHA256 != capture.SourceManifestSHA256) {
		t.Fatal("pre-call source manifest differs")
	}
	var manifest struct {
		Phase string
		Files map[string]string
	}
	if json.Unmarshal([]byte(capture.SourceManifestRaw), &manifest) != nil || manifest.Phase != capture.SourceManifest.Phase || !reflect.DeepEqual(manifest.Files, capture.SourceManifest.Files) || len(capture.Sources) != len(manifest.Files) {
		t.Fatal("frozen source manifest payload differs")
	}
	root := t.TempDir()
	for name, source := range capture.Sources {
		sum := sha256.Sum256([]byte(source.Source))
		if filepath.IsAbs(name) || strings.Contains(name, "..") || source.Source == "" || hex.EncodeToString(sum[:]) != source.SHA256 || manifest.Files[name] != source.SHA256 {
			t.Fatal("frozen source digest differs")
		}
		writeFile(t, filepath.Join(root, name), source.Source)
	}
	projectRoot := root
	for _, class := range []string{"AtlasAction", "GlyphAction"} {
		path := "force-app/main/default/classes/" + class + ".cls-meta.xml"
		sourcePath := "force-app/main/default/classes/" + class + ".cls"
		var metadata struct {
			API string `xml:"apiVersion"`
		}
		if xml.Unmarshal([]byte(capture.Sources[path].Source), &metadata) != nil || metadata.API != capture.HelperAPI {
			t.Fatal("helper metadata API differs")
		}
		if anonymous && (capture.Sources[sourcePath].SHA256 != helperDigests[sourcePath] || capture.Sources[path].SHA256 != helperDigests[path]) {
			t.Fatal("anonymous helper differs from installed source")
		}
	}
	projectPath := "sfdx-project.json"
	var projectMetadata struct {
		API string `json:"sourceApiVersion"`
	}
	if json.Unmarshal([]byte(capture.Sources[projectPath].Source), &projectMetadata) != nil || projectMetadata.API != capture.ProjectAPI {
		t.Fatal("project metadata API differs")
	}
	if !anonymous {
		var metadata struct {
			API string `xml:"apiVersion"`
		}
		if xml.Unmarshal([]byte(capture.Sources["force-app/main/default/classes/AnnotatedMethodObservation.cls-meta.xml"].Source), &metadata) != nil || metadata.API != capture.SourceAPI {
			t.Fatal("observer metadata API differs")
		}
	}
	index := loadTestIndex(t, projectRoot)
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
