package apextest

import (
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

type valueCaptureNativeSource struct {
	HelperName     string `json:"helperName"`
	HelperSource   string `json:"helperSource"`
	HelperMetadata string `json:"helperMetadata"`
	TestName       string `json:"testName"`
	TestSource     string `json:"testSource"`
	TestMetadata   string `json:"testMetadata"`
	Anonymous      string `json:"anonymous"`
}

type valueCaptureNativeCell struct {
	API     string         `json:"api"`
	Context string         `json:"context"`
	Regex   map[string]any `json:"regex"`
	Map     map[string]any `json:"map"`
}

type valueCaptureNativeFixture struct {
	APIs         []string                            `json:"apis"`
	SourcesByAPI map[string]valueCaptureNativeSource `json:"sourcesByAPI"`
	Cells        []valueCaptureNativeCell            `json:"cells"`
	Carries      []struct {
		Family string `json:"family"`
		Field  string `json:"field"`
		Reason string `json:"reason"`
	} `json:"carries"`
}

func TestMixedCaptureOrderNativeConformance(t *testing.T) {
	runValueCaptureNativeConformance(t, "regex", "regexObserved", "CAPTURE_REGEX|")
}

func TestMapStringValueNativeConformance(t *testing.T) {
	runValueCaptureNativeConformance(t, "map", "mapObserved", "CAPTURE_MAP|")
}

func TestMapNumericValueNativeConformance(t *testing.T) {
	data := loadValueCaptureNativeFixtureFile(t, "testdata/conformance/numeric_map_value_native.json")
	for _, cell := range data.Cells {
		if len(cell.Map) != 20 || len(cell.Regex) != 0 {
			t.Fatalf("numeric native observation shape: %s/%s", cell.API, cell.Context)
		}
	}
	runValueCaptureNativeFixture(t, data, "map", "observed", "NUMERIC_MAP|")
}

func loadValueCaptureNativeFixture(t *testing.T) valueCaptureNativeFixture {
	t.Helper()
	data := loadValueCaptureNativeFixtureFile(t, "testdata/conformance/value_capture_native.json")
	for _, cell := range data.Cells {
		if len(cell.Regex) == 0 {
			t.Fatalf("missing regex snapshot: %s/%s", cell.API, cell.Context)
		}
	}
	return data
}

func loadValueCaptureNativeFixtureFile(t *testing.T, path string) valueCaptureNativeFixture {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var data valueCaptureNativeFixture
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIs, ",") != "62.0,67.0,68.0" || len(data.SourcesByAPI) != 3 || len(data.Cells) != 6 {
		t.Fatal("incomplete native API/context matrix")
	}
	if len(data.Carries) != 0 {
		t.Fatal("fully asserted native fixture contains unresolved carries")
	}
	seen := make(map[string]bool)
	for _, cell := range data.Cells {
		key := cell.API + "/" + cell.Context
		if seen[key] || (cell.API != "62.0" && cell.API != "67.0" && cell.API != "68.0") || (cell.Context != "anonymous" && cell.Context != "istest") || len(cell.Map) == 0 {
			t.Fatalf("invalid or repeated native cell %s", key)
		}
		seen[key] = true
	}
	for _, api := range data.APIs {
		source, present := data.SourcesByAPI[api]
		if !present || source.HelperName == "" || source.HelperSource == "" || source.TestName == "" || source.TestSource == "" || source.Anonymous == "" || !strings.Contains(source.HelperMetadata, "<apiVersion>"+api+"</apiVersion>") || !strings.Contains(source.TestMetadata, "<apiVersion>"+api+"</apiVersion>") {
			t.Fatalf("missing captured source or API metadata: %s", api)
		}
		for _, context := range []string{"anonymous", "istest"} {
			if !seen[api+"/"+context] {
				t.Fatalf("missing native answer: %s/%s", api, context)
			}
		}
	}
	return data
}

func runValueCaptureNativeConformance(t *testing.T, family, testMethod, marker string) {
	t.Helper()
	data := loadValueCaptureNativeFixture(t)
	runValueCaptureNativeFixture(t, data, family, testMethod, marker)
}

func runValueCaptureNativeFixture(t *testing.T, data valueCaptureNativeFixture, family, testMethod, marker string) {
	t.Helper()
	for _, cell := range data.Cells {
		t.Run(cell.API+"/"+cell.Context, func(t *testing.T) {
			source := data.SourcesByAPI[cell.API]
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, cell.API))
			for _, class := range []struct{ name, body, metadata string }{
				{source.HelperName, source.HelperSource, source.HelperMetadata},
				{source.TestName, source.TestSource, source.TestMetadata},
			} {
				path := filepath.Join(root, "force-app/main/default/classes", class.name+".cls")
				writeFile(t, path, class.body)
				writeFile(t, path+"-meta.xml", class.metadata)
			}
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native accepts captured source: parser=%v semantics=%v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = cell.API
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			var payload string
			if cell.Context == "anonymous" {
				analysis := sema.AnalyzeAnonymous(index, source.Anonymous, cell.API)
				program, err := vm.CompileAnonymousWithOptions(source.Anonymous, vm.CompileOptions{APIVersion: cell.API})
				if analysis.HasErrors() || err != nil {
					t.Fatalf("native anonymous source: semantics=%v compile=%v", analysis.Diagnostics, err)
				}
				result, err := runner.execute(program)
				if err != nil {
					t.Fatal(err)
				}
				for _, line := range result.Debug {
					if at := strings.Index(line, marker); at >= 0 {
						if payload != "" {
							t.Fatal("duplicate captured debug transport")
						}
						payload = line[at+len(marker):]
					}
				}
			} else {
				cases := Discover(index, Options{SelectedClasses: []string{source.TestName}, SelectedMethod: testMethod})
				if len(cases) != 1 {
					t.Fatalf("captured test discovery: %d", len(cases))
				}
				methods, methodErrors := compileTestMethods(cases)
				programs, programErrors := compileTestInvokePrograms(cases)
				key := testCaseKey(cases[0])
				if methodErrors[key] != nil || programErrors[key] != nil {
					t.Fatalf("captured test compile: %v %v", methodErrors[key], programErrors[key])
				}
				machine := runner.newMachine()
				if err := machine.RegisterMethod(methods[key]); err != nil {
					t.Fatal(err)
				}
				machine.EnableTestContext()
				_, err := machine.ExecuteInClass(programs[key], cases[0].ClassName)
				var runtimeErr *vm.RuntimeError
				if !errors.As(err, &runtimeErr) || runtimeErr.Type != "System.AssertException" {
					t.Fatalf("missing captured assertion transport: %v", err)
				}
				message := runtimeErr.ExceptionMessage()
				if at := strings.Index(message, marker); at >= 0 {
					payload = message[at+len(marker):]
				}
			}
			var got map[string]any
			if err := json.Unmarshal([]byte(payload), &got); err != nil || got == nil {
				t.Fatalf("missing or invalid captured payload %q: %v", payload, err)
			}
			want := cell.Regex
			if family == "map" {
				want = cell.Map
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("native %s=%#v; local=%#v", family, want, got)
			}
		})
	}
}
