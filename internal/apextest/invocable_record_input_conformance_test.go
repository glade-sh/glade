package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// Replay captured record, wrapper and String action inputs using the deployed
// class source versions independently of the request transport version.
func TestInvocableRecordInputConformance(t *testing.T) {
	var data struct {
		Classes      map[string]string `json:"classes"`
		ClassAPIs    map[string]string `json:"classApis"`
		Observations []struct {
			SourceAPI    string            `json:"sourceApi"`
			CallerAPI    string            `json:"callerApi"`
			TransportAPI string            `json:"transportApi"`
			Route        string            `json:"route"`
			Native       json.RawMessage   `json:"native"`
			SourceSHA256 map[string]string `json:"sourceSha256"`
		} `json:"observations"`
		Carried []struct {
			ID            string          `json:"id"`
			Reason        string          `json:"reason"`
			NativeControl json.RawMessage `json:"nativeControl"`
		} `json:"carried"`
	}
	raw, err := os.ReadFile("testdata/conformance/invocable_record_input.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Observations) != 8 {
		t.Fatal("incomplete native record-input observations")
	}
	expected := map[string]bool{}
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, route := range []string{"anonymous", "istest"} {
			expected[api+"/"+api+"/"+api+"/"+route] = true
		}
	}
	expected["67.0/62.0/62.0/anonymous"] = true
	expected["67.0/67.0/62.0/istest"] = true
	seen := map[string]bool{}
	for _, observation := range data.Observations {
		key := observation.SourceAPI + "/" + observation.CallerAPI + "/" + observation.TransportAPI + "/" + observation.Route
		if !expected[key] || seen[key] {
			t.Fatalf("invalid or repeated native record-input observation: %s", key)
		}
		seen[key] = true
		suffix := strings.TrimSuffix(observation.SourceAPI, ".0")
		names := []string{"InputRecord" + suffix, "InputContact" + suffix, "InputWrapper" + suffix, "InputText" + suffix, "RecordInputObserve" + suffix, "RecordInputObserve" + suffix + "Test"}
		if len(observation.SourceSHA256) != len(names) {
			t.Fatalf("incomplete native source digests: %s", key)
		}
		for _, name := range names {
			source := data.Classes[name]
			digest := sha256.Sum256([]byte(source))
			if source == "" || data.ClassAPIs[name] != observation.SourceAPI || hex.EncodeToString(digest[:]) != observation.SourceSHA256[name] {
				t.Fatalf("changed native source or class API pin: %s/%s", key, name)
			}
		}
		var native map[string]json.RawMessage
		if err := json.Unmarshal(observation.Native, &native); err != nil || len(native) != 6 {
			t.Fatalf("incomplete native payload: %s", key)
		}
		var isTest bool
		if err := json.Unmarshal(native["isTest"], &isTest); err != nil || isTest != (observation.Route == "istest") {
			t.Fatalf("changed native execution context: %s", key)
		}
		for _, id := range []string{"RI01", "RI02", "RI03", "RI04", "RI05"} {
			var outcome map[string]any
			if err := json.Unmarshal(native[id], &outcome); err != nil || len(outcome) == 0 {
				t.Fatalf("missing native action outcome: %s/%s", key, id)
			}
		}
	}
	counts := map[string]*conformanceCounts{}
	for _, observation := range data.Observations {
		frame := observation.SourceAPI + "/" + observation.TransportAPI
		if counts[frame] == nil {
			counts[frame] = newConformanceCountLog(t, "InvocableRecordInput", observation.SourceAPI+" transport "+observation.TransportAPI)
		}
		key := observation.SourceAPI + "/" + observation.CallerAPI + "/" + observation.TransportAPI + "/" + observation.Route
		t.Run(key, func(t *testing.T) {
			route := observation.Route
			if route == "istest" {
				route = "isTest"
			}
			counts[frame].TrackKind(t, route, "exact", 5)
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+observation.TransportAPI+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			for name := range observation.SourceSHA256 {
				path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
				writeFile(t, path, data.Classes[name])
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+data.ClassAPIs[name]+"</apiVersion></ApexClass>")
			}
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native source accepted; local diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = observation.TransportAPI
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			helper := "RecordInputObserve" + strings.TrimSuffix(observation.SourceAPI, ".0")
			marker := "RECORD_INPUT|" + observation.SourceAPI + "|"
			var execErr error
			if observation.Route == "anonymous" {
				body := "System.assert(false,'" + marker + "'+" + helper + ".observe());"
				checked := sema.AnalyzeAnonymous(index, body, observation.CallerAPI)
				program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: observation.CallerAPI})
				if checked.HasErrors() || err != nil {
					t.Fatalf("native anonymous source accepted; local diagnostics=%v compile=%v", checked.Diagnostics, err)
				}
				_, execErr = runner.execute(program)
			} else {
				cases := Discover(index, Options{SelectedClasses: []string{helper + "Test"}, SelectedMethod: "observe"})
				if len(cases) != 1 {
					t.Fatalf("native test discovery: %d", len(cases))
				}
				methods, methodErrors := compileTestMethodsForIndex(&index, cases)
				programs, programErrors := compileTestInvokePrograms(cases)
				methodKey := testCaseKey(cases[0])
				if methodErrors[methodKey] != nil || programErrors[methodKey] != nil {
					t.Fatalf("native test compilation: %v %v", methodErrors[methodKey], programErrors[methodKey])
				}
				machine := runner.newMachine()
				if err := machine.RegisterMethod(methods[methodKey]); err != nil {
					t.Fatal(err)
				}
				machine.EnableTestContext()
				_, execErr = machine.ExecuteInClass(programs[methodKey], cases[0].ClassName)
			}
			comparatorContractValue(t, marker, genericInterfaceCastExpected{Outcome: "runtime", Value: string(observation.Native)}, execErr)
		})
	}
	expectedCarries := map[string]bool{"generic_sobject_input": true, "other_record_input_values": true}
	if len(data.Carried) != len(expectedCarries) {
		t.Fatal("incomplete record-input carry accounting")
	}
	seenCarries := map[string]bool{}
	for _, carry := range data.Carried {
		if !expectedCarries[carry.ID] || seenCarries[carry.ID] || strings.TrimSpace(carry.Reason) == "" || !json.Valid(carry.NativeControl) || string(carry.NativeControl) == "null" {
			t.Fatal("uncaptured record-input scope needs its reason and native control")
		}
		seenCarries[carry.ID] = true
		t.Logf("CARRIED %s: %s; native control=%s", carry.ID, carry.Reason, carry.NativeControl)
	}
}
