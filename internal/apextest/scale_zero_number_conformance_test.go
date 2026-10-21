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
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

const scaleZeroNumberFixture = "testdata/conformance/scale_zero_number"

type scaleZeroNumberNative struct {
	Outcome     string          `json:"outcome"`
	Value       json.RawMessage `json:"value"`
	Diagnostics json.RawMessage `json:"diagnostics"`
}
type scaleZeroNumberCell struct {
	API, Route, PrimaryDiagnostic, Carry string
	Native                               scaleZeroNumberNative
}
type scaleZeroNumberSource struct {
	Source, SourceSHA256, ClassName string
}
type scaleZeroNumberRow struct {
	ID, Group, Twin string
	Routes          map[string]scaleZeroNumberSource
	Cells           []scaleZeroNumberCell
}
type scaleZeroNumberData struct {
	APIVersions, Routes                []string
	ShapeCount, NativeObservationCount int
	NoAnswerRows                       int
	RowErrorsFilePresent               bool
	SourceManifestSHA256               string
	Files                              []struct{ Path, SHA256 string }
	Rows                               []scaleZeroNumberRow
}

func TestScaleZeroNumberMemoryOrgConformance(t *testing.T) {
	runScaleZeroNumberConformance(t, "memory", 20)
}
func TestScaleZeroNumberBoundariesOrgConformance(t *testing.T) {
	runScaleZeroNumberConformance(t, "boundaries", 17)
}
func TestScaleZeroNumberPersistedQueryOrgConformance(t *testing.T) {
	runScaleZeroNumberConformance(t, "persisted-query", 15)
}
func TestScaleZeroNumberAggregateOrgConformance(t *testing.T) {
	runScaleZeroNumberConformance(t, "aggregate", 8)
}
func TestScaleZeroNumberEqualityMapOrgConformance(t *testing.T) {
	runScaleZeroNumberConformance(t, "equality-map", 8)
}
func TestScaleZeroNumberTriggerOrgConformance(t *testing.T) {
	runScaleZeroNumberConformance(t, "trigger", 40)
}
func TestScaleZeroNumberDirectCastOrgConformance(t *testing.T) {
	runScaleZeroNumberConformance(t, "direct-cast", 18)
}
func TestScaleZeroNumberTransportControlOrgConformance(t *testing.T) {
	runScaleZeroNumberConformance(t, "transport-control", 1)
}
func TestScaleZeroNumberObjectInstanceofOrgConformance(t *testing.T) {
	runScaleZeroNumberConformance(t, "object-instanceof", 2)
}

// Replay the exact captured source, including its observation assertion. JSON
// object ordering is transport framing; scalar strings, nulls, types, casts,
// messages and key-lookup results are compared without normalization. Each
// cell gets a fresh shared-runner VM/org, including private trigger statics.
func runScaleZeroNumberConformance(t *testing.T, group string, shapes int) {
	t.Helper()
	data := scaleZeroNumberLoad(t)
	var rows []scaleZeroNumberRow
	for _, row := range data.Rows {
		if row.Group == group {
			rows = append(rows, row)
		}
	}
	if len(rows) != shapes {
		t.Fatalf("cluster %s: %d shapes, want %d", group, len(rows), shapes)
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "ScaleZeroNumber_"+group, api)
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			for name, folder := range map[string]string{"C155Number__c.object": "objects/C155Number__c/C155Number__c.object-meta.xml", "C155NITriggerView.cls": "classes/C155NITriggerView.cls", "C155NIObserve.trigger": "triggers/C155NIObserve.trigger"} {
				text, err := os.ReadFile(filepath.Join(scaleZeroNumberFixture, "setup", name))
				if err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(root, "force-app/main/default", folder)
				writeFile(t, path, string(text))
				if strings.HasSuffix(name, ".cls") {
					writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
				} else if strings.HasSuffix(name, ".trigger") {
					writeFile(t, path+"-meta.xml", "<ApexTrigger><apiVersion>"+api+"</apiVersion></ApexTrigger>")
				}
			}
			fixtureProject, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			metadata, err := gladeschema.LoadProject(fixtureProject)
			if err != nil {
				t.Fatal(err)
			}
			buildIndex := func(source scaleZeroNumberSource) typesys.Index {
				p := fixtureProject
				p.ApexFiles = append([]string{}, fixtureProject.ApexFiles...)
				if source.ClassName != "" {
					path := filepath.Join(root, "force-app/main/default/classes", source.ClassName+".cls")
					writeFile(t, path, source.Source)
					writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
					p.ApexFiles = append(p.ApexFiles, path)
				}
				return typesys.Build(p, metadata)
			}
			index := buildIndex(scaleZeroNumberSource{})
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("captured setup accepted; Glade rejected: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			if len(index.Triggers) != 1 || !strings.EqualFold(index.Triggers[0].Name, "C155NIObserve") || !strings.EqualFold(index.Triggers[0].ObjectName, "C155Number__c") || len(index.Triggers[0].Events) != 2 {
				t.Fatalf("captured setup requires one two-event trigger declaration: %v", index.Triggers)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runtime, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
			if err != nil {
				t.Fatal(err)
			}
			// compileProjectTriggers expands each declaration into one runtime
			// registration per event. The captured before/after insert trigger
			// therefore has two registrations, not one compiled registration.
			events := map[string]bool{"before insert": true, "after insert": true}
			if len(runtime.Triggers) != len(events) {
				t.Fatalf("captured setup requires two insert event registrations; got %d: %v", len(runtime.Triggers), runtime.Triggers)
			}
			for _, trigger := range runtime.Triggers {
				event := trigger.Timing + " " + trigger.Operation
				if !events[event] || !strings.EqualFold(trigger.Name, "C155NIObserve") || !strings.EqualFold(trigger.Object, "C155Number__c") || trigger.APIVersion != api {
					t.Fatalf("invalid/repeated captured trigger registration: %s %s %s API %s", trigger.Name, trigger.Object, event, trigger.APIVersion)
				}
				delete(events, event)
			}
			base := newConformanceRunner(t, index, conformanceRunnerOptions{RequestRuntime: &runtime, Org: &org})
			for ordinal, row := range rows {
				t.Run(fmt.Sprintf("case_%02d", ordinal), func(t *testing.T) {
					for _, route := range data.Routes {
						t.Run(route, func(t *testing.T) {
							cell := scaleZeroNumberFind(t, row, api, route)
							countRoute := route
							if route == "istest" {
								countRoute = "isTest"
							}
							if cell.Carry != "" {
								counts.TrackKind(t, countRoute, "carried", 1)
								t.Logf("%s/%s/%s carried: %s", row.ID, api, route, cell.Carry)
								return
							}
							counts.TrackKind(t, countRoute, "exact", 1)
							source := row.Routes[route]
							if route == "anonymous" {
								analysis := sema.AnalyzeAnonymous(index, source.Source, api)
								program, compileErr := vm.CompileAnonymousWithOptions(source.Source, vm.CompileOptions{APIVersion: api})
								if !scaleZeroNumberCompile(t, cell, analysis.Diagnostics, compileErr) {
									return
								}
								_, execErr := base.execute(program)
								scaleZeroNumberObservation(t, row.ID, cell, execErr)
								return
							}
							namedIndex := buildIndex(source)
							diagnostics := append(append([]diagnostic.Diagnostic{}, namedIndex.Diagnostics...), sema.Analyze(namedIndex).Diagnostics...)
							if !scaleZeroNumberCompile(t, cell, diagnostics, nil) {
								return
							}
							cases := Discover(namedIndex, Options{SelectedClasses: []string{source.ClassName}, SelectedMethod: "observe"})
							if len(cases) != 1 {
								t.Fatalf("one captured test method required, got %d", len(cases))
							}
							methods, methodErrors := compileTestMethods(cases)
							programs, programErrors := compileTestInvokePrograms(cases)
							key := testCaseKey(cases[0])
							if methodErrors[key] != nil || programErrors[key] != nil {
								t.Fatalf("named compilation: %v %v", methodErrors[key], programErrors[key])
							}
							// The API base already links the captured trigger. Link only
							// new classes here so the trigger is not registered twice.
							named := newConformanceRunner(t, namedIndex, conformanceRunnerOptions{Base: base, LinkProject: true, Org: &org})
							machine := named.newMachine()
							machine.EnableTestContext()
							if err := machine.RegisterMethod(methods[key]); err != nil {
								t.Fatal(err)
							}
							_, execErr := machine.ExecuteInClass(programs[key], cases[0].ClassName)
							scaleZeroNumberObservation(t, row.ID, cell, execErr)
						})
					}
				})
			}
		})
	}
}

func scaleZeroNumberLoad(t *testing.T) scaleZeroNumberData {
	t.Helper()
	var data scaleZeroNumberData
	raw, err := os.ReadFile(filepath.Join(scaleZeroNumberFixture, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data.ShapeCount != 129 || data.NativeObservationCount != 774 || len(data.Rows) != 129 || data.NoAnswerRows != 0 || data.RowErrorsFilePresent || strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" || strings.Join(data.Routes, ",") != "anonymous,istest" {
		t.Fatal("incomplete r3 native matrix")
	}
	native := map[string]map[string]scaleZeroNumberNative{}
	listed := map[string]bool{}
	for _, file := range data.Files {
		if listed[file.Path] || filepath.IsAbs(file.Path) || strings.Contains(file.Path, "..") {
			t.Fatal("invalid fixture file")
		}
		listed[file.Path] = true
		text, err := os.ReadFile(filepath.Join(scaleZeroNumberFixture, file.Path))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(text)
		if hex.EncodeToString(sum[:]) != file.SHA256 {
			t.Fatalf("captured setup/evidence changed: %s", file.Path)
		}
		if strings.HasSuffix(file.Path, "/org.tsv") {
			cells := map[string]scaleZeroNumberNative{}
			lines := strings.Split(strings.TrimSuffix(string(text), "\n"), "\n")
			if len(lines) != 130 || lines[0] != "id\tgroup\troute\tstatus\tvalue" {
				t.Fatal("incomplete native TSV")
			}
			for _, line := range lines[1:] {
				fields := strings.SplitN(line, "\t", 5)
				if len(fields) != 5 || cells[fields[0]].Outcome != "" {
					t.Fatal("invalid/repeated native TSV row")
				}
				var value scaleZeroNumberNative
				if err := json.Unmarshal([]byte(fields[4]), &value); err != nil || value.Outcome != fields[3] {
					t.Fatal("invalid native TSV value")
				}
				cells[fields[0]] = value
			}
			native[file.Path] = cells
		}
	}
	if len(native) != 6 || len(listed) != 16 {
		t.Fatal("missing native evidence or setup file")
	}
	seen := map[string]bool{}
	for _, row := range data.Rows {
		if seen[row.ID] || row.ID == "" || len(row.Routes) != 2 || len(row.Cells) != 6 {
			t.Fatal("invalid native row")
		}
		seen[row.ID] = true
		for _, route := range data.Routes {
			source, ok := row.Routes[route]
			sum := sha256.Sum256([]byte(source.Source))
			if !ok || source.Source == "" || hex.EncodeToString(sum[:]) != source.SourceSHA256 || (route == "istest") != (source.ClassName != "") {
				t.Fatal("captured source hash/route mismatch")
			}
			for _, api := range data.APIVersions {
				cell := scaleZeroNumberFind(t, row, api, route)
				path := "native/api" + strings.TrimSuffix(api, ".0") + "/" + route + "/org.tsv"
				want, ok := native[path][row.ID]
				if !ok || !scaleZeroNumberJSONEqual(cell.Native, want) {
					t.Fatalf("case detached from native TSV: %s/%s/%s", row.ID, api, route)
				}
				if cell.Native.Outcome == "compile-rejected" && cell.PrimaryDiagnostic != scaleZeroNumberNativePrimary(t, cell.Native.Diagnostics) {
					t.Fatal("primary diagnostic detached from native evidence")
				}
				if cell.Carry == "" && cell.Native.Outcome != "runtime" && (cell.Native.Outcome != "compile-rejected" || cell.PrimaryDiagnostic == "") {
					t.Fatal("artifact outcomes require an explicit carry")
				}
				if cell.Carry != "" && (cell.Native.Outcome == "runtime" || cell.PrimaryDiagnostic != "") {
					t.Fatal("native behavioral answers must be asserted")
				}
			}
		}
	}
	for _, row := range data.Rows {
		if !seen[row.Twin] {
			t.Fatalf("missing captured twin: %s", row.ID)
		}
	}
	return data
}

func scaleZeroNumberNativePrimary(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var diagnostics struct {
		CompileProblem string
		Compile        []struct{ Problem string }
	}
	if err := json.Unmarshal(raw, &diagnostics); err != nil {
		t.Fatal(err)
	}
	if diagnostics.CompileProblem != "" {
		return diagnostics.CompileProblem
	}
	for _, problem := range diagnostics.Compile {
		if problem.Problem != "" {
			return problem.Problem
		}
	}
	return ""
}

func scaleZeroNumberFind(t *testing.T, row scaleZeroNumberRow, api, route string) scaleZeroNumberCell {
	t.Helper()
	var found []scaleZeroNumberCell
	for _, cell := range row.Cells {
		if cell.API == api && cell.Route == route {
			found = append(found, cell)
		}
	}
	if len(found) != 1 {
		t.Fatalf("missing/repeated native cell: %s/%s/%s", row.ID, api, route)
	}
	return found[0]
}

func scaleZeroNumberCompile(t *testing.T, cell scaleZeroNumberCell, items []diagnostic.Diagnostic, compileErr error) bool {
	t.Helper()
	primary := ""
	for _, item := range items {
		if item.Severity == diagnostic.Error {
			primary = item.NativeMessage
			if primary == "" {
				primary = item.Message
			}
			break
		}
	}
	if primary == "" && compileErr != nil {
		primary = compileErr.Error()
		var syntax *vm.ApexSyntaxError
		if errors.As(compileErr, &syntax) && syntax.NativeMessage != "" {
			primary = syntax.NativeMessage
		}
	}
	if cell.Native.Outcome == "compile-rejected" {
		if primary != cell.PrimaryDiagnostic {
			t.Fatalf("native primary <%s>; Glade <%s> diagnostics=%v compile=%v", cell.PrimaryDiagnostic, primary, items, compileErr)
		}
		return false
	}
	if primary != "" || compileErr != nil {
		t.Fatalf("native compilation accepted; Glade rejected: %v %v", items, compileErr)
	}
	return true
}

func scaleZeroNumberObservation(t *testing.T, id string, cell scaleZeroNumberCell, execErr error) {
	t.Helper()
	var runtimeErr *vm.RuntimeError
	if execErr == nil || !errors.As(execErr, &runtimeErr) || runtimeErr.Type != "System.AssertException" {
		t.Fatalf("captured terminal assertion missing: %v", execErr)
	}
	marker := "C126_RESULT|" + id + "|"
	message := runtimeErr.ExceptionMessage()
	_, observed, found := strings.Cut(message, marker)
	if !found || strings.Count(message, marker) != 1 {
		t.Fatalf("missing/duplicate owned marker: %s", message)
	}
	var value json.RawMessage
	if err := json.Unmarshal([]byte(observed), &value); err != nil {
		t.Fatalf("invalid observation JSON: %s", observed)
	}
	if !scaleZeroNumberJSONEqual(value, cell.Native.Value) {
		t.Fatalf("native %s; Glade %s", cell.Native.Value, observed)
	}
}

func scaleZeroNumberJSONEqual(left, right any) bool {
	decode := func(value any) (any, error) {
		raw, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		var result any
		err = json.Unmarshal(raw, &result)
		return result, err
	}
	a, errA := decode(left)
	b, errB := decode(right)
	return errA == nil && errB == nil && reflect.DeepEqual(a, b)
}
