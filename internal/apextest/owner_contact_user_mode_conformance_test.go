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
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

type ownerContactNativeCapture struct {
	Context, SourceAPI, Route, ProjectAPI, ReplayProjectAPI, TransportAPI, NamespaceContext, TestClass string
	SourceManifestSHA256, SourceManifestRaw string; OrgRun string `json:"runMetadata"`
	SourceManifest                                                                                     struct {
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

func TestOwnerContactUserModeNativeConformance(t *testing.T) {
	var data struct {
		SourceAPIVersions, Routes, PendingCoverage []string
		NamespaceContext, Marker, OrgIDSHA256      string
		KnownRows                                  int
		ReplayCurrentUser                          struct{ FieldNullness map[string]bool }
		Captures                                   []ownerContactNativeCapture
		ExcludedAttempts                           []struct{ Context, Reason string }
	}
	raw, err := os.ReadFile("testdata/conformance/owner_contact_user_mode.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(data.SourceAPIVersions, []string{"61.0", "62.0", "67.0"}) || !reflect.DeepEqual(data.Routes, []string{"isTest", "anonymous"}) || data.NamespaceContext != "none" || data.Marker != "OWNER_CONTACT_OBSERVATION|" || data.KnownRows != 9 || len(data.Captures) != 6 || len(data.PendingCoverage) == 0 || len(data.ExcludedAttempts) != 2 {
		t.Fatal("native owner-contact scope differs")
	}
	if !reflect.DeepEqual(data.ReplayCurrentUser.FieldNullness, map[string]bool{"Id": false, "Name": false, "FirstName": false, "LastName": false, "Phone": true, "Email": false}) {
		t.Fatal("anonymous current-user nullness scope differs")
	}
	want := map[string]string{"named61-exact": "61.0", "named62": "62.0", "named67": "67.0", "anonymous61": "61.0", "anonymous62": "62.0", "anonymous67": "67.0"}
	count := 0
	for _, capture := range data.Captures {
		t.Run(capture.Context, func(t *testing.T) {
			if want[capture.Context] != capture.SourceAPI || capture.NamespaceContext != data.NamespaceContext || capture.ReplayProjectAPI != "62.0" {
				t.Fatal("native source or replay project scope differs")
			}
			delete(want, capture.Context)
			actual := replayOwnerContactNative(t, capture, data.Marker, data.OrgIDSHA256, data.ReplayCurrentUser.FieldNullness)
			if len(actual) != len(capture.Rows) {
				t.Fatal("replayed row count differs")
			}
			seen := map[string]bool{}
			for _, row := range capture.Rows {
				if seen[row.ID] {
					t.Fatal("duplicate native row")
				}
				seen[row.ID] = true
				count++
				if !reflect.DeepEqual(actual[row.ID], row.Native) {
					t.Errorf("row %s: native=%v; Glade=%v", row.ID, row.Native, actual[row.ID])
				}
			}
		})
	}
	if count != data.KnownRows || len(want) != 0 {
		t.Fatal("native row coverage differs")
	}
	for _, carry := range data.ExcludedAttempts {
		if carry.Context == "" || carry.Reason == "" {
			t.Fatal("excluded transport/setup attempt lacks a reason")
		}
	}
}

func replayOwnerContactNative(t *testing.T, capture ownerContactNativeCapture, marker, orgHash string, userNullness map[string]bool) map[string]map[string]any {
	t.Helper()
	anonymous := capture.Route == "anonymous"
	wantRows := 2
	resultName := "deploy.json"
	if anonymous {
		wantRows, resultName = 1, "run.json"
		e := capture.Execution
		if capture.ProjectAPI != "" || capture.TransportAPI != capture.SourceAPI || capture.TestClass != "" || e.Status != 0 || !e.Compiled || !e.Success || e.CompileProblem != "" || e.ExceptionMessage != "" || capture.Errors.ExitCode != 0 || !capture.Errors.Compiled || !capture.Errors.Success || capture.Errors.CompileProblem != "" || capture.Errors.ExceptionMessage != "" {
			t.Fatal("anonymous native execution scope differs")
		}
	} else {
		d := capture.Deployment
		if capture.Route != "isTest" || capture.ProjectAPI != "62.0" || capture.TransportAPI != "62.0" || capture.TestClass != "OwnerContactObservation" || d.ID == "" || !d.Done || !d.CheckOnly || d.Status != "Failed" || d.NumberComponentErrors != 0 || d.NumberTestErrors != 2 || d.NumberTestsCompleted != 0 || d.NumberTestsTotal != 2 || len(capture.Errors.ComponentFailures) != 0 || len(capture.Errors.TransportOrSetupErrors) != 0 {
			t.Fatal("named native check-only execution incomplete")
		}
	}
	if len(capture.Rows) != wantRows || !strings.Contains(capture.OrgRun, "orgIdSha256="+orgHash) || !strings.Contains(capture.OrgRun, "sourceApi="+capture.SourceAPI) || !strings.Contains(capture.OrgRun, "transportApi="+capture.TransportAPI) {
		t.Fatal("native row count or D7 pins differ")
	}
	for _, name := range []string{resultName, "org.tsv", "errors.json", "ORG-RUN.txt", "source-digests.json"} {
		sum, err := hex.DecodeString(capture.RawSHA256[name])
		if err != nil || len(sum) != sha256.Size {
			t.Fatalf("native raw digest missing: %s", name)
		}
	}
	if len(capture.Command.Command) == 0 || capture.Command.Command[0] != "sf" || !strings.Contains(strings.Join(capture.Command.Command, " "), "--api-version "+capture.TransportAPI) {
		t.Fatal("native command transport pin differs")
	}
	manifestSum := sha256.Sum256([]byte(capture.SourceManifestRaw))
	if capture.SourceManifest.Phase != "before-native-call" || hex.EncodeToString(manifestSum[:]) != capture.SourceManifestSHA256 || capture.RawSHA256["source-digests.json"] != capture.SourceManifestSHA256 {
		t.Fatal("pre-call source manifest differs")
	}
	var manifest struct {
		Phase string
		Files map[string]string
	}
	if json.Unmarshal([]byte(capture.SourceManifestRaw), &manifest) != nil || manifest.Phase != capture.SourceManifest.Phase || !reflect.DeepEqual(manifest.Files, capture.SourceManifest.Files) || len(capture.Sources) != len(manifest.Files) {
		t.Fatal("pre-call source manifest payload differs")
	}
	root := t.TempDir()
	for name, source := range capture.Sources {
		sum := sha256.Sum256([]byte(source.Source))
		if filepath.IsAbs(name) || strings.Contains(name, "..") || source.Source == "" || hex.EncodeToString(sum[:]) != source.SHA256 || manifest.Files[name] != source.SHA256 {
			t.Fatal("frozen source digest differs")
		}
		writeFile(t, filepath.Join(root, name), source.Source)
	}
	if anonymous {
		// Anonymous native execution has no metadata project. This empty project
		// only supplies the existing local conformance loader's schema boundary.
		writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"62.0","packageDirectories":[{"path":"force-app","default":true}]}`)
		if err := os.MkdirAll(filepath.Join(root, "force-app/main/default/classes"), 0o755); err != nil {
			t.Fatal(err)
		}
	} else {
		var projectMetadata struct {
			API string `json:"sourceApiVersion"`
		}
		var classMetadata struct {
			API string `xml:"apiVersion"`
		}
		if len(capture.Sources) != 3 || json.Unmarshal([]byte(capture.Sources["sfdx-project.json"].Source), &projectMetadata) != nil || projectMetadata.API != capture.ProjectAPI || xml.Unmarshal([]byte(capture.Sources["force-app/main/default/classes/OwnerContactObservation.cls-meta.xml"].Source), &classMetadata) != nil || classMetadata.API != capture.SourceAPI {
			t.Fatal("native class/project API metadata differs")
		}
	}
	index := loadTestIndex(t, root)
	analysis := sema.Analyze(index)
	if index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("native Apex compiled; diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
	}
	org := orgFromIndex(index)
	org.APIVersion = capture.ReplayProjectAPI
	// Native current-user PII is deliberately absent from the payload. Seed
	// arbitrary owned values with the observed nullness; equality still compares
	// each relationship projection with the actual same user record.
	users := org.Objects["User"]
	user := users.Records[storage.DefaultUserID]
	if user.ID == "" {
		t.Fatal("conformance current user absent")
	}
	for field, isNull := range userNullness {
		if field == "Id" {
			continue
		}
		if isNull {
			user.Fields[field] = storage.NullValue()
		} else {
			user.Fields[field] = storage.StringValue("Owned " + field)
		}
	}
	users.Records[user.ID] = user
	org.Objects["User"] = users
	decode := func(message string) map[string]any {
		t.Helper()
		_, payload, ok := strings.Cut(message, marker)
		if !ok {
			t.Fatal("native observer marker absent")
		}
		var value map[string]any
		if err := json.Unmarshal([]byte(payload), &value); err != nil {
			t.Fatal(err)
		}
		return value
	}
	actual := map[string]map[string]any{}
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
		for _, message := range result.Debug {
			if strings.HasPrefix(message, marker) {
				if actual["userOwned"] != nil {
					t.Fatal("repeated anonymous observation")
				}
				actual["userOwned"] = decode(message)
			}
		}
		return actual
	}
	runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
	cases := Discover(index, Options{SelectedClasses: []string{capture.TestClass}})
	if len(cases) != wantRows {
		t.Fatal("native observer discovery differs")
	}
	methods, methodErrors := compileTestMethodsForIndex(&index, cases)
	programs, programErrors := compileTestInvokePrograms(cases)
	for _, row := range capture.Rows {
		if !reflect.DeepEqual(decode(row.NativeFailure.Message), row.Native) {
			t.Fatal("native payload differs from assertion envelope")
		}
		key := ""
		for _, c := range cases {
			if c.MethodName == row.ID {
				key = testCaseKey(c)
			}
		}
		if key == "" || methodErrors[key] != nil || programErrors[key] != nil {
			t.Fatal("native observer compilation differs")
		}
		machine := runner.newMachine()
		if err := machine.RegisterMethod(methods[key]); err != nil {
			t.Fatal(err)
		}
		machine.EnableTestContext()
		_, execErr := machine.ExecuteInClass(programs[key], capture.TestClass)
		var runtimeErr *vm.RuntimeError
		if !errors.As(execErr, &runtimeErr) || genericInterfaceCastExceptionType(runtimeErr.Type) != "System.AssertException" {
			t.Fatalf("native observer assertion absent: %v", execErr)
		}
		actual[row.ID] = decode(runtimeErr.Message)
	}
	return actual
}
