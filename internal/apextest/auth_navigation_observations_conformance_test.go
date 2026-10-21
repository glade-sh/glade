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

// Replay callable observations with their captured helper and test source
// versions. A transport API does not replace the source version of a class.
// Ordinary scratch calls supply no evidence for hosted Site or Network routes.
func TestAuthNavigationObservationsConformance(t *testing.T) {
	var data struct {
		Classes           map[string]string `json:"classes"`
		ClassAPIs         map[string]string `json:"classApis"`
		ExpectedCellCount int               `json:"expectedCellCount"`
		Cells             []struct {
			ID           string          `json:"id"`
			SourceAPI    string          `json:"sourceApi"`
			CallerAPI    string          `json:"callerApi"`
			TransportAPI string          `json:"transportApi"`
			Route        string          `json:"route"`
			HelperClass  string          `json:"helperClass"`
			TestClass    string          `json:"testClass"`
			Status       string          `json:"status"`
			Native       json.RawMessage `json:"native"`
			SourceSHA256 string          `json:"sourceSha256"`
		} `json:"cells"`
		Carried []struct {
			ID            string          `json:"id"`
			Reason        string          `json:"reason"`
			NativeControl json.RawMessage `json:"nativeControl"`
		} `json:"carried"`
	}
	raw, err := os.ReadFile("testdata/conformance/auth_navigation_observations.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data.ExpectedCellCount != 64 || len(data.Cells) != data.ExpectedCellCount {
		t.Fatal("incomplete callable observation matrix")
	}
	expected := map[string]bool{}
	for _, id := range []string{"AN01", "AN02", "AN03", "AN04", "AN05", "AN06", "AN07", "AN08"} {
		original := "41.0"
		if id == "AN01" || id == "AN02" {
			original = "38.0"
		}
		for _, sourceAPI := range []string{original, "62.0", "67.0", "68.0"} {
			for _, route := range []string{"anonymous", "istest"} {
				expected[id+"/"+sourceAPI+"/"+route] = true
			}
		}
	}
	seen := map[string]bool{}
	for _, cell := range data.Cells {
		key := cell.ID + "/" + cell.SourceAPI + "/" + cell.Route
		if !expected[key] || seen[key] || cell.Status != "runtime" {
			t.Fatalf("invalid or repeated callable cell: %s", key)
		}
		seen[key] = true
		callerAPI, transportAPI := cell.SourceAPI, cell.SourceAPI
		if cell.SourceAPI == "38.0" || cell.SourceAPI == "41.0" {
			transportAPI = "62.0"
			if cell.Route == "anonymous" {
				callerAPI = "62.0"
			}
		}
		if cell.CallerAPI != callerAPI || cell.TransportAPI != transportAPI {
			t.Fatalf("changed caller/transport source pins: %s", key)
		}
		if cell.HelperClass == "" || cell.TestClass == "" || cell.HelperClass == cell.TestClass {
			t.Fatalf("missing callable class names: %s", key)
		}
		for _, name := range []string{cell.HelperClass, cell.TestClass} {
			if data.Classes[name] == "" || data.ClassAPIs[name] != cell.SourceAPI {
				t.Fatalf("missing or changed source pin: %s/%s", key, name)
			}
		}
		sum := sha256.Sum256([]byte(data.Classes[cell.HelperClass]))
		if hex.EncodeToString(sum[:]) != cell.SourceSHA256 {
			t.Fatalf("changed captured helper source: %s", key)
		}
		var native map[string]any
		if err := json.Unmarshal(cell.Native, &native); err != nil || len(native) == 0 || native["isTest"] != (cell.Route == "istest") {
			t.Fatalf("missing native context observation: %s", key)
		}
		if _, ok := native["siteId"]; !ok {
			if _, fault := native["siteContextException"]; !fault {
				t.Fatalf("missing Site context: %s", key)
			}
		}
		if _, ok := native["networkId"]; !ok {
			if _, fault := native["networkContextException"]; !fault {
				t.Fatalf("missing Network context: %s", key)
			}
		}
	}
	counts := map[string]*conformanceCounts{}
	for _, cell := range data.Cells {
		if counts[cell.SourceAPI] == nil {
			counts[cell.SourceAPI] = newConformanceCountLog(t, "AuthNavigationObservations", cell.SourceAPI)
		}
		route := cell.Route
		if route == "istest" {
			route = "isTest"
		}
		counts[cell.SourceAPI].Run(t, route, cell.ID, cell.ID+"/"+cell.SourceAPI+"/"+cell.Route, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+cell.TransportAPI+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			for _, name := range []string{cell.HelperClass, cell.TestClass} {
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
			org.APIVersion = cell.TransportAPI
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			marker := "AUTH_OBS|" + cell.ID + "|" + cell.SourceAPI + "|"
			var execErr error
			if cell.Route == "anonymous" {
				body := "System.assert(false,'" + marker + "'+" + cell.HelperClass + ".observe());"
				checked := sema.AnalyzeAnonymous(index, body, cell.CallerAPI)
				program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: cell.CallerAPI})
				if checked.HasErrors() || err != nil {
					t.Fatalf("native anonymous source accepted; local diagnostics=%v compile=%v", checked.Diagnostics, err)
				}
				_, execErr = runner.execute(program)
			} else {
				cases := Discover(index, Options{SelectedClasses: []string{cell.TestClass}, SelectedMethod: "observe"})
				if len(cases) != 1 {
					t.Fatalf("named callable observation discovery: %d", len(cases))
				}
				methods, methodErrors := compileTestMethodsForIndex(&index, cases)
				programs, programErrors := compileTestInvokePrograms(cases)
				key := testCaseKey(cases[0])
				if methodErrors[key] != nil || programErrors[key] != nil {
					t.Fatalf("named callable compilation: %v %v", methodErrors[key], programErrors[key])
				}
				machine := runner.newMachine()
				if err := machine.RegisterMethod(methods[key]); err != nil {
					t.Fatal(err)
				}
				machine.EnableTestContext()
				_, execErr = machine.ExecuteInClass(programs[key], cases[0].ClassName)
			}
			comparatorContractValue(t, marker, genericInterfaceCastExpected{Outcome: "runtime", Value: string(cell.Native)}, execErr)
		})
	}
	expectedCarries := map[string]bool{
		"hosted_site_authentication":    true,
		"hosted_network_authentication": true,
		"other_login_argument_shapes":   true,
		"other_network_overload_shapes": true,
	}
	if len(data.Carried) != len(expectedCarries) {
		t.Fatal("incomplete callable carry accounting")
	}
	seenCarries := map[string]bool{}
	for _, carry := range data.Carried {
		if !expectedCarries[carry.ID] || seenCarries[carry.ID] || strings.TrimSpace(carry.Reason) == "" || !json.Valid(carry.NativeControl) || string(carry.NativeControl) == "null" {
			t.Fatal("hosted carry requires its scope, reason and native control")
		}
		seenCarries[carry.ID] = true
		t.Logf("CARRIED %s: %s; native control=%s", carry.ID, carry.Reason, carry.NativeControl)
	}
}
