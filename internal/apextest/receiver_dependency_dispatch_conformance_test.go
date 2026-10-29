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

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

const receiverDependencyFixture = "testdata/conformance/receiver_dependency_dispatch"

type receiverDependencySource struct {
	Anonymous                        string
	Classes, ClassAPIs, SourceSHA256 map[string]string
	Tests                            []string
}
type receiverDependencyCell struct {
	ID, API, Route, Outcome, Primary, RejectUnit, RejectClass string
	Source                                                    receiverDependencySource
	Native                                                    json.RawMessage
}
type receiverDependencyFile struct{ Path, SHA256 string }

func runReceiverDependencyConformance(t *testing.T, cluster string, cells int) {
	t.Helper()
	var data struct {
		Cluster     string
		CellCount   int
		APIVersions []string
		Cells       []receiverDependencyCell
	}
	raw, err := os.ReadFile(filepath.Join(receiverDependencyFixture, cluster+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data.Cluster != strings.ReplaceAll(cluster, "_", "-") || data.CellCount != cells || len(data.Cells) != cells {
		t.Fatal("incomplete admitted cluster")
	}
	var files []receiverDependencyFile
	raw, err = os.ReadFile(filepath.Join(receiverDependencyFixture, "fixture_manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &files); err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		receiverDependencyRead(t, file)
	}
	seen := map[string]bool{}
	for _, cell := range data.Cells {
		key := cell.ID + "/" + cell.API + "/" + cell.Route
		if seen[key] || !strings.HasPrefix(cell.ID, "C154") || (cell.Route != "anonymous" && cell.Route != "istest") {
			t.Fatalf("invalid native cell: %s", key)
		}
		seen[key] = true
		values := map[string]string{}
		if cell.Route == "anonymous" {
			values["anonymous"] = cell.Source.Anonymous
		}
		for name, text := range cell.Source.Classes {
			values["class:"+name] = text
		}
		if len(values) == 0 || len(values) != len(cell.Source.SourceSHA256) {
			t.Fatal("missing captured source hashes")
		}
		for name, text := range values {
			sum := sha256.Sum256([]byte(text))
			if text == "" || hex.EncodeToString(sum[:]) != cell.Source.SourceSHA256[name] {
				t.Fatalf("captured source changed: %s", name)
			}
		}
		var native struct{ Outcome string }
		if err := json.Unmarshal(cell.Native, &native); err != nil || native.Outcome != cell.Outcome {
			t.Fatal("missing independent native answer")
		}
		switch cell.Outcome {
		case "runtime", "uncaught_anonymous", "uncaught_test_failure":
		case "compile-rejected":
			if cell.Primary == "" || (cell.RejectUnit != "route" && cell.RejectUnit != "fixture") {
				t.Fatal("empty native rejection")
			}
			if cell.RejectClass != "" && cell.Source.Classes[cell.RejectClass] == "" {
				t.Fatal("native rejected class missing from source")
			}
		default:
			t.Fatal("non-answer admitted")
		}
	}
	for _, api := range data.APIVersions {
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "ReceiverDependency-"+cluster, api)
			index := receiverDependencyIndex(t, api, receiverDependencySource{}, files)
			org := orgFromIndex(index)
			org.APIVersion = api
			base := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			for _, cell := range data.Cells {
				if cell.API != api {
					continue
				}
				t.Run(cell.ID+"/"+cell.Route, func(t *testing.T) {
					kind := "exact"
					if cell.Outcome == "compile-rejected" {
						kind = "category"
					}
					route := cell.Route
					if route == "istest" {
						route = "isTest"
					}
					counts.TrackKind(t, route, kind, 1)
					receiverDependencyExecute(t, cell, files, base)
				})
			}
		})
	}
}

func receiverDependencyRead(t *testing.T, file receiverDependencyFile) string {
	t.Helper()
	if filepath.IsAbs(file.Path) || strings.Contains(file.Path, "..") {
		t.Fatal("invalid fixture path")
	}
	raw, err := os.ReadFile(filepath.Join(receiverDependencyFixture, file.Path))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	if hex.EncodeToString(sum[:]) != file.SHA256 {
		t.Fatalf("captured package fixture changed: %s", file.Path)
	}
	return string(raw)
}

// Load the package through project.ManagedPackageDependencies, as the production
// project loader does. Its uploaded class API metadata stays independent of the
// route API; subscriber class APIs include the captured 59/62/67/68 mixtures.
func receiverDependencyIndex(t *testing.T, api string, source receiverDependencySource, files []receiverDependencyFile) typesys.Index {
	t.Helper()
	root := t.TempDir()
	for _, file := range files {
		text := receiverDependencyRead(t, file)
		if file.Path == "fixture/subscriber/sfdx-project.json" {
			text = fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"","sourceApiVersion":%q}`, api)
		}
		writeFile(t, filepath.Join(root, strings.TrimPrefix(file.Path, "fixture/")), text)
	}
	for name, text := range source.Classes {
		classAPI := source.ClassAPIs[name]
		if classAPI == "" {
			classAPI = api
		}
		path := filepath.Join(root, "subscriber/force-app/main/default/classes", name+".cls")
		writeFile(t, path, text)
		writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+classAPI+"</apiVersion></ApexClass>")
	}
	index := loadTestIndex(t, filepath.Join(root, "subscriber"))
	if index.Project.Namespace != "" {
		t.Fatal("subscriber must be non-namespaced")
	}
	found := false
	for _, typ := range index.Types {
		if typ.Dependency && typ.Namespace == "gladens001" && typ.Name == "Builder" {
			found = true
		}
	}
	if !found {
		t.Fatal("managed collision package not loaded")
	}
	return index
}

func receiverDependencyExecute(t *testing.T, cell receiverDependencyCell, files []receiverDependencyFile, base *conformanceRunner) {
	t.Helper()
	// Subscriber fixture deployment precedes the route natively. A rejected
	// fixture must fail independently of any diagnostic in its observation unit.
	setupSource := cell.Source
	if cell.Route == "istest" && cell.RejectUnit == "fixture" {
		setupSource.Classes = make(map[string]string)
		for name, text := range cell.Source.Classes {
			isTest := false
			for _, test := range cell.Source.Tests {
				if name == test {
					isTest = true
				}
			}
			if !isTest {
				setupSource.Classes[name] = text
			}
		}
	}
	index := receiverDependencyIndex(t, cell.API, setupSource, files)
	items := append(append([]diagnostic.Diagnostic{}, index.Diagnostics...), sema.Analyze(index).Diagnostics...)
	if cell.RejectUnit == "fixture" {
		receiverDependencyCompile(t, cell, items, nil)
		return
	}
	if cell.Route == "anonymous" {
		// Setup was accepted natively even if the anonymous unit was rejected.
		accepted := cell
		accepted.Outcome = "runtime"
		receiverDependencyCompile(t, accepted, items, nil)
		analysis := sema.AnalyzeAnonymous(index, cell.Source.Anonymous, cell.API)
		program, compileErr := vm.CompileAnonymousWithOptions(cell.Source.Anonymous, vm.CompileOptions{APIVersion: cell.API})
		if !receiverDependencyCompile(t, cell, analysis.Diagnostics, compileErr) {
			return
		}
		runtime, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
		if err != nil {
			t.Fatal(err)
		}
		org := base.newOrg()
		runner := newConformanceRunner(t, index, conformanceRunnerOptions{Base: base, RequestRuntime: &runtime, Org: &org})
		_, execErr := runner.execute(program)
		receiverDependencyValue(t, cell, execErr)
		return
	}
	if !receiverDependencyCompile(t, cell, items, nil) {
		return
	}
	if len(cell.Source.Tests) != 1 {
		t.Fatal("one captured test class required")
	}
	cases := Discover(index, Options{SelectedClasses: cell.Source.Tests, SelectedMethod: "observe"})
	if len(cases) != 1 {
		t.Fatalf("captured test discovery: %d", len(cases))
	}
	methods, methodErrors := compileTestMethodsForIndex(&index, cases)
	programs, programErrors := compileTestInvokePrograms(cases)
	key := testCaseKey(cases[0])
	if methodErrors[key] != nil || programErrors[key] != nil {
		t.Fatalf("named compile: %v %v", methodErrors[key], programErrors[key])
	}
	org := base.newOrg()
	runner := newConformanceRunner(t, index, conformanceRunnerOptions{Base: base, LinkProject: true, Org: &org})
	machine := runner.newMachine()
	if err := machine.RegisterMethod(methods[key]); err != nil {
		t.Fatal(err)
	}
	machine.EnableTestContext()
	_, execErr := machine.ExecuteInClass(programs[key], cases[0].ClassName)
	receiverDependencyValue(t, cell, execErr)
}

func receiverDependencyCompile(t *testing.T, cell receiverDependencyCell, items []diagnostic.Diagnostic, compileErr error) bool {
	t.Helper()
	rejected := compileErr != nil
	if cell.RejectClass != "" {
		rejected = false
	}
	for _, item := range items {
		if item.Severity == diagnostic.Error && (cell.RejectClass == "" || strings.EqualFold(filepath.Base(item.File), cell.RejectClass+".cls")) {
			rejected = true
		}
	}
	if cell.Outcome == "compile-rejected" {
		if cell.Primary == "" || !rejected {
			t.Fatalf("native rejection <%s>; Glade diagnostics=%v compile=%v", cell.Primary, items, compileErr)
		}
		return false
	}
	if rejected {
		t.Fatalf("native compilation accepted; Glade diagnostics=%v compile=%v", items, compileErr)
	}
	return true
}

func receiverDependencyQualifiedType(name string) string {
	if !strings.Contains(name, ".") {
		return "System." + name
	}
	return name
}
func receiverDependencyValue(t *testing.T, cell receiverDependencyCell, execErr error) {
	t.Helper()
	var runtimeErr *vm.RuntimeError
	if !errors.As(execErr, &runtimeErr) {
		t.Fatalf("native observation missing: %v", execErr)
	}
	message := runtimeErr.ExceptionMessage()
	var native struct {
		Value                           map[string]any
		ExceptionType, ExceptionMessage string
		TestFailures                    []struct{ ExceptionType, Message string }
	}
	if err := json.Unmarshal(cell.Native, &native); err != nil {
		t.Fatal(err)
	}
	if cell.Outcome != "runtime" {
		typ, returned := native.ExceptionType, native.ExceptionMessage
		if cell.Outcome == "uncaught_test_failure" {
			if cell.Route != "istest" || len(native.TestFailures) != 1 {
				t.Fatal("missing native uncaught test failure")
			}
			typ, returned = native.TestFailures[0].ExceptionType, native.TestFailures[0].Message
		}
		// The native route decorates the raw message with its returned type.
		// Compare the raw ExceptionMessage exactly, retaining native text in data.
		prefix := typ + ": "
		if typ == "" || !strings.HasPrefix(returned, prefix) || receiverDependencyQualifiedType(runtimeErr.Type) != typ || message != strings.TrimPrefix(returned, prefix) {
			t.Fatalf("native <%s>; Glade <%s: %s>", returned, runtimeErr.Type, message)
		}
		return
	}
	if receiverDependencyQualifiedType(runtimeErr.Type) != "System.AssertException" {
		t.Fatalf("wrong observation transport: %v", execErr)
	}
	_, envelope, ok := strings.Cut(message, "C126_RESULT|"+cell.ID+"|")
	if !ok || strings.Count(message, "C126_RESULT|") != 1 {
		t.Fatalf("missing captured marker: %s", message)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(envelope), &got); err != nil {
		t.Fatal(err)
	}
	if value, ok := got["value"].(map[string]any); ok {
		if typ, ok := value["exceptionType"].(string); ok {
			value["exceptionType"] = receiverDependencyQualifiedType(typ)
		}
	}
	if !reflect.DeepEqual(got, native.Value) {
		t.Fatalf("native %s; Glade %s", cell.Native, envelope)
	}
}

func TestReceiverDependencyControlsConformance(t *testing.T) {
	runReceiverDependencyConformance(t, "controls", 32)
}
func TestReceiverDependencyReceiverConformance(t *testing.T) {
	runReceiverDependencyConformance(t, "receiver", 54)
}
func TestReceiverDependencyVisibilityConformance(t *testing.T) {
	runReceiverDependencyConformance(t, "visibility", 42)
}
func TestReceiverDependencyStaticConformance(t *testing.T) {
	runReceiverDependencyConformance(t, "static", 9)
}
func TestReceiverDependencyNoInheritanceConformance(t *testing.T) {
	runReceiverDependencyConformance(t, "no_inheritance", 24)
}
func TestReceiverDependencyPackageInheritanceConformance(t *testing.T) {
	runReceiverDependencyConformance(t, "package_inheritance", 48)
}
func TestReceiverDependencyPackageBoundaryConformance(t *testing.T) {
	runReceiverDependencyConformance(t, "package_boundary", 30)
}
func TestReceiverDependencyPackageLexicalConformance(t *testing.T) {
	runReceiverDependencyConformance(t, "package_lexical", 12)
}
func TestReceiverDependencyVersionsConformance(t *testing.T) {
	runReceiverDependencyConformance(t, "versions", 52)
}
