package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

type sharingReasonLabelContext struct {
	SourceAPI    string            `json:"sourceApi"`
	TransportAPI string            `json:"transportApi"`
	Route        string            `json:"route"`
	Values       []json.RawMessage `json:"values"`
}

func TestSharingReasonLabelConformance(t *testing.T) {
	const fixture = "testdata/conformance/sharing_reason_label"
	var data struct {
		Files []struct {
			Path   string `json:"path"`
			SHA256 string `json:"sha256"`
		} `json:"files"`
		Contexts []sharingReasonLabelContext `json:"contexts"`
	}
	raw, err := os.ReadFile(filepath.Join(fixture, "observations.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Files) != 4 || len(data.Contexts) != 8 {
		t.Fatal("incomplete sharing reason label observations")
	}
	expectedFiles := map[string]bool{
		"project/force-app/main/default/classes/SharingReasonLabelObservation.cls":                                      true,
		"project/force-app/main/default/classes/SharingReasonLabelObservationTest.cls":                                  true,
		"project/force-app/main/default/objects/ArchiveItem__c/ArchiveItem__c.object-meta.xml":                          true,
		"project/force-app/main/default/objects/ArchiveItem__c/sharingReasons/Authored_Audit__c.sharingReason-meta.xml": true,
	}
	files := map[string]string{}
	for _, file := range data.Files {
		if !expectedFiles[file.Path] || files[file.Path] != "" {
			t.Fatalf("invalid or repeated sharing reason label source: %s", file.Path)
		}
		body, err := os.ReadFile(filepath.Join(fixture, filepath.FromSlash(file.Path)))
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(body)
		if len(body) == 0 || hex.EncodeToString(digest[:]) != file.SHA256 {
			t.Fatalf("captured sharing reason label source changed: %s", file.Path)
		}
		files[file.Path] = string(body)
	}
	expectedContexts := map[string]bool{}
	for _, api := range []string{"62.0", "66.0", "67.0", "68.0"} {
		for _, route := range []string{"anonymous", "istest"} {
			expectedContexts[api+"/"+route] = true
		}
	}
	seen := map[string]bool{}
	cells := 0
	for _, context := range data.Contexts {
		key := context.SourceAPI + "/" + context.Route
		if !expectedContexts[key] || seen[key] || context.TransportAPI != context.SourceAPI || len(context.Values) != 6 {
			t.Fatalf("invalid or repeated sharing reason label context: %+v", context)
		}
		seen[key] = true
		for i, value := range context.Values {
			var native any
			if err := json.Unmarshal(value, &native); err != nil {
				t.Fatalf("invalid sharing reason label value: %s", value)
			}
			if i == 2 {
				if _, ok := native.(bool); !ok {
					t.Fatalf("native sharing reason active value is not Boolean: %s", value)
				}
			} else if _, ok := native.(string); !ok {
				t.Fatalf("native sharing reason text value is not String: %s", value)
			}
		}
		cells += len(context.Values)
	}
	if cells != 48 {
		t.Fatalf("sharing reason label cells=%d, want 48", cells)
	}
	for _, context := range data.Contexts {
		t.Run(context.SourceAPI+"/"+context.Route, func(t *testing.T) {
			sharingReasonLabelReplay(t, context, files)
		})
	}
}

func sharingReasonLabelReplay(t *testing.T, context sharingReasonLabelContext, files map[string]string) {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+context.SourceAPI+`","packageDirectories":[{"path":"force-app","default":true}]}`)
	for path, body := range files {
		destination := filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(path, "project/")))
		writeFile(t, destination, body)
		if strings.HasSuffix(path, ".cls") {
			writeFile(t, destination+"-meta.xml", "<ApexClass><apiVersion>"+context.SourceAPI+"</apiVersion></ApexClass>")
		}
	}
	index := loadTestIndex(t, root)
	pinned := 0
	for _, typ := range index.Types {
		if typ.Name == "SharingReasonLabelObservation" || typ.Name == "SharingReasonLabelObservationTest" {
			if typ.EffectiveAPIVersion != context.SourceAPI {
				t.Fatalf("changed sharing reason label source pin: %s=%s", typ.Name, typ.EffectiveAPIVersion)
			}
			pinned++
		}
	}
	if pinned != 2 {
		t.Fatalf("sharing reason label helper count=%d, want 2", pinned)
	}
	analysis := sema.Analyze(index)
	if index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("native sharing reason label source accepted; local diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
	}
	org := orgFromIndex(index)
	org.APIVersion = context.TransportAPI
	runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
	const marker = "REASON_LABEL:"
	var execErr error
	if context.Route == "anonymous" {
		body := "System.assert(false,'" + marker + "'+JSON.serialize(SharingReasonLabelObservation.observe()));"
		checked := sema.AnalyzeAnonymous(index, body, context.SourceAPI)
		program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: context.SourceAPI})
		if checked.HasErrors() || err != nil {
			t.Fatalf("native anonymous sharing reason label accepted; local diagnostics=%v compile=%v", checked.Diagnostics, err)
		}
		_, execErr = runner.execute(program)
	} else {
		cases := Discover(index, Options{SelectedClasses: []string{"SharingReasonLabelObservationTest"}, SelectedMethod: "capture"})
		if len(cases) != 1 {
			t.Fatalf("named sharing reason label discovery: %d", len(cases))
		}
		methods, methodErrors := compileTestMethodsForIndex(&index, cases)
		programs, programErrors := compileTestInvokePrograms(cases)
		key := testCaseKey(cases[0])
		if methodErrors[key] != nil || programErrors[key] != nil {
			t.Fatalf("named sharing reason label compilation: %v %v", methodErrors[key], programErrors[key])
		}
		machine := runner.newMachine()
		if err := machine.RegisterMethod(methods[key]); err != nil {
			t.Fatal(err)
		}
		machine.EnableTestContext()
		_, execErr = machine.ExecuteInClass(programs[key], cases[0].ClassName)
	}
	var runtimeErr *vm.RuntimeError
	if !errors.As(execErr, &runtimeErr) || runtimeErr.Type != "System.AssertException" {
		t.Fatalf("missing sharing reason label observation: %v", execErr)
	}
	message := runtimeErr.ExceptionMessage()
	_, payload, found := strings.Cut(message, marker)
	if !found || strings.Count(message, marker) != 1 {
		t.Fatalf("invalid sharing reason label marker: %v", execErr)
	}
	var observed []json.RawMessage
	if err := json.Unmarshal([]byte(payload), &observed); err != nil || len(observed) != 6 {
		t.Fatalf("incomplete sharing reason label payload: %s", payload)
	}
	counts := newConformanceCountLog(t, "SharingReasonLabel", context.SourceAPI)
	route := context.Route
	if route == "istest" {
		route = "isTest"
	}
	for i, id := range []string{"customValue", "customLabel", "customActive", "customToken", "manualValue", "manualLabel"} {
		counts.Run(t, route, id, id, func(t *testing.T) {
			var native, local any
			if err := json.Unmarshal(context.Values[i], &native); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(observed[i], &local); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(native, local) {
				t.Fatalf("native sharing reason label=%s; local=%s", context.Values[i], observed[i])
			}
		})
	}
}
