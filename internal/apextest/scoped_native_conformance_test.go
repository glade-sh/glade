package apextest

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

type scopedNativeCell struct {
	API, Route, Status, NativeSHA256 string
	Native                           map[string]any
}

type scopedNativeRow struct {
	ID, Source, Anonymous, Named string
	SourceSHA256                 map[string]string
	Cells                        []scopedNativeCell
}

func TestCollectionTraversalNativeConformance(t *testing.T) {
	runScopedNativeConformance(t, "collection_traversal_native.json", "CollectionTraversalObservation", 32, 192, 18)
}

// Replay observed values or uncaught type/message. No stage or final-list
// readback is inferred from an uncaught cell. Unobserved refinements retain
// their native control evidence and are logged without skipping any cell.
func runScopedNativeConformance(t *testing.T, file, className string, shapes, cells, pending int) {
	var data struct {
		APIVersions, Routes                                []string
		ShapeCount, NativeCellCount, StillPendingCellCount int
		Helpers, HelperSHA256                              map[string]string
		Rows                                               []scopedNativeRow
		UnobservedShapes                                   []struct{ ID, Apex, NativeControl, Reason string }
	}
	raw, err := os.ReadFile("testdata/conformance/" + file)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" || strings.Join(data.Routes, ",") != "anonymous,istest" || data.ShapeCount != shapes || len(data.Rows) != shapes || data.NativeCellCount != cells || data.StillPendingCellCount != pending || len(data.UnobservedShapes)*6 != pending {
		t.Fatal("incomplete native matrix")
	}
	checkSource := func(source, want string) {
		t.Helper()
		if source == "" || fmt.Sprintf("%x", sha256.Sum256([]byte(source))) != want {
			t.Fatal("changed owned observation source")
		}
	}
	for name, source := range data.Helpers {
		checkSource(source, data.HelperSHA256[name])
	}
	seen := map[string]bool{}
	for _, row := range data.Rows {
		if row.ID == "" || seen[row.ID] || len(row.Cells) != 6 {
			t.Fatal("invalid/repeated native row")
		}
		seen[row.ID] = true
		checkSource(row.Anonymous, row.SourceSHA256["anonymous"])
		checkSource(row.Named, row.SourceSHA256["istest"])
		seenCells := map[string]bool{}
		for _, cell := range row.Cells {
			key := cell.API + "/" + cell.Route
			if seenCells[key] || len(cell.NativeSHA256) != 64 || len(cell.Native) == 0 || (cell.Status != "runtime" && cell.Status != "uncaught_"+cell.Route) {
				t.Fatal("invalid/repeated native cell")
			}
			seenCells[key] = true
		}
		for _, api := range data.APIVersions {
			for _, route := range data.Routes {
				if !seenCells[api+"/"+route] {
					t.Fatal("missing native cell")
				}
			}
		}
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCountLog(t, className, api)
			classes := map[string]string{}
			for name, source := range data.Helpers {
				classes[name] = source
			}
			var source strings.Builder
			source.WriteString("@IsTest private class " + className + "{")
			for _, row := range data.Rows {
				source.WriteString("@IsTest static void observe" + row.ID + "(){" + row.Named + "}")
			}
			source.WriteString("}")
			classes[className] = source.String()
			index := nativeBoundaryIndex(t, api, classes, nil)
			if index.HasErrors() || sema.Analyze(index).HasErrors() {
				t.Fatal("invalid owned native observation declarations")
			}
			cases := Discover(index, Options{SelectedClasses: []string{className}})
			if len(cases) != shapes {
				t.Fatal("incomplete named discovery")
			}
			methods, methodErrors := compileTestMethodsForIndex(&index, cases)
			programs, programErrors := compileTestInvokePrograms(cases)
			org := orgFromIndex(index)
			org.APIVersion = api
			base := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			for _, row := range data.Rows {
				for _, cell := range row.Cells {
					if cell.API != api {
						continue
					}
					route := cell.Route
					if route == "istest" {
						route = "isTest"
					}
					counts.Run(t, route, row.ID, row.ID+"/"+cell.Route, func(t *testing.T) {
						machine := base.newMachine()
						var result vm.Result
						var runErr error
						if cell.Route == "anonymous" {
							analysis := sema.AnalyzeAnonymous(index, row.Anonymous, api)
							program, compileErr := vm.CompileAnonymousWithOptions(row.Anonymous, vm.CompileOptions{APIVersion: api})
							if analysis.HasErrors() || compileErr != nil {
								t.Fatalf("native executes: diagnostics=%v compile=%v", analysis.Diagnostics, compileErr)
							}
							result, runErr = machine.Execute(program)
						} else {
							machine.EnableTestContext()
							found := false
							for _, test := range cases {
								if test.MethodName != "observe"+row.ID {
									continue
								}
								found = true
								key := testCaseKey(test)
								if methodErrors[key] != nil || programErrors[key] != nil {
									t.Fatalf("native executes: method=%v program=%v", methodErrors[key], programErrors[key])
								}
								if err := machine.RegisterMethod(methods[key]); err != nil {
									t.Fatal(err)
								}
								result, runErr = machine.ExecuteInClass(programs[key], className)
							}
							if !found {
								t.Fatal("missing named row")
							}
						}
						if cell.Status != "runtime" {
							var runtimeErr *vm.RuntimeError
							if !errors.As(runErr, &runtimeErr) || runtimeErr.Type != cell.Native["exceptionType"] || runtimeErr.Type+": "+runtimeErr.Message != cell.Native["exceptionMessage"] {
								t.Fatalf("native=%v; local error=%v", cell.Native, runErr)
							}
							return
						}
						if runErr != nil {
							t.Fatal(runErr)
						}
						markers := 0
						for _, line := range result.Debug {
							if !strings.HasPrefix(line, "SCOPED_NATIVE|") {
								continue
							}
							markers++
							var got map[string]any
							if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "SCOPED_NATIVE|")), &got); err != nil {
								t.Fatal(err)
							}
							if !reflect.DeepEqual(got, cell.Native["value"]) {
								t.Fatalf("native=%v; local=%v", cell.Native["value"], got)
							}
						}
						if markers != 1 {
							t.Fatalf("expected one marker, got %d", markers)
						}
					})
				}
			}
			for _, gap := range data.UnobservedShapes {
				if gap.Apex == "" || gap.Reason == "" || !seen[gap.NativeControl] {
					t.Fatal("unobserved shape lacks its source/reason/native control")
				}
				for _, row := range data.Rows {
					if row.ID != gap.NativeControl {
						continue
					}
					for _, cell := range row.Cells {
						if cell.API == api {
							native, _ := json.Marshal(cell.Native)
							t.Logf("still-pending %s/%s: %s; native control %s=%s", gap.ID, cell.Route, gap.Reason, row.ID, native)
						}
					}
				}
			}
		})
	}
}
