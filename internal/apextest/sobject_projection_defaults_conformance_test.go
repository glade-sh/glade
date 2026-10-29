package apextest

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

const projectionDefaultsFixture = "testdata/conformance/sobject_projection_defaults"

type projectionDefaultsCell struct {
	API, Route    string
	Native        json.RawMessage
	CarriedFields map[string]string
}

type projectionDefaultsRow struct {
	ID, Group, Body, AssertionBody string
	Sections                       []string
	Cells                          []projectionDefaultsCell
}

type projectionDefaultsData struct {
	APIVersions, Routes                []string
	ShapeCount, NativeObservationCount int
	Rows                               []projectionDefaultsRow
}

type projectionDefaultsNamedPrograms struct {
	Methods  map[string]vm.Method
	Programs map[string]ir.Program
}

// Replay every independent native cell. Preserve full native snapshots and
// explicit field-level carries; the equality and projection assertions never
// infer an answer from another API, route, or clone shape.
func TestSObjectProjectionAndDefaultsOrgConformance(t *testing.T) {
	data, objects := loadProjectionDefaultsData(t)
	for _, api := range data.APIVersions {
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "SObjectProjectionDefaults", api)
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			for name, source := range objects {
				writeFile(t, filepath.Join(root, "force-app/main/default/objects", name+".object"), source)
			}
			var named strings.Builder
			named.WriteString("@IsTest private class ProjectionDefaultsProbe {")
			for _, row := range data.Rows {
				if row.ID == "CTRL_COMPILE" {
					continue
				}
				body := projectionDefaultsObservationBody(row)
				fmt.Fprintf(&named, "@IsTest static void observe_%s(){%s}", row.ID, body)
				if row.AssertionBody != "" {
					fmt.Fprintf(&named, "@IsTest static void assert_%s(){%s}", row.ID, row.AssertionBody)
				}
			}
			named.WriteString("}")
			path := filepath.Join(root, "force-app/main/default/classes/ProjectionDefaultsProbe.cls")
			writeFile(t, path, named.String())
			writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			fixture, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			metadata, err := gladeschema.LoadProject(fixture)
			if err != nil {
				t.Fatal(err)
			}
			index := typesys.Build(fixture, metadata)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("captured source rejected: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			// The native source API varies independently of the org's REST
			// serialization version, which is 68 in every observed snapshot.
			org.APIVersion = "68.0"
			base := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org, LinkProject: true})
			cases := Discover(index, Options{SelectedClasses: []string{"ProjectionDefaultsProbe"}})
			methods, methodErrors := compileTestMethods(cases)
			programs, programErrors := compileTestInvokePrograms(cases)
			namedPrograms := projectionDefaultsNamedPrograms{Methods: map[string]vm.Method{}, Programs: map[string]ir.Program{}}
			for _, c := range cases {
				key := testCaseKey(c)
				if methodErrors[key] != nil || programErrors[key] != nil {
					t.Fatalf("named source compilation: %v %v", methodErrors[key], programErrors[key])
				}
				namedPrograms.Methods[c.MethodName] = methods[key]
				namedPrograms.Programs[c.MethodName] = programs[key]
			}
			if len(cases) != 112 {
				t.Fatalf("named source methods=%d; want112", len(cases))
			}
			for _, row := range data.Rows {
				t.Run(row.ID, func(t *testing.T) {
					for _, route := range data.Routes {
						t.Run(route, func(t *testing.T) {
							cell := projectionDefaultsFindCell(t, row, api, route)
							var native struct {
								Outcome, ExceptionType, ExceptionMessage string
								Value                                    map[string]any
							}
							if err := json.Unmarshal(cell.Native, &native); err != nil {
								t.Fatal(err)
							}
							kind := "exact"
							if len(cell.CarriedFields) > 0 {
								kind = "partial"
								t.Logf("field carries=%v; native=%s", cell.CarriedFields, cell.Native)
							}
							counts.TrackKind(t, route, kind, 1)
							if native.Outcome == "compile_error" {
								checkProjectionDefaultsCompileControl(t, index, fixture, metadata, row, api, route)
								return
							}
							machine := base.newMachine()
							if route == "istest" {
								machine.EnableTestContext()
							}
							body := projectionDefaultsObservationBody(row)
							program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
							if err != nil {
								t.Fatal(err)
							}
							if route == "anonymous" {
								if a := sema.AnalyzeAnonymous(index, body, api); a.HasErrors() {
									t.Fatalf("native anonymous accepted: %v", a.Diagnostics)
								}
							}
							var execErr error
							if route == "istest" {
								method := "observe_" + row.ID
								if err := machine.RegisterMethod(namedPrograms.Methods[method]); err != nil {
									t.Fatal(err)
								}
								_, execErr = machine.ExecuteInClass(namedPrograms.Programs[method], "ProjectionDefaultsProbe")
							} else {
								_, execErr = machine.Execute(program)
							}
							var runtimeErr *vm.RuntimeError
							if !errors.As(execErr, &runtimeErr) {
								t.Fatalf("terminal native observation missing: %v", execErr)
							}
							if strings.HasPrefix(native.Outcome, "uncaught_") {
								name := runtimeErr.Type
								if !strings.HasPrefix(name, "System.") {
									name = "System." + name
								}
								if name != native.ExceptionType || name+": "+runtimeErr.ExceptionMessage() != native.ExceptionMessage {
									t.Fatalf("native uncaught=%s; local=%v", cell.Native, execErr)
								}
								return
							}
							const marker = "PROJECTION_OBSERVATION|"
							_, observed, found := strings.Cut(runtimeErr.ExceptionMessage(), marker)
							if runtimeErr.Type != "System.AssertException" || !found {
								t.Fatalf("snapshot marker missing: %v", execErr)
							}
							var actual map[string]any
							if err := json.Unmarshal([]byte(observed), &actual); err != nil {
								t.Fatal(err)
							}
							if row.AssertionBody != "" {
								actual["assertEquals"] = projectionDefaultsNativeAssertion(t, base, namedPrograms, row, api, route)
							}
							want := projectionDefaultsComparable(t, row, cell, native.Value)
							got := projectionDefaultsComparable(t, row, cell, actual)
							if !reflect.DeepEqual(got, want) {
								t.Errorf("native=%s\nlocal=%s", projectionDefaultsJSON(want), projectionDefaultsJSON(got))
							}
						})
					}
				})
			}
		})
	}
}

func projectionDefaultsObservationBody(row projectionDefaultsRow) string {
	if row.ID == "CTRL_UNCAUGHT" || row.ID == "CTRL_COMPILE" {
		return row.Body
	}
	return row.Body + `System.assert(false,'PROJECTION_OBSERVATION|'+JSON.serialize(result));`
}

func projectionDefaultsNativeAssertion(t *testing.T, base *conformanceRunner, named projectionDefaultsNamedPrograms, row projectionDefaultsRow, api, route string) map[string]any {
	t.Helper()
	machine := base.newMachine()
	if route == "istest" {
		machine.EnableTestContext()
	}
	body := row.AssertionBody
	program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
	if err != nil {
		t.Fatal(err)
	}
	if route == "istest" {
		method := "assert_" + row.ID
		if err := machine.RegisterMethod(named.Methods[method]); err != nil {
			t.Fatal(err)
		}
		_, err = machine.ExecuteInClass(named.Programs[method], "ProjectionDefaultsProbe")
	} else {
		_, err = machine.Execute(program)
	}
	if err == nil {
		return map[string]any{"pass": true}
	}
	var runtimeErr *vm.RuntimeError
	if !errors.As(err, &runtimeErr) || runtimeErr.Type != "System.AssertException" {
		t.Fatalf("unexpected equality assertion error: %v", err)
	}
	return map[string]any{"pass": false, "type": runtimeErr.Type, "message": runtimeErr.ExceptionMessage()}
}

var projectionDefaultsPrincipal = regexp.MustCompile(`005[0-9A-Za-z]{15}`)

func projectionDefaultsComparable(t *testing.T, row projectionDefaultsRow, cell projectionDefaultsCell, value map[string]any) map[string]any {
	t.Helper()
	// Normalize only the seeded execution principal; native seed and parent
	// identifiers were normalized by the synthetic capture source itself.
	raw := projectionDefaultsPrincipal.ReplaceAllString(projectionDefaultsJSON(value), "<current-user-id>")
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatal(err)
	}
	for key, item := range out {
		if text, ok := item.(string); ok && strings.Contains(key, "JSON") {
			var decoded any
			if err := json.Unmarshal([]byte(text), &decoded); err != nil {
				t.Fatalf("native JSON snapshot malformed: %v", err)
			}
			out[key] = decoded
		}
	}
	// Carry only specifically identified semantic snapshot fields. The full
	// original native JSON text remains in the cell and the TSV.
	for path := range cell.CarriedFields {
		parts := strings.Split(path, ".")
		if len(parts) == 2 && strings.Contains(parts[0], "JSON") && parts[1] != "text" && parts[1] != "systemDefaults" && parts[1] != "emptyTextDefault" {
			if snapshot, ok := out[parts[0]].(map[string]any); ok {
				delete(snapshot, parts[1])
			}
		}
	}
	if row.Group == "B" {
		equality, ok := out["equality"].(map[string]any)
		if !ok {
			t.Fatalf("complete native equality missing: %s", raw)
		}
		if row.ID != "B07_schema_deep" {
			equality["hashMatch"] = equality["aHash"] == equality["bHash"]
		}
		delete(equality, "aHash")
		delete(equality, "bHash")
		for _, key := range []string{"beforeaMap", "beforebMap", "afteraMap", "afterbMap", "beforeaJSON", "beforebJSON", "afteraJSON", "afterbJSON"} {
			if snapshot, ok := out[key].(map[string]any); ok && cell.CarriedFields[key+".systemDefaults"] != "" {
				delete(snapshot, "OwnerId")
				delete(snapshot, "IsDeleted")
			}
			if snapshot, ok := out[key].(map[string]any); ok && cell.CarriedFields[key+".emptyTextDefault"] != "" {
				delete(snapshot, "BlankCaption__c")
			}
		}
		if assertion, ok := out["assertEquals"].(map[string]any); ok {
			delete(assertion, "message")
		}
	}
	return out
}

func projectionDefaultsJSON(value any) string { raw, _ := json.Marshal(value); return string(raw) }

func checkProjectionDefaultsCompileControl(t *testing.T, index typesys.Index, fixture project.Project, metadata gladeschema.Schema, row projectionDefaultsRow, api, route string) {
	t.Helper()
	analysis := sema.AnalyzeAnonymous(index, row.Body, api)
	if route == "istest" {
		path := filepath.Join(fixture.Root, "force-app/main/default/classes/ProjectionCompileControl.cls")
		writeFile(t, path, "@IsTest private class ProjectionCompileControl {@IsTest static void observe(){"+row.Body+"}}")
		writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
		fixture.ApexFiles = append(append([]string{}, fixture.ApexFiles...), path)
		analysis = sema.Analyze(typesys.Build(fixture, metadata))
	}
	found := false
	for _, d := range analysis.Diagnostics {
		if d.Severity == diagnostic.Error && strings.Contains(d.Message+d.NativeMessage, "missingProbeIdentifier") {
			found = true
		}
	}
	if !found {
		t.Fatalf("native missing-variable rejection not observed: %v", analysis.Diagnostics)
	}
}

func projectionDefaultsFindCell(t *testing.T, row projectionDefaultsRow, api, route string) projectionDefaultsCell {
	t.Helper()
	for _, cell := range row.Cells {
		if cell.API == api && cell.Route == route {
			return cell
		}
	}
	t.Fatalf("missing independent native cell %s/%s/%s", row.ID, api, route)
	return projectionDefaultsCell{}
}

func loadProjectionDefaultsData(t *testing.T) (projectionDefaultsData, map[string]string) {
	t.Helper()
	var data projectionDefaultsData
	raw, err := os.ReadFile(filepath.Join(projectionDefaultsFixture, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data.ShapeCount != 101 || data.NativeObservationCount != 606 || len(data.Rows) != 101 || strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" || strings.Join(data.Routes, ",") != "anonymous,istest" {
		t.Fatal("incomplete native matrix")
	}
	var sources struct {
		Objects map[string]string
		Rows    []projectionDefaultsRow
	}
	raw, err = os.ReadFile(filepath.Join(projectionDefaultsFixture, "sources.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &sources); err != nil {
		t.Fatal(err)
	}
	if len(sources.Objects) != 2 || len(sources.Rows) != 101 {
		t.Fatal("incomplete source manifest")
	}
	for name, source := range sources.Objects {
		raw, err := os.ReadFile(filepath.Join(projectionDefaultsFixture, "setup", name+".object"))
		if err != nil || string(raw) != source+"\n" {
			t.Fatalf("metadata detached from capture: %s", name)
		}
	}
	tables := map[string]map[string]json.RawMessage{}
	for _, api := range data.APIVersions {
		for _, route := range data.Routes {
			path := filepath.Join(projectionDefaultsFixture, "native", "api"+strings.TrimSuffix(api, ".0"), route, "org.tsv")
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
			if len(lines) != 102 || lines[0] != "id\tgroup\troute\tstatus\tvalue" {
				t.Fatalf("incomplete native table %s", path)
			}
			table := map[string]json.RawMessage{}
			for _, line := range lines[1:] {
				fields := strings.SplitN(line, "\t", 5)
				if len(fields) != 5 || table[fields[0]] != nil || fields[2] != route || !json.Valid([]byte(fields[4])) {
					t.Fatal("invalid native table cell")
				}
				table[fields[0]] = json.RawMessage(fields[4])
			}
			tables[api+"/"+route] = table
		}
	}
	seen := map[string]bool{}
	for i, row := range data.Rows {
		if row.ID == "" || seen[row.ID] || len(row.Cells) != 6 {
			t.Fatal("invalid native row")
		}
		seen[row.ID] = true
		source := sources.Rows[i]
		if source.ID != row.ID || source.Body != row.Body || source.AssertionBody != row.AssertionBody || source.Group != row.Group {
			t.Fatalf("row detached from source: %s", row.ID)
		}
		keys := map[string]bool{}
		for _, cell := range row.Cells {
			key := cell.API + "/" + cell.Route
			if keys[key] || !scaleZeroNumberJSONEqual(tables[key][row.ID], cell.Native) {
				t.Fatalf("cell detached from native table: %s/%s", row.ID, key)
			}
			keys[key] = true
			for field, reason := range cell.CarriedFields {
				if field == "" || reason == "" {
					t.Fatal("unexplained carry")
				}
			}
		}
	}
	return data, sources.Objects
}
