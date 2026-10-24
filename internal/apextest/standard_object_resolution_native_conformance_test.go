package apextest

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

func TestStandardObjectResolutionNativeConformanceRoutes(t *testing.T) {
	const fixture = "../../testdata/conformance/standard_object_resolution"
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			project := filepath.Join(fixture, "project")
			if err := filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				relative, err := filepath.Rel(project, path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, relative), strings.ReplaceAll(string(body), "67.0", api))
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("project compilation: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			storage.DeleteObject(&org, "Asset")
			org.ClearRuntimeSchemaStamp()
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org, LinkProject: true})
			cases := Discover(index, Options{SelectedClasses: []string{"StandardResolutionTest"}})
			methods, methodErrors := compileTestMethods(cases)
			programs, programErrors := compileTestInvokePrograms(cases)
			if len(cases) != 12 || len(methodErrors) != 0 || len(programErrors) != 0 {
				t.Fatalf("named compilation: cases=%d methods=%v programs=%v", len(cases), methodErrors, programErrors)
			}
			native := filepath.Join(fixture, "native", "api"+api[:2])
			receipt, err := os.ReadFile(filepath.Join(native, "RUN-FACTS.txt"))
			if err != nil || !strings.Contains(string(receipt), "source_api="+api+"\n") || !strings.Contains(string(receipt), "native_complete=true\n") {
				t.Fatalf("incomplete native capture: %v", err)
			}
			table, err := os.ReadFile(filepath.Join(native, "org.tsv"))
			if err != nil {
				t.Fatal(err)
			}
			rows := strings.Split(strings.TrimSpace(string(table)), "\n")
			if len(rows) != 25 || rows[0] != "source_api\tentry\troute\tobservation" {
				t.Fatal("incomplete native matrix")
			}
			seen := map[string]bool{}
			for _, row := range rows[1:] {
				columns := strings.SplitN(row, "\t", 4)
				if len(columns) != 4 || columns[0] != api {
					t.Fatalf("invalid native row: %s", row)
				}
				entry, route := columns[1], columns[2]
				key := entry + "/" + route
				if seen[key] {
					t.Fatal("duplicate native row: " + key)
				}
				seen[key] = true
				for _, loaded := range []bool{false, true} {
					state := "cold"
					if loaded {
						state = "loaded"
					}
					t.Run(key+"/"+state, func(t *testing.T) {
						machine := runner.newMachine()
						if loaded {
							// A separate org generation is used for the loaded control.
							control := runner.newOrg()
							storage.EnsureStandardObject(&control, "Asset")
							machine.SetOrg(&control)
						}
						stamp, generation, schemaGeneration := machine.Org.RuntimeSchemaStamp, machine.Org.RuntimeGenerationID(), machine.Org.SchemaGeneration()
						var execErr error
						if entry == "istest" {
							machine.EnableTestContext()
							found := false
							for _, c := range cases {
								if c.MethodName == route {
									found = true
									k := testCaseKey(c)
									if err := machine.RegisterMethod(methods[k]); err != nil {
										t.Fatal(err)
									}
									_, execErr = machine.ExecuteInClass(programs[k], c.ClassName)
								}
							}
							if !found {
								t.Fatal("missing test method " + route)
							}
						} else if entry == "anonymous" {
							body := "System.assert(false,'RESOLUTION|'+StandardResolutionProbe.observe('" + route + "'));"
							if analysis := sema.AnalyzeAnonymous(index, body, api); analysis.HasErrors() {
								t.Fatal(analysis.Diagnostics)
							}
							program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
							if err != nil {
								t.Fatal(err)
							}
							_, execErr = machine.Execute(program)
						} else {
							t.Fatal("unknown entry " + entry)
						}
						var fault *vm.RuntimeError
						if !errors.As(execErr, &fault) || !strings.HasSuffix(fault.Type, "AssertException") {
							t.Fatalf("missing native observation: %v", execErr)
						}
						_, payload, found := strings.Cut(fault.ExceptionMessage(), "RESOLUTION|")
						var got, want any
						if !found || json.Unmarshal([]byte(payload), &got) != nil || json.Unmarshal([]byte(columns[3]), &want) != nil {
							t.Fatalf("invalid observation: %s", fault.ExceptionMessage())
						}
						if !reflect.DeepEqual(got, want) {
							t.Errorf("native=%s; Glade=%s", columns[3], payload)
						}
						if machine.Org.RuntimeSchemaStamp != stamp || machine.Org.RuntimeGenerationID() != generation || machine.Org.SchemaGeneration() != schemaGeneration {
							t.Error("project route changed schema generation")
						}
						if !loaded {
							if _, present := machine.Org.Objects["Asset"]; present {
								t.Error("project route installed a cold standard object")
							}
						}
					})
				}
			}
		})
	}
}
