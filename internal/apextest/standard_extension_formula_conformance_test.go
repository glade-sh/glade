package apextest

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

type standardExtensionNativeRow struct {
	ID, Variant, Expression, ExpressionSHA256, CarryReason string
	Native                                                 json.RawMessage
	ProjectionCarries                                      map[string]string
	Metadata, MetadataSHA256                               map[string]string
}

type standardExtensionNativeCapture struct {
	API, Context, Route, Namespace      string
	BindingSHA256, SourceManifestSHA256 string
	Metadata, MetadataSHA256            map[string]string
	Rows                                []standardExtensionNativeRow
}

type standardExtensionNativeData struct {
	APIVersions, UncapturedAPIs []string
	Excluded                    map[string]string
	Captures                    []standardExtensionNativeCapture
	PendingCells                []struct{ API, Context, Route, ID, NativeOutcome, Reason string }
}

func TestStandardExtensionFormulaOrgConformance(t *testing.T) {
	runStandardExtensionNative(t, "standard_extension_formula", 17)
}

// Captured callers and metadata are pinned to their observed API. Native
// anonymous and named values stay separate; missing contexts never use a twin.
func runStandardExtensionNative(t *testing.T, family string, rowCount int) {
	t.Helper()
	var data standardExtensionNativeData
	raw, err := os.ReadFile("testdata/conformance/" + family + ".json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Captures) != 9 || strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" || len(data.UncapturedAPIs) != 0 || len(data.Excluded) != 3 {
		t.Fatal("incomplete native matrix disposition")
	}
	if len(data.PendingCells) != 0 {
		t.Fatal("incomplete unanswered API matrix")
	}
	seenPending := map[string]bool{}
	for _, cell := range data.PendingCells {
		key := cell.API + "/" + cell.Context + "/" + cell.Route + "/" + cell.ID
		if cell.API != "68.0" || (cell.Context != "namespace" || cell.Route != "istest") || cell.NativeOutcome != "NOT_RUN" || cell.Reason == "" || seenPending[key] {
			t.Fatal("pending cell lacks explicit non-observation")
		}
		seenPending[key] = true
	}
	for _, api := range data.UncapturedAPIs {
		t.Logf("pending API %s: uncaptured context/route cells remain", api)
	}
	for cell, reason := range data.Excluded {
		if reason == "" {
			t.Fatal("excluded cell lacks reason")
		}
		t.Logf("excluded %s: %s", cell, reason)
	}
	runStandardExtensionNativeCaptures(t, family, data.Captures, rowCount)
}

func runStandardExtensionNativeCaptures(t *testing.T, family string, captures []standardExtensionNativeCapture, rowCount int) {
	t.Helper()
	bases := newConformanceSchemaCache(t)
	seenCapture := map[string]bool{}
	for _, capture := range captures {
		name := capture.Context + "/" + capture.Route
		if capture.API != "62.0" {
			name = capture.API + "/" + name
		}
		key := capture.API + "/" + capture.Context + "/" + capture.Route
		if key != "62.0/plain/anonymous" && key != "62.0/plain/istest" && key != "62.0/namespace/istest" && key != "67.0/plain/anonymous" && key != "67.0/plain/istest" && key != "67.0/namespace/istest" && key != "68.0/plain/anonymous" && key != "68.0/plain/istest" && key != "68.0/namespace/istest" || seenCapture[key] {
			t.Fatal("duplicate or unobserved context route")
		}
		seenCapture[key] = true
		runConformanceSubtestParallel(t, name, func(t *testing.T) {
			if (capture.Context == "namespace" && (capture.Route != "istest" || capture.Namespace != "gladens003")) || (capture.Context == "plain" && capture.Namespace != "") || len(capture.Rows) != rowCount {
				t.Fatal("invalid independent native capture")
			}
			counts := newConformanceCountLog(t, family, capture.API)
			seen := map[string]bool{}
			for _, row := range capture.Rows {
				if row.ID == "" || seen[row.ID] || fmt.Sprintf("%x", sha256.Sum256([]byte(row.Expression))) != row.ExpressionSHA256 {
					t.Fatal("native row changed or duplicated")
				}
				seen[row.ID] = true
				var native struct {
					Outcome string
					Value   json.RawMessage
				}
				if err := json.Unmarshal(row.Native, &native); err != nil || native.Outcome == "" {
					t.Fatal("missing native evidence")
				}
				kind := "exact"
				if row.CarryReason != "" {
					kind = "carried"
				} else if len(row.ProjectionCarries) > 0 {
					kind = "partial"
				}
				counts.RunKind(t, capture.Route, row.ID, row.ID, kind, func(t *testing.T) {
					if row.CarryReason != "" {
						if native.Outcome != "NO_ANSWER" && row.ID != "CTRL_UNCAUGHT" {
							t.Fatal("whole-cell carry must retain a nonanswer")
						}
						if row.ID == "CTRL_UNCAUGHT" {
							var observed overrideConversionNative
							if err := json.Unmarshal(row.Native, &observed); err != nil || observed.ExceptionType != "System.MathException" || observed.ExceptionMessage != "System.MathException: Divide by 0" {
								t.Fatal("uncaught carry lacks exact native evidence")
							}
						}
						t.Logf("carry: %s; native=%s", row.CarryReason, row.Native)
						return
					}
					if row.ID == "CTRL_COMPILE" {
						index := standardExtensionNativeIndex(t, capture, row)
						analysis := sema.Analyze(index)
						if !index.HasErrors() && !analysis.HasErrors() {
							t.Fatal("native compile rejection was accepted")
						}
						if native.Outcome != "compile-rejected" {
							t.Fatal("missing compiler evidence")
						}
						var evidence struct{ Diagnostics json.RawMessage }
						if err := json.Unmarshal(row.Native, &evidence); err != nil {
							t.Fatal(err)
						}
						problem := ""
						if capture.Route == "anonymous" {
							var observed struct{ CompileProblem string }
							if err := json.Unmarshal(evidence.Diagnostics, &observed); err != nil {
								t.Fatal(err)
							}
							problem = observed.CompileProblem
						} else {
							var observed []nativeBoundaryDiagnostic
							if err := json.Unmarshal(evidence.Diagnostics, &observed); err != nil || len(observed) != 1 {
								t.Fatal("missing compiler problem")
							}
							problem = observed[0].Problem
						}
						text := conformanceCompilerText(analysis.Diagnostics, nil)
						if problem != "Illegal assignment from String to Integer" || !strings.HasSuffix(text, problem) {
							t.Fatalf("native compiler problem=%s; local=%s", problem, text)
						}
						return
					}
					if native.Outcome != "runtime" && row.ID != "CTRL_UNCAUGHT" || native.Outcome == "runtime" && !json.Valid(native.Value) {
						t.Fatal("invalid answered native cell")
					}
					index := standardExtensionNativeIndex(t, capture, row)
					if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
						t.Fatalf("native accepted; local diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
					}
					objects := standardExtensionNativeObjects(t, capture, row)
					base := bases.get(t, capture.API, capture.Namespace, objects, func(t *testing.T) *conformanceRunner {
						return conformanceSchemaBase(t, standardExtensionNativeProjectIndex(t, capture, row, false), capture.API)
					})
					runner := newConformanceRunner(t, index, conformanceRunnerOptions{Base: base, LinkProject: true, Org: base.orgSource})
					source := standardExtensionNativeBody(t, row)
					var runErr error
					if capture.Route == "anonymous" {
						if analysis := sema.AnalyzeAnonymous(index, source, capture.API); analysis.HasErrors() {
							t.Fatal(analysis.Diagnostics)
						}
						program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: capture.API})
						if err != nil {
							t.Fatal(err)
						}
						_, runErr = runner.execute(program)
					} else {
						cases := Discover(index, Options{SelectedClasses: []string{"ExtensionObservation"}, SelectedMethod: "observe"})
						if len(cases) != 1 {
							t.Fatalf("named discovery=%d", len(cases))
						}
						methods, methodErrors := compileTestMethods(cases)
						programs, programErrors := compileTestInvokePrograms(cases)
						key := testCaseKey(cases[0])
						if methodErrors[key] != nil || programErrors[key] != nil {
							t.Fatalf("named compile=%v %v", methodErrors[key], programErrors[key])
						}
						machine := runner.newMachine()
						if err := machine.RegisterMethod(methods[key]); err != nil {
							t.Fatal(err)
						}
						machine.EnableTestContext()
						_, runErr = machine.ExecuteInClass(programs[key], cases[0].ClassName)
					}
					if row.ID == "CTRL_UNCAUGHT" {
						var observed overrideConversionNative
						if err := json.Unmarshal(row.Native, &observed); err != nil {
							t.Fatal(err)
						}
						assertOverrideConversionValue(t, row.ID, observed, runErr)
						return
					}
					var runtimeErr *vm.RuntimeError
					if !errors.As(runErr, &runtimeErr) || runtimeErr.Type != "System.AssertException" {
						t.Fatalf("missing observation: %v", runErr)
					}
					_, payload, found := strings.Cut(runtimeErr.ExceptionMessage(), "EXTENSION_RESULT|"+row.ID+"|")
					var actual, expected map[string]any
					if !found || json.Unmarshal([]byte(payload), &actual) != nil || json.Unmarshal(native.Value, &expected) != nil {
						t.Fatalf("invalid observation: %v", runErr)
					}
					for field, reason := range row.ProjectionCarries {
						value, exists := expected[field]
						if !exists || reason == "" {
							t.Fatal("partial field lacks native evidence or reason")
						}
						t.Logf("partial %s: %s; native=%v; local=%v", field, reason, value, actual[field])
						kind, _, _ := strings.Cut(reason, ":")
						expected[field], actual[field] = standardExtensionProjection(t, kind, value, actual[field])
					}
					if !reflect.DeepEqual(actual, expected) {
						t.Fatalf("native=%s; local=%s", native.Value, payload)
					}
				})
			}
		})
	}
}

func standardExtensionNativeBody(t *testing.T, row standardExtensionNativeRow) string {
	if row.ID == "CTRL_UNCAUGHT" || row.ID == "CTRL_COMPILE" {
		return row.Expression
	}
	var native struct {
		MarkerNormalisation string `json:"marker_normalisation_apex"`
	}
	if err := json.Unmarshal(row.Native, &native); err != nil {
		t.Fatal(err)
	}
	prefix, payload := "", "JSON.serialize(result)"
	if native.MarkerNormalisation != "" {
		prefix = "Map<String,String> replayIds=new Map<String,String>();"
		payload = "stableRaw"
	}
	return prefix + "Map<String,Object> result=new Map<String,Object>();String step='start';try{" + row.Expression + "}catch(Exception failure){result.put('exceptionType',failure.getTypeName());result.put('exceptionMessage',failure.getMessage());result.put('exceptionStep',step);}" + native.MarkerNormalisation + "System.assert(false,'EXTENSION_RESULT|" + row.ID + "|'+" + payload + ");"
}

func standardExtensionNativeObjects(t *testing.T, capture standardExtensionNativeCapture, row standardExtensionNativeRow) map[string]string {
	t.Helper()
	objects := make(map[string]string)
	// A supplemental row has fresh metadata names; other captured rows retain
	// their original metadata and source unchanged.
	if len(row.Metadata) > 0 {
		capture.Metadata, capture.MetadataSHA256 = row.Metadata, row.MetadataSHA256
	}
	for name, body := range capture.Metadata {
		if fmt.Sprintf("%x", sha256.Sum256([]byte(body))) != capture.MetadataSHA256[name] {
			t.Fatal("native metadata changed")
		}
		// The saved fixture zip is the final present phase. Keep its package and
		// permission XML as hashed evidence, but replay only the row's object phase.
		// These rows do not query permissions; loading the final-phase grants would
		// make permission-driven field inference recreate the absent lookup.
		if !strings.HasPrefix(name, "objects/") || !strings.HasSuffix(name, ".object") {
			continue
		}
		if row.Variant == "absent" && name == "objects/Account.object" {
			start := strings.Index(body, "<fields><fullName>")
			for start >= 0 {
				end := strings.Index(body[start:], "</fields>")
				if end < 0 {
					t.Fatal("invalid captured field metadata")
				}
				end += start + len("</fields>")
				if strings.Contains(body[start:end], "<type>Lookup</type>") {
					body = body[:start] + body[end:]
					break
				}
				next := strings.Index(body[end:], "<fields><fullName>")
				if next < 0 {
					break
				}
				start = end + next
			}
		}
		objects[name] = body
	}
	return objects
}

func standardExtensionNativeIndex(t *testing.T, capture standardExtensionNativeCapture, row standardExtensionNativeRow) typesys.Index {
	t.Helper()
	return standardExtensionNativeProjectIndex(t, capture, row, true)
}

func standardExtensionNativeProjectIndex(t *testing.T, capture standardExtensionNativeCapture, row standardExtensionNativeRow, source bool) typesys.Index {
	t.Helper()
	root := t.TempDir()
	p := project.Project{Root: root, Namespace: capture.Namespace, SourceAPIVersion: capture.API}
	for name, body := range standardExtensionNativeObjects(t, capture, row) {
		path := filepath.Join(root, name)
		writeFile(t, path, body)
		p.ObjectFiles = append(p.ObjectFiles, path)
	}
	if source {
		source := "@IsTest private class ExtensionObservation {@IsTest static void observe(){" + standardExtensionNativeBody(t, row) + "}}"
		path := filepath.Join(root, "classes", "ExtensionObservation.cls")
		writeFile(t, path, source)
		writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+capture.API+"</apiVersion></ApexClass>")
		p.ApexFiles = []string{path}
	}
	schema, err := gladeschema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	return typesys.Build(p, schema)
}

// Projections retain every core namespace/formula/lookup observation. Carries
// record the untouched native values and explain only the omitted defaults,
// unstable Id text, platform URL, ordering, or generated companion type.
func standardExtensionProjection(t *testing.T, kind string, expected, actual any) (any, any) {
	t.Helper()
	switch kind {
	case "formula-json-defaults-order", "lookup-json-id-url-defaults":
		var native, local any
		ns, nok := expected.(string)
		ls, lok := actual.(string)
		if !nok || !lok || json.Unmarshal([]byte(ns), &native) != nil || json.Unmarshal([]byte(ls), &local) != nil {
			t.Fatal("invalid captured SObject JSON")
		}
		standardExtensionDropJSONDefaults(native, kind == "lookup-json-id-url-defaults")
		standardExtensionDropJSONDefaults(local, kind == "lookup-json-id-url-defaults")
		if kind == "lookup-json-id-url-defaults" {
			standardExtensionProjectIDs(t, native, local)
		}
		return native, local
	case "formula-key-defaults-order", "global-changeevent":
		companion := ""
		if kind == "global-changeevent" {
			list, ok := expected.([]any)
			if !ok {
				t.Fatal("invalid native global keys")
			}
			for _, item := range list {
				key, ok := item.(string)
				if !ok {
					t.Fatal("invalid native global key")
				}
				if strings.HasSuffix(strings.ToLower(key), "__changeevent") {
					if companion != "" {
						t.Fatal("unexpected native companion multiplicity")
					}
					companion = key
				}
			}
			if companion == "" {
				t.Fatal("missing carried native companion")
			}
		}
		filter := func(value any) []string {
			list, ok := value.([]any)
			if !ok {
				t.Fatal("captured key set is not an array")
			}
			out := make([]string, 0, len(list))
			for _, item := range list {
				key, ok := item.(string)
				if !ok {
					t.Fatal("captured key is not String")
				}
				if kind == "formula-key-defaults-order" && standardExtensionDefaultKey(key) {
					continue
				}
				if kind == "global-changeevent" && key == companion {
					continue
				}
				out = append(out, key)
			}
			sort.Strings(out)
			return out
		}
		return filter(expected), filter(actual)
	case "lookup-map-id-url-defaults":
		standardExtensionDropJSONDefaults(expected, true)
		standardExtensionDropJSONDefaults(actual, true)
		standardExtensionProjectIDs(t, expected, actual)
		return expected, actual
	case "lookup-read-id":
		standardExtensionProjectIDs(t, expected, actual)
		return expected, actual
	case "lookup-parent-json":
		native, nok := expected.([]any)
		local, lok := actual.([]any)
		if !nok || !lok || len(native) != len(local) {
			t.Fatal("parent read count changed")
		}
		for i, item := range native {
			nm, ok := item.(map[string]any)
			lm, lok := local[i].(map[string]any)
			if !ok || !lok {
				t.Fatal("invalid parent observation")
			}
			nj, exists := nm["json"]
			if !exists {
				continue
			}
			lj := lm["json"]
			nm["json"], lm["json"] = standardExtensionProjection(t, "lookup-json-id-url-defaults", nj, lj)
		}
		return native, local
	default:
		t.Fatalf("unknown partial observation %s", kind)
		return expected, actual
	}
}

func standardExtensionDefaultKey(key string) bool {
	switch key {
	case "OwnerId", "Division", "IsCustomerPortal", "IsDeleted":
		return true
	}
	return false
}

func standardExtensionDropJSONDefaults(value any, lookup bool) {
	object, ok := value.(map[string]any)
	if !ok {
		return
	}
	for key, child := range object {
		if standardExtensionDefaultKey(key) {
			delete(object, key)
			continue
		}
		if lookup && key == "attributes" {
			if attrs, ok := child.(map[string]any); ok {
				delete(attrs, "url")
			}
		}
		if lookup {
			standardExtensionDropJSONDefaults(child, true)
		}
	}
}

func standardExtensionProjectIDs(t *testing.T, native, local any) {
	t.Helper()
	switch expected := native.(type) {
	case map[string]any:
		actual, ok := local.(map[string]any)
		if !ok {
			return
		}
		for key, value := range expected {
			if marker, ok := value.(string); ok && marker == "[ID]" {
				text, ok := actual[key].(string)
				if !ok || len(text) != 15 && len(text) != 18 {
					t.Fatal("local Id effect missing")
				}
				actual[key] = "[ID]"
			} else {
				standardExtensionProjectIDs(t, value, actual[key])
			}
		}
	case []any:
		actual, ok := local.([]any)
		if !ok || len(actual) != len(expected) {
			return
		}
		for i, value := range expected {
			standardExtensionProjectIDs(t, value, actual[i])
		}
	}
}
