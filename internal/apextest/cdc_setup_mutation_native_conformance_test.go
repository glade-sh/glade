package apextest

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/vm"
)

const cdcSetupMutationFixture = "testdata/conformance/cdc_g1_g3"

type cdcSetupMutationCase struct {
	ID, Class, Method, Gap, Description string
}

type cdcSetupMutationCapture struct {
	APIs         []string                     `json:"apis"`
	Routes       []string                     `json:"routes"`
	Cases        []cdcSetupMutationCase       `json:"cases"`
	SourceSHA256 map[string]map[string]string `json:"source_sha256"`
}

func readCdcSetupMutationCapture(t *testing.T) (cdcSetupMutationCapture, map[string]map[string]any) {
	t.Helper()
	var capture cdcSetupMutationCapture
	raw, err := os.ReadFile(filepath.Join(cdcSetupMutationFixture, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &capture); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(capture.APIs, []string{"62.0", "67.0"}) || !reflect.DeepEqual(capture.Routes, []string{"anonymous", "istest", "deploy_istest"}) || len(capture.Cases) != 8 {
		t.Fatal("incomplete native case matrix")
	}
	ids := map[string]bool{}
	for _, row := range capture.Cases {
		if row.ID == "" || row.Class == "" || row.Method != row.ID || row.Gap == "" || row.Description == "" || ids[row.ID] {
			t.Fatalf("invalid native case: %+v", row)
		}
		ids[row.ID] = true
	}
	for _, api := range capture.APIs {
		manifest := capture.SourceSHA256[api]
		count := 0
		for _, dir := range []string{"project", "anonymous.apex"} {
			err := filepath.WalkDir(filepath.Join(cdcSetupMutationFixture, dir), func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				rel, err := filepath.Rel(cdcSetupMutationFixture, path)
				if err != nil {
					return err
				}
				digest := fmt.Sprintf("%x", sha256.Sum256([]byte(strings.ReplaceAll(string(body), "62.0", api))))
				if manifest[filepath.ToSlash(rel)] != digest {
					t.Errorf("native source changed at API %s: %s", api, rel)
				}
				count++
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		}
		if len(manifest) != count {
			t.Fatal("incomplete native source manifest")
		}
	}
	raw, err = os.ReadFile(filepath.Join(cdcSetupMutationFixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if lines[0] != "api\troute\tid\tvalue" {
		t.Fatal("invalid native row header")
	}
	rows := map[string]map[string]any{}
	for _, line := range lines[1:] {
		cells := strings.SplitN(line, "\t", 4)
		if len(cells) != 4 || !ids[cells[2]] {
			t.Fatalf("invalid native row: %s", line)
		}
		key := strings.Join(cells[:3], "/")
		var value map[string]any
		if rows[key] != nil || json.Unmarshal([]byte(cells[3]), &value) != nil || value == nil {
			t.Fatalf("invalid native observation: %s", line)
		}
		rows[key] = value
	}
	for _, api := range capture.APIs {
		for _, route := range capture.Routes {
			for _, row := range capture.Cases {
				if rows[api+"/"+route+"/"+row.ID] == nil {
					t.Fatalf("missing native row: %s/%s/%s", api, route, row.ID)
				}
			}
		}
	}
	if len(rows) != len(capture.APIs)*len(capture.Routes)*len(capture.Cases) {
		t.Fatal("unexpected native rows")
	}
	return capture, rows
}

func assertCdcSetupMutationPayload(t *testing.T, id, message string, want map[string]any) {
	t.Helper()
	_, payload, ok := strings.Cut(message, "CDC_GAP_OBS|"+id+"|")
	var got map[string]any
	if !ok || json.Unmarshal([]byte(payload), &got) != nil {
		t.Fatalf("missing observation transport: %s", message)
	}
	if !reflect.DeepEqual(got, want) {
		native, _ := json.Marshal(want)
		t.Fatalf("native=%s; Glade=%s", native, payload)
	}
}

func TestCdcSetupAndMutationNativeRows(t *testing.T) {
	readCdcSetupMutationCapture(t)
}

func TestCdcSetupAndMutationNativeConformance(t *testing.T) {
	capture, native := readCdcSetupMutationCapture(t)
	for _, api := range capture.APIs {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			sourceRoot := filepath.Join(cdcSetupMutationFixture, "project")
			if err := filepath.WalkDir(sourceRoot, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				rel, err := filepath.Rel(sourceRoot, path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, rel), strings.ReplaceAll(string(body), "62.0", api))
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			t.Run("project_semantics", func(t *testing.T) {
				analysis := sema.Analyze(index)
				if index.HasErrors() || analysis.HasErrors() {
					t.Fatalf("native source compiled; Glade diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
				}
			})
			t.Run("anonymous", func(t *testing.T) {
				org := orgFromIndex(index)
				org.APIVersion = api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				template, err := os.ReadFile(filepath.Join(cdcSetupMutationFixture, "anonymous.apex"))
				if err != nil {
					t.Fatal(err)
				}
				for _, row := range capture.Cases {
					t.Run(row.ID, func(t *testing.T) {
						source := strings.ReplaceAll(string(template), "CASE_ID", row.ID)
						program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						_, err = runner.newMachine().Execute(program)
						var fault *vm.RuntimeError
						if !errors.As(err, &fault) || !strings.HasSuffix(fault.Type, "AssertException") {
							t.Fatalf("native observation completed; Glade execution: %v", err)
						}
						assertCdcSetupMutationPayload(t, row.ID, fault.ExceptionMessage(), native[api+"/anonymous/"+row.ID])
					})
				}
			})
			classes := []string{"CdcSetupTwinA", "CdcSetupTwinB", "CdcSetupSelected", "CdcMutationObservationTest"}
			for _, mode := range []string{"serial_project_istest", "cached_parallel_project_istest"} {
				t.Run(mode, func(t *testing.T) {
					// Component APIs come from the captured metadata. These probes
					// do not issue REST requests; use the supported runtime context.
					opts := Options{SelectedClasses: classes, RuntimeRESTAPIVersion: "67.0", NoDiskCache: true, Parallelism: 1}
					if mode == "cached_parallel_project_istest" {
						if _, ok := validMemoryRuntimeExecution(runtimeKey(index)); !ok {
							t.Fatal("serial run did not warm the project runtime cache")
						}
						opts.Parallelism, opts.ParallelMethods = 2, true
					}
					report := Run(index, opts)
					results := map[string]testreport.Case{}
					for _, suite := range report.Suites {
						for _, result := range suite.Cases {
							results[result.ClassName+"."+result.MethodName] = result
						}
					}
					if len(results) != len(capture.Cases) {
						t.Errorf("discovered %d methods, want %d; exclusions=%v", len(results), len(capture.Cases), report.ExcludedClasses)
					}
					for _, route := range []string{"istest", "deploy_istest"} {
						for _, row := range capture.Cases {
							t.Run(route+"/"+row.ID, func(t *testing.T) {
								result, ok := results[row.Class+"."+row.Method]
								if !ok || result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
									t.Fatalf("native observation completed; Glade status=%s; problem=%+v", result.Status, result.Problem)
								}
								assertCdcSetupMutationPayload(t, row.ID, result.Problem.Message, native[api+"/"+route+"/"+row.ID])
							})
						}
					}
				})
			}
		})
	}
}
