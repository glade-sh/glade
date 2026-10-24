package apextest

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/vm"
)

func TestRuntimeSchemaIsolationNativeConformance(t *testing.T) {
	runRuntimeSchemaIsolationNativeConformance(t)
}

func runRuntimeSchemaIsolationNativeConformance(t *testing.T) {
	t.Helper()
	fixture := filepath.Join("testdata", "conformance", "runtime_schema_isolation")
	raw, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	rows := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(rows) != 7 || rows[0] != "source_api\troute\tobservation" {
		t.Fatal("incomplete native matrix")
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			err := filepath.WalkDir(filepath.Join(fixture, "fixture"), func(path string, entry os.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if entry.IsDir() {
					return nil
				}
				rel, err := filepath.Rel(filepath.Join(fixture, "fixture"), path)
				if err != nil {
					return err
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				source := strings.ReplaceAll(string(data), "62.0", api)
				writeFile(t, filepath.Join(root, rel), source)
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			org := orgFromIndex(index)
			initializeTestOrg(&org)
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			for _, line := range rows[1:] {
				row := strings.Split(line, "\t")
				if len(row) != 3 {
					t.Fatal("invalid native row")
				}
				if row[0] != api {
					continue
				}
				t.Run(row[1], func(t *testing.T) {
					if row[1] == "anonymous" {
						source, err := os.ReadFile(filepath.Join(root, "anonymous.apex"))
						if err != nil {
							t.Fatal(err)
						}
						program, err := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						result, err := runner.execute(program)
						if err != nil {
							t.Fatal(err)
						}
						if !strings.Contains(strings.Join(result.Debug, "\n"), "SCHEMA_ROW:"+row[2]) {
							t.Fatalf("native=%s local logs=%v", row[2], result.Debug)
						}
						return
					}
					class := "SchemaIsolationPlainTest"
					if row[1] == "isTest_setup" {
						class = "SchemaIsolationSetupTest"
					}
					cases := Discover(index, Options{SelectedClasses: []string{class}})
					if len(cases) != 4 {
						t.Fatalf("discovered %d cases, want 4", len(cases))
					}
					methods, methodErrors := compileTestMethodsForIndex(&index, cases)
					programs, programErrors := compileTestInvokePrograms(cases)
					runtimeMethods := indexTestRuntimeMethods(methods)
					setups, setupErrors, setupPrograms, setupProgramErrors := compileTestSetupMethods(index)
					counters := newRunPerfCounters(false)
					seed, random, setupErr, _ := prepareTestSetupOrg(context.Background(), class, runner.base, nil, setups[class], setupErrors[class], setupPrograms[class], setupProgramErrors[class], nil, runner.newOrg(), Options{}, counters)
					if setupErr != nil {
						t.Fatal(setupErr)
					}
					journal := storage.NewIsolationJournal(&seed)
					generation := seed.RuntimeGenerationID()
					builds := 0
					for _, tc := range cases {
						key := testCaseKey(tc)
						if methodErrors[key] != nil || programErrors[key] != nil {
							t.Fatalf("compile: %v %v", methodErrors[key], programErrors[key])
						}
						mark := markTestJournal(journal)
						seed.RuntimeGenerationCacheValue("owned:reset-index:"+row[1], func() any { builds++; return new(int) })
						result := runCase(context.Background(), tc, methods[key], runtimeMethods[testMethodSourceKey(tc.ClassName, tc.File)], nil, programs[key], nil, runner.base, nil, nil, nil, seed, random, Options{}, false, journal, counters)
						if result.Status != testreport.StatusPass {
							t.Fatalf("%s: %s %#v", tc.MethodName, result.Status, result.Problem)
						}
						// The deployed method's assertion pins this captured native value.
						source, err := os.ReadFile(tc.File)
						if err != nil {
							t.Fatal(err)
						}
						if !strings.Contains(string(source), "System.assertEquals('"+row[2]+"', observed)") {
							t.Fatal("method assertion detached from native row")
						}
						if err := rollbackJournal(journal, mark, counters); err != nil {
							t.Fatal(err)
						}
					}
					if got := seed.RuntimeGenerationID(); got != generation {
						t.Errorf("per-test reset generation=%d, want %d", got, generation)
					}
					if builds != 1 {
						t.Errorf("per-test reset index builds=%d, want 1", builds)
					}
				})
			}
			// Exercise the ordinary project scheduler and both journal worker routes.
			for _, parallelMethods := range []bool{false, true} {
				run := Run(index, Options{Parallelism: 2, ParallelMethods: parallelMethods, NoDiskCache: true})
				if got := run.Summary(); got.Total != 8 || got.Passed != 8 {
					t.Fatalf("parallelMethods=%v summary=%#v", parallelMethods, got)
				}
			}
		})
	}
}
