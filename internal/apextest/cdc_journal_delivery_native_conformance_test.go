package apextest

import (
	"context"
	"crypto/sha256"
	"encoding/json"
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

func TestChangeDataCaptureJournalNativeRows(t *testing.T) {
	cases := readPlatformTransactionCases(t)
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "istest"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				readPlatformTransactionNative(t, "cdc", api, route, cases)
			})
		}
	}
}

func TestChangeDataCaptureMutationNativeConformanceAPIRoutes(t *testing.T) {
	const directory = "testdata/conformance/cdc_journal_delivery"
	receipt, err := os.ReadFile(filepath.Join(directory, "RUN-FACTS.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"native_complete=true\n", "source_apis=62.0,67.0\n", "rows=24\n"} {
		if !strings.Contains(string(receipt), required) {
			t.Fatalf("incomplete native capture: %s", required)
		}
	}
	table, err := os.ReadFile(filepath.Join(directory, "cases.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(table)), "\n")[1:] {
		cells := strings.Split(line, "\t")
		if len(cells) != 3 || !strings.HasPrefix(cells[0], "cdc") || cells[1] != "istest" || cases[cells[0]] {
			t.Fatalf("invalid native case: %s", line)
		}
		cases[cells[0]] = true
	}
	raw, err := os.ReadFile(filepath.Join(directory, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 1+2*len(cases) || lines[0] != "source_api\troute\tcase\tobservation" {
		t.Fatal("incomplete native mutation rows")
	}
	seen := map[string]bool{}
	for _, line := range lines[1:] {
		cells := strings.SplitN(line, "\t", 4)
		if len(cells) != 4 || (cells[0] != "62.0" && cells[0] != "67.0") || cells[1] != "istest" || !cases[cells[2]] {
			t.Fatalf("invalid native mutation row: %s", line)
		}
		api, method := cells[0], cells[2]
		key := api + "/project_istest/" + method
		if seen[key] {
			t.Fatalf("duplicate native row: %s", key)
		}
		seen[key] = true
		t.Run(key, func(t *testing.T) {
			var want map[string][]string
			if json.Unmarshal([]byte(cells[3]), &want) != nil || len(want) == 0 {
				t.Fatal("invalid native mutation JSON")
			}
			root := t.TempDir()
			sourceRoot := filepath.Join(directory, "api"+strings.TrimSuffix(api, ".0"))
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
				if !strings.Contains(string(receipt), fmt.Sprintf("source_sha256:%s:%s=%x\n", api, filepath.ToSlash(rel), sha256.Sum256(body))) {
					t.Fatalf("native source changed: %s", rel)
				}
				writeFile(t, filepath.Join(root, rel), string(body))
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native source compiled; Glade diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			report := Run(index, Options{SelectedClasses: []string{"CedarDeliveryObservationTest"}, SelectedMethod: method, NoDiskCache: true, Parallelism: 1})
			if len(report.Suites) != 1 || len(report.Suites[0].Cases) != 1 {
				t.Fatalf("unexpected test selection: %+v", report.Summary())
			}
			result := report.Suites[0].Cases[0]
			if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
				t.Fatalf("native observation completed; Glade problem: %+v", result.Problem)
			}
			_, payload, ok := strings.Cut(result.Problem.Message, "OBSERVED|"+method+"|")
			var got map[string][]string
			if !ok || json.Unmarshal([]byte(payload), &got) != nil {
				t.Fatalf("missing mutation observation: %+v", result.Problem)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("native=%s; Glade=%s", cells[3], payload)
			}
		})
	}
}

func TestChangeDataCaptureJournalNativeConformanceAPIRoutes(t *testing.T) {
	cases := readPlatformTransactionCases(t)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			anonymous := readPlatformTransactionNative(t, "cdc", api, "anonymous", cases)
			named := readPlatformTransactionNative(t, "cdc", api, "istest", cases)
			root := copyPlatformTransactionProject(t, "cdc", api)
			index := loadTestIndex(t, root)
			t.Run("project_semantics", func(t *testing.T) {
				analysis := sema.Analyze(index)
				if index.HasErrors() || analysis.HasErrors() {
					t.Fatalf("native project accepted; Glade diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
				}
			})
			var own []TestCase
			for _, row := range Discover(index, Options{SelectedClasses: []string{"CapturePlatformProbeTest"}}) {
				if named[row.MethodName] != nil {
					own = append(own, row)
				}
			}
			if len(own) != len(named) {
				t.Fatalf("discovered %d native CDC methods, want %d", len(own), len(named))
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			triggers, triggerErrors := compileProjectTriggers(index)
			if len(triggerErrors) != 0 {
				t.Fatalf("compile native subscribers: %v", triggerErrors)
			}
			for _, trigger := range triggers {
				if err := runner.base.RegisterTrigger(trigger); err != nil {
					t.Fatal(err)
				}
			}
			methods, methodErrors := compileTestMethodsForIndex(&index, own)
			programs, programErrors := compileTestInvokePrograms(own)
			template, err := os.ReadFile(filepath.Join(platformTransactionFixture, "anonymous.apex"))
			if err != nil {
				t.Fatal(err)
			}
			for _, row := range own {
				t.Run(row.MethodName, func(t *testing.T) {
					t.Run("anonymous", func(t *testing.T) {
						source := strings.ReplaceAll(string(template), "CASE_ID", row.MethodName)
						program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						_, err = runner.newMachine().Execute(program)
						assertPlatformTransactionObservation(t, row.MethodName, err, anonymous[row.MethodName])
					})
					t.Run("named_istest", func(t *testing.T) {
						key := testCaseKey(row)
						if methodErrors[key] != nil || programErrors[key] != nil {
							t.Fatalf("compile: %v %v", methodErrors[key], programErrors[key])
						}
						machine := runner.newMachine()
						machine.EnableTestContext()
						if err := machine.RegisterMethod(methods[key]); err != nil {
							t.Fatal(err)
						}
						_, err := machine.ExecuteTestInClass(programs[key], row.ClassName)
						assertPlatformTransactionObservation(t, row.MethodName, err, named[row.MethodName])
					})
				})
			}
			for _, route := range []struct {
				name        string
				noDiskCache bool
				parallelism int
			}{
				{"project_test_runner", true, 1},
				{"project_cached_parallel_runner", false, 2},
			} {
				t.Run(route.name, func(t *testing.T) {
					report := RunCasesContext(context.Background(), index, Options{SelectedClasses: []string{"CapturePlatformProbeTest"}, NoDiskCache: route.noDiskCache, Parallelism: route.parallelism}, own)
					results := map[string]testreport.Case{}
					for _, suite := range report.Suites {
						for _, result := range suite.Cases {
							results[result.MethodName] = result
						}
					}
					for _, row := range own {
						t.Run(row.MethodName, func(t *testing.T) {
							result, ok := results[row.MethodName]
							if !ok || result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
								t.Fatalf("native observation completed; Glade project status=%s; problem=%+v; exclusions=%v", result.Status, result.Problem, report.ExcludedClasses)
							}
							assertPlatformTransactionPayload(t, row.MethodName, result.Problem.Message, named[row.MethodName])
						})
					}
				})
			}
		})
	}
}
