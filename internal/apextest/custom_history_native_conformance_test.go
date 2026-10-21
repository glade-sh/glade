package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

type customHistoryNativeSourceFile struct {
	Path      string `json:"path"`
	Role      string `json:"role"`
	ClassName string `json:"className"`
	SHA256    string `json:"sha256"`
	Content   string `json:"content"`
}

type customHistoryNativeCell struct {
	ID              string         `json:"id"`
	Family          string         `json:"family"`
	API             string         `json:"api"`
	ProjectManifest string         `json:"projectManifest"`
	Context         string         `json:"context"`
	Classes         []string       `json:"classes"`
	Anonymous       string         `json:"anonymousSource"`
	TestClass       string         `json:"testClass"`
	TestMethod      string         `json:"testMethod"`
	Marker          string         `json:"marker"`
	Expected        map[string]any `json:"expected"`
	Capture         string         `json:"capture"`
}

type customHistoryNativeCompileRejection struct {
	ID              string   `json:"id"`
	API             string   `json:"api"`
	ProjectManifest string   `json:"projectManifest"`
	Classes         []string `json:"classes"`
	ExpectedErrors  []string `json:"expectedErrors"`
	Capture         string   `json:"capture"`
}

type customHistoryNativeProjectManifest struct {
	SHA256         string `json:"sha256"`
	Content        string `json:"content"`
	CaptureProject string `json:"captureProject"`
}

type customHistoryNativeCarry struct {
	ID               string `json:"id"`
	API              string `json:"api"`
	Context          string `json:"context"`
	Outcome          string `json:"outcome"`
	RawNativeMessage string `json:"rawNativeMessage"`
	Capture          string `json:"capture"`
	Reason           string `json:"reason"`
}

type customHistoryNativeFixture struct {
	SourceFamily       string                                        `json:"sourceFamily"`
	NativeOrgSHA256    string                                        `json:"nativeOrgSha256"`
	NormalizedCommands map[string]string                             `json:"normalizedCommands"`
	ProjectManifests   map[string]customHistoryNativeProjectManifest `json:"projectManifests"`
	SourceFiles        []customHistoryNativeSourceFile               `json:"sourceFiles"`
	RuntimeCells       []customHistoryNativeCell                     `json:"runtimeCells"`
	CompileRejections  []customHistoryNativeCompileRejection         `json:"compileRejections"`
	Carries            []customHistoryNativeCarry                    `json:"carries"`
}

func TestCustomHistoryNativeConformance(t *testing.T) {
	data := loadCustomHistoryNativeFixture(t)
	for _, cell := range data.RuntimeCells {
		cell := cell
		t.Run(cell.ID, func(t *testing.T) {
			customHistoryNativeRuntimeCell(t, data, cell)
		})
	}
	for _, rejected := range data.CompileRejections {
		rejected := rejected
		t.Run(rejected.ID, func(t *testing.T) {
			assertCustomHistoryNativeCompileRejection(t, data, rejected)
		})
	}
	// Carries are deliberately recorded as native observations with a concrete
	// reason; they do not become skips or attempted local replays.
	t.Logf("retained %d setup-limited native carries", len(data.Carries))
}

func loadCustomHistoryNativeFixture(t *testing.T) customHistoryNativeFixture {
	t.Helper()
	raw, err := os.ReadFile("testdata/conformance/custom_history_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var data customHistoryNativeFixture
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data.SourceFamily != "custom_history_native" || !customHistoryIsSHA256(data.NativeOrgSHA256) {
		t.Fatal("missing custom-history capture provenance")
	}
	for _, key := range []string{"deployProject", "deployClasses", "deploySchema", "runTest", "runAnonymous", "checkOnlyDeploy"} {
		if strings.TrimSpace(data.NormalizedCommands[key]) == "" {
			t.Fatalf("missing normalized native command %s", key)
		}
	}
	if len(data.RuntimeCells) != 30 || len(data.CompileRejections) != 5 || len(data.Carries) != 6 {
		t.Fatalf("incomplete native evidence: runtime=%d compile=%d carries=%d", len(data.RuntimeCells), len(data.CompileRejections), len(data.Carries))
	}
	if len(data.ProjectManifests) != 6 {
		t.Fatalf("incomplete captured source-project manifests: %d", len(data.ProjectManifests))
	}
	for name, manifest := range data.ProjectManifests {
		digest := sha256.Sum256([]byte(manifest.Content))
		if name == "" || manifest.CaptureProject == "" || hex.EncodeToString(digest[:]) != manifest.SHA256 {
			t.Fatalf("invalid captured project manifest %s", name)
		}
	}

	files := make(map[string]customHistoryNativeSourceFile, len(data.SourceFiles))
	classes := make(map[string]map[string]bool)
	for _, source := range data.SourceFiles {
		if source.Path == "" || filepath.IsAbs(source.Path) || filepath.Clean(source.Path) != source.Path || strings.HasPrefix(source.Path, "..") {
			t.Fatalf("invalid source path %q", source.Path)
		}
		if _, exists := files[source.Path]; exists {
			t.Fatalf("duplicate captured source path %s", source.Path)
		}
		digest := sha256.Sum256([]byte(source.Content))
		if hex.EncodeToString(digest[:]) != source.SHA256 {
			t.Fatalf("source hash mismatch for %s", source.Path)
		}
		files[source.Path] = source
		switch source.Role {
		case "schema":
			if source.ClassName != "" {
				t.Fatalf("schema source has class name: %s", source.Path)
			}
		case "class", "classMetadata":
			if source.ClassName == "" {
				t.Fatalf("class source missing name: %s", source.Path)
			}
			if classes[source.ClassName] == nil {
				classes[source.ClassName] = make(map[string]bool)
			}
			classes[source.ClassName][source.Role] = true
		default:
			t.Fatalf("unknown captured source role %q", source.Role)
		}
	}
	if len(files) < 70 {
		t.Fatalf("captured metadata and class source set is incomplete: %d", len(files))
	}
	for path, source := range files {
		if source.Role == "schema" && !strings.HasPrefix(path, "force-app/main/default/objects/") {
			t.Fatalf("unexpected schema source path %s", path)
		}
	}

	wantFamilies := map[string]int{
		"descriptor":            8,
		"system_helper_insert":  14,
		"direct_test_insert":    2,
		"cross_version_insert":  2,
		"middle_bridge_insert":  1,
		"erased_lifecycle":      2,
		"required_input_insert": 1,
	}
	seenCells := make(map[string]bool)
	gotFamilies := make(map[string]int)
	for _, cell := range data.RuntimeCells {
		if cell.ID == "" || seenCells[cell.ID] || cell.Expected == nil || cell.Marker == "" || cell.Capture == "" {
			t.Fatalf("invalid or repeated native cell %s", cell.ID)
		}
		seenCells[cell.ID] = true
		gotFamilies[cell.Family]++
		if cell.Context != "anonymous" && cell.Context != "istest" {
			t.Fatalf("unknown native context %s in %s", cell.Context, cell.ID)
		}
		if cell.API == "" || cell.ProjectManifest == "" || len(cell.Classes) == 0 {
			t.Fatalf("native cell lacks API or source classes: %s", cell.ID)
		}
		if _, ok := data.ProjectManifests[cell.ProjectManifest]; !ok {
			t.Fatalf("native cell %s lacks project manifest %s", cell.ID, cell.ProjectManifest)
		}
		for _, className := range cell.Classes {
			if !classes[className]["class"] || !classes[className]["classMetadata"] {
				t.Fatalf("native cell %s lacks captured class source %s", cell.ID, className)
			}
		}
		if cell.Context == "anonymous" {
			if cell.Anonymous == "" || cell.TestClass != "" || cell.TestMethod != "" {
				t.Fatalf("invalid anonymous source shape in %s", cell.ID)
			}
		} else if cell.Anonymous != "" || cell.TestClass == "" || cell.TestMethod == "" {
			t.Fatalf("invalid named-test source shape in %s", cell.ID)
		}
	}
	if !reflect.DeepEqual(gotFamilies, wantFamilies) {
		t.Fatalf("native family matrix: got %#v, want %#v", gotFamilies, wantFamilies)
	}

	seenCompile := make(map[string]bool)
	for _, rejected := range data.CompileRejections {
		if rejected.ID == "" || seenCompile[rejected.ID] || rejected.API == "" || rejected.ProjectManifest == "" || len(rejected.Classes) != 1 || len(rejected.ExpectedErrors) == 0 || rejected.Capture == "" {
			t.Fatalf("invalid native compile rejection %s", rejected.ID)
		}
		if _, ok := data.ProjectManifests[rejected.ProjectManifest]; !ok {
			t.Fatalf("compile rejection %s lacks project manifest %s", rejected.ID, rejected.ProjectManifest)
		}
		seenCompile[rejected.ID] = true
		if !classes[rejected.Classes[0]]["class"] || !classes[rejected.Classes[0]]["classMetadata"] {
			t.Fatalf("compile rejection lacks source %s", rejected.Classes[0])
		}
	}
	seenCarries := make(map[string]bool)
	for _, carry := range data.Carries {
		if carry.ID == "" || seenCarries[carry.ID] || carry.API == "" || carry.Outcome == "" || carry.RawNativeMessage == "" || carry.Capture == "" || carry.Reason == "" {
			t.Fatalf("incomplete native carry %s", carry.ID)
		}
		seenCarries[carry.ID] = true
	}
	return data
}

func customHistoryIsSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func customHistoryNativeRuntimeCell(t *testing.T, data customHistoryNativeFixture, cell customHistoryNativeCell) {
	t.Helper()
	index := customHistoryNativeProject(t, data, cell.ProjectManifest, cell.Classes)
	analysis := sema.Analyze(index)
	if index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("native executes captured source: parser=%v semantics=%v", index.Diagnostics, analysis.Diagnostics)
	}
	org := orgFromIndex(index)
	org.APIVersion = cell.API
	runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
	var payload string
	if cell.Context == "anonymous" {
		anonymousAnalysis := sema.AnalyzeAnonymous(index, cell.Anonymous, cell.API)
		program, err := vm.CompileAnonymousWithOptions(cell.Anonymous, vm.CompileOptions{APIVersion: cell.API})
		if anonymousAnalysis.HasErrors() || err != nil {
			t.Fatalf("native anonymous source: semantics=%v compile=%v", anonymousAnalysis.Diagnostics, err)
		}
		result, err := runner.execute(program)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range result.Debug {
			if at := strings.Index(line, cell.Marker); at >= 0 {
				if payload != "" {
					t.Fatal("duplicate captured debug transport")
				}
				payload = line[at+len(cell.Marker):]
			}
		}
	} else {
		cases := Discover(index, Options{SelectedClasses: []string{cell.TestClass}, SelectedMethod: cell.TestMethod})
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
		if at := strings.Index(message, cell.Marker); at >= 0 {
			payload = message[at+len(cell.Marker):]
		}
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(payload), &got); err != nil || got == nil {
		t.Fatalf("missing or invalid captured payload %q: %v", payload, err)
	}
	if !reflect.DeepEqual(got, cell.Expected) {
		t.Fatalf("native value=%#v; local=%#v", cell.Expected, got)
	}
}

func assertCustomHistoryNativeCompileRejection(t *testing.T, data customHistoryNativeFixture, rejected customHistoryNativeCompileRejection) {
	t.Helper()
	index := customHistoryNativeProject(t, data, rejected.ProjectManifest, rejected.Classes)
	analysis := sema.Analyze(index)
	if index.HasErrors() {
		t.Fatalf("captured compile rejection has parser errors: %v", index.Diagnostics)
	}
	var got []string
	for _, item := range analysis.Diagnostics {
		if item.Severity == diagnostic.Error {
			got = append(got, item.Message)
		}
	}
	// Unknown-type diagnostics use Glade's diagnostic vocabulary; the native
	// contract here is rejection of that unavailable type, including cascades.
	if len(rejected.ExpectedErrors) == 1 && strings.HasPrefix(rejected.ExpectedErrors[0], "Invalid type: ") {
		name := strings.TrimPrefix(rejected.ExpectedErrors[0], "Invalid type: ")
		for _, item := range analysis.Diagnostics {
			if item.Severity == diagnostic.Error && item.Code == "GLADESEMA006" && strings.Contains(item.Message, name) {
				return
			}
		}
		t.Fatalf("missing unknown-type rejection for %s: %v", name, analysis.Diagnostics)
	}
	want := append([]string(nil), rejected.ExpectedErrors...)
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("native semantic rejection=%q; local=%q; diagnostics=%#v", want, got, analysis.Diagnostics)
	}
}

func customHistoryNativeProject(t *testing.T, data customHistoryNativeFixture, manifestName string, classNames []string) typesys.Index {
	t.Helper()
	root := t.TempDir()
	manifest, ok := data.ProjectManifests[manifestName]
	if !ok {
		t.Fatalf("missing native project manifest %s", manifestName)
	}
	writeFile(t, filepath.Join(root, "sfdx-project.json"), manifest.Content)
	classFiles := make(map[string]map[string]customHistoryNativeSourceFile)
	for _, source := range data.SourceFiles {
		if source.Role == "schema" {
			writeFile(t, filepath.Join(root, filepath.FromSlash(source.Path)), source.Content)
			continue
		}
		if classFiles[source.ClassName] == nil {
			classFiles[source.ClassName] = make(map[string]customHistoryNativeSourceFile)
		}
		classFiles[source.ClassName][source.Role] = source
	}
	for _, name := range classNames {
		for _, role := range []string{"class", "classMetadata"} {
			source, ok := classFiles[name][role]
			if !ok {
				t.Fatalf("missing captured %s for %s", role, name)
			}
			writeFile(t, filepath.Join(root, filepath.FromSlash(source.Path)), source.Content)
		}
	}
	return loadTestIndex(t, root)
}
