package apextest

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

const numberTokenFixture = "testdata/conformance/number_token_provenance"

type numberTokenCell struct {
	API, Route    string
	Native        json.RawMessage
	CarriedFields map[string]string
}

type numberTokenRow struct {
	ID, Group, Anonymous string
	Named                map[string]string
	Cells                []numberTokenCell
}

type numberTokenData struct {
	APIVersions, Routes                []string
	ShapeCount, NativeObservationCount int
	Files                              []string
	Rows                               []numberTokenRow
}

// Replay all 49 independently observed rows on both routes at API 62/67/68.
// Named observations are separate native segments, not inferred from the
// anonymous transport. JSON ordering is framing; text, casts, previous values,
// exact runtime errors and typed-versus-boxed observations remain exact.
func TestNumberTokenProvenanceOrgConformance(t *testing.T) {
	data := loadNumberTokenData(t)
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "NumberTokenProvenance", api)
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			for _, relative := range []string{"ProbeGauge__c/ProbeGauge__c.object-meta.xml", "ProbeGauge__c/fields/TickValue__c.field-meta.xml", "NumericCaptureBridge.cls", "NumericObservationBridge.cls"} {
				raw, err := os.ReadFile(filepath.Join(numberTokenFixture, "setup", relative))
				if err != nil {
					t.Fatal(err)
				}
				folder := "objects"
				if strings.HasSuffix(relative, ".cls") {
					folder = "classes"
				}
				path := filepath.Join(root, "force-app/main/default", folder, relative)
				writeFile(t, path, string(raw))
				if folder == "classes" {
					writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
				}
			}
			fixture, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			metadata, err := gladeschema.LoadProject(fixture)
			if err != nil {
				t.Fatal(err)
			}
			index := typesys.Build(fixture, metadata)
			if index.HasErrors() || sema.Analyze(index).HasErrors() {
				t.Fatal("captured helpers rejected")
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			base := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org, LinkProject: true})
			for _, row := range data.Rows {
				t.Run(row.ID, func(t *testing.T) {
					for _, route := range data.Routes {
						t.Run(route, func(t *testing.T) {
							cell := numberTokenFindCell(t, row, api, route)
							var native struct {
								Outcome, Problem, ExceptionType, ExceptionMessage string
								Value                                             json.RawMessage
								Segments                                          map[string]json.RawMessage
								Diagnostics                                       []struct{ Problem string }
							}
							if err := json.Unmarshal(cell.Native, &native); err != nil {
								t.Fatal(err)
							}
							kind := "exact"
							if native.Outcome == "compile_error" {
								kind = "category"
								t.Logf("native wrapper locations retained with reasons %v; native=%s", cell.CarriedFields, cell.Native)
							}
							counts.TrackKind(t, route, kind, 1)
							checkCompile := func(diagnostics []diagnostic.Diagnostic, compileErr error) bool {
								if native.Outcome != "compile_error" {
									for _, d := range diagnostics {
										if d.Severity == diagnostic.Error {
											t.Fatalf("native compilation accepted; Glade: %v", diagnostics)
										}
									}
									if compileErr != nil {
										t.Fatal(compileErr)
									}
									return true
								}
								problem := native.Problem
								if problem == "" && len(native.Diagnostics) == 1 {
									problem = native.Diagnostics[0].Problem
								}
								if problem == "" {
									t.Fatal("empty native compile diagnostic")
								}
								actual := ""
								for _, d := range diagnostics {
									if d.Severity == diagnostic.Error {
										actual = d.NativeMessage
										if actual == "" {
											actual = d.Message
										}
										break
									}
								}
								if actual == "" && compileErr != nil {
									actual = compileErr.Error()
								}
								if actual != problem {
									t.Fatalf("native compile <%s>; Glade <%s>", problem, actual)
								}
								return false
							}
							checkExecution := func(execErr error, marker string, want json.RawMessage) {
								var runtimeErr *vm.RuntimeError
								if !errors.As(execErr, &runtimeErr) {
									t.Fatalf("native terminal exception missing: %v", execErr)
								}
								if strings.HasPrefix(native.Outcome, "uncaught_") {
									name := runtimeErr.Type
									if !strings.HasPrefix(name, "System.") {
										name = "System." + name
									}
									if name != native.ExceptionType || name+": "+runtimeErr.ExceptionMessage() != native.ExceptionMessage {
										t.Fatalf("native uncaught %s; Glade %v", cell.Native, execErr)
									}
									return
								}
								if runtimeErr.Type != "System.AssertException" {
									t.Fatalf("terminal assertion missing: %v", execErr)
								}
								_, observed, found := strings.Cut(runtimeErr.ExceptionMessage(), marker)
								if !found || strings.Count(runtimeErr.ExceptionMessage(), marker) != 1 {
									t.Fatalf("native marker %s missing: %v", marker, execErr)
								}
								var value json.RawMessage
								if err := json.Unmarshal([]byte(observed), &value); err != nil {
									t.Fatal(err)
								}
								if !scaleZeroNumberJSONEqual(value, want) {
									t.Fatalf("native %s; Glade %s", want, observed)
								}
							}
							if route == "anonymous" {
								analysis := sema.AnalyzeAnonymous(index, row.Anonymous, api)
								program, compileErr := vm.CompileAnonymousWithOptions(row.Anonymous, vm.CompileOptions{APIVersion: api})
								if checkCompile(analysis.Diagnostics, compileErr) {
									_, execErr := base.execute(program)
									checkExecution(execErr, "C126_RESULT|"+row.ID+"|", native.Value)
								}
								return
							}
							segments := make([]string, 0, len(row.Named))
							for segment := range row.Named {
								segments = append(segments, segment)
							}
							sort.Strings(segments)
							var source strings.Builder
							source.WriteString("@IsTest private class NumericRowProbe {")
							for _, segment := range segments {
								fmt.Fprintf(&source, "@IsTest static void observe%s(){%s}", strings.ToUpper(segment[:1])+segment[1:], row.Named[segment])
							}
							source.WriteString("}")
							path := filepath.Join(root, "force-app/main/default/classes/NumericRowProbe.cls")
							writeFile(t, path, source.String())
							writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
							p := fixture
							p.ApexFiles = append(append([]string{}, fixture.ApexFiles...), path)
							namedIndex := typesys.Build(p, metadata)
							diagnostics := append(append([]diagnostic.Diagnostic{}, namedIndex.Diagnostics...), sema.Analyze(namedIndex).Diagnostics...)
							if !checkCompile(diagnostics, nil) {
								return
							}
							cases := Discover(namedIndex, Options{SelectedClasses: []string{"NumericRowProbe"}})
							methods, methodErrors := compileTestMethods(cases)
							programs, programErrors := compileTestInvokePrograms(cases)
							if len(cases) != len(segments) {
								t.Fatalf("native %d segments; Glade %d", len(segments), len(cases))
							}
							named := newConformanceRunner(t, namedIndex, conformanceRunnerOptions{Base: base, LinkProject: true, Org: &org})
							for _, c := range cases {
								segment := strings.ToLower(strings.TrimPrefix(c.MethodName, "observe"))
								key := testCaseKey(c)
								if methodErrors[key] != nil || programErrors[key] != nil {
									t.Fatalf("named compilation: %v %v", methodErrors[key], programErrors[key])
								}
								machine := named.newMachine()
								machine.EnableTestContext()
								if err := machine.RegisterMethod(methods[key]); err != nil {
									t.Fatal(err)
								}
								_, execErr := machine.ExecuteInClass(programs[key], c.ClassName)
								marker := "C126_RESULT|" + row.ID + "_" + segment + "|"
								want := native.Segments[segment]
								if row.Group == "CONTROL" {
									marker = "C126_RESULT|" + row.ID + "|"
									want = native.Value
								}
								checkExecution(execErr, marker, want)
							}
						})
					}
				})
			}
		})
	}
}

func numberTokenFindCell(t *testing.T, row numberTokenRow, api, route string) numberTokenCell {
	t.Helper()
	for _, cell := range row.Cells {
		if cell.API == api && cell.Route == route {
			return cell
		}
	}
	t.Fatalf("missing native cell %s/%s/%s", row.ID, api, route)
	return numberTokenCell{}
}

func loadNumberTokenData(t *testing.T) numberTokenData {
	t.Helper()
	var data numberTokenData
	raw, err := os.ReadFile(filepath.Join(numberTokenFixture, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data.ShapeCount != 49 || data.NativeObservationCount != 294 || len(data.Rows) != 49 || strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" || strings.Join(data.Routes, ",") != "anonymous,istest" {
		t.Fatal("incomplete native matrix")
	}
	seenFiles := map[string]bool{}
	tables := map[string]map[string]json.RawMessage{}
	var sources struct {
		Rows, Controls []numberTokenRow
		Helpers        map[string]string
	}
	for _, path := range data.Files {
		if seenFiles[path] || filepath.IsAbs(path) || strings.Contains(path, "..") {
			t.Fatal("invalid/repeated native file")
		}
		seenFiles[path] = true
		raw, err := os.ReadFile(filepath.Join(numberTokenFixture, path))
		if err != nil {
			t.Fatal(err)
		}
		if path == "sources.json" {
			if err := json.Unmarshal(raw, &sources); err != nil {
				t.Fatal(err)
			}
		}
		if strings.HasSuffix(path, "/RUN-FACTS.txt") {
			parts := strings.Split(path, "/")
			if len(parts) != 4 {
				t.Fatal("invalid native run path")
			}
			api := strings.TrimPrefix(parts[1], "api") + ".0"
			for _, line := range []string{"api=" + api, "route=" + parts[2], "rows_processed=49", "data_rows=45", "control_rows=4", "native_complete=true"} {
				if !strings.Contains("\n"+string(raw), "\n"+line+"\n") {
					t.Fatalf("incomplete native run: %s", path)
				}
			}
		}
		if strings.HasSuffix(path, "/org.tsv") {
			rows := map[string]json.RawMessage{}
			lines := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
			if len(lines) != 50 || lines[0] != "id\tgroup\troute\tstatus\tvalue" {
				t.Fatal("incomplete native TSV")
			}
			for _, line := range lines[1:] {
				fields := strings.SplitN(line, "\t", 5)
				if len(fields) != 5 || rows[fields[0]] != nil || !json.Valid([]byte(fields[4])) {
					t.Fatal("invalid/repeated native TSV cell")
				}
				rows[fields[0]] = json.RawMessage(fields[4])
			}
			tables[path] = rows
		}
	}
	if len(tables) != 6 || !seenFiles["sources.json"] {
		t.Fatal("missing native tables or source manifest")
	}
	capturedSources := map[string]numberTokenRow{}
	for _, row := range append(sources.Rows, sources.Controls...) {
		if _, duplicate := capturedSources[row.ID]; duplicate {
			t.Fatal("duplicate captured source")
		}
		capturedSources[row.ID] = row
	}
	if len(capturedSources) != data.ShapeCount {
		t.Fatal("incomplete captured source manifest")
	}
	for helper, source := range sources.Helpers {
		path := "setup/" + helper + ".cls"
		raw, err := os.ReadFile(filepath.Join(numberTokenFixture, path))
		if err != nil || !seenFiles[path] || string(raw) != source {
			t.Fatalf("helper detached from captured source: %s", helper)
		}
	}
	seenRows := map[string]bool{}
	for _, row := range data.Rows {
		if row.ID == "" || seenRows[row.ID] || len(row.Cells) != 6 || len(row.Named) == 0 {
			t.Fatal("invalid native row")
		}
		seenRows[row.ID] = true
		captured, exists := capturedSources[row.ID]
		if !exists || captured.Anonymous != row.Anonymous || captured.Group != row.Group || len(captured.Named) != len(row.Named) {
			t.Fatalf("row detached from captured source: %s", row.ID)
		}
		for segment, source := range row.Named {
			if captured.Named[segment] != source {
				t.Fatalf("named segment detached from captured source: %s/%s", row.ID, segment)
			}
		}
		seenCells := map[string]bool{}
		for _, cell := range row.Cells {
			if (row.ID == "CTRL_COMPILE") != (len(cell.CarriedFields) == 1) {
				t.Fatal("unexpected or missing diagnostic-location carry")
			}
			for field, reason := range cell.CarriedFields {
				if field != "diagnosticLocations" || reason == "" {
					t.Fatal("unexplained native carry")
				}
			}
			key := cell.API + "/" + cell.Route
			if seenCells[key] {
				t.Fatal("duplicate native cell")
			}
			seenCells[key] = true
			path := "native/api" + strings.TrimSuffix(cell.API, ".0") + "/" + cell.Route + "/org.tsv"
			if native, exists := tables[path][row.ID]; !exists || !scaleZeroNumberJSONEqual(native, cell.Native) {
				t.Fatalf("cell detached from native TSV: %s/%s", row.ID, key)
			}
		}
	}
	return data
}
