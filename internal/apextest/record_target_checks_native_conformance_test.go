package apextest

import (
	"encoding/csv"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestRecordTargetChecksNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/record_target_checks"
	file, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	file.Close()
	if err != nil || len(rows) != 33 || !reflect.DeepEqual(rows[0], []string{"source_api", "route", "row", "observation"}) {
		t.Fatalf("incomplete native observations: %v", err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			project := filepath.Join(fixture, "project")
			if err := filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if strings.HasPrefix(entry.Name(), ".") {
					if entry.IsDir() {
						return fs.SkipDir
					}
					return nil
				}
				if entry.IsDir() {
					return nil
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				name, err := filepath.Rel(project, path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, name), strings.ReplaceAll(string(body), "62.0", api))
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native project compiled: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true})
			for _, route := range []string{"anonymous", "isTest"} {
				t.Run(route, func(t *testing.T) {
					want := map[string]string{}
					for _, row := range rows[1:] {
						if len(row) != 4 {
							t.Fatal("invalid native row")
						}
						if row[0] == api && row[1] == route {
							if _, duplicate := want[row[2]]; duplicate {
								t.Fatal("duplicate native row")
							}
							want[row[2]] = row[3]
						}
					}
					if len(want) != 8 {
						t.Fatal("missing native route")
					}
					got := map[string]string{}
					if route == "anonymous" {
						program, err := vm.CompileAnonymousWithOptions("RecordTargetProbe.emit();", vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						result, err := runner.execute(program)
						if err != nil {
							t.Fatal(err)
						}
						for _, line := range result.Debug {
							parts := strings.SplitN(line, "|", 3)
							if len(parts) != 3 || parts[0] != "ROW" {
								t.Fatalf("unexpected observation: %q", line)
							}
							got[parts[1]] = parts[2]
						}
					} else {
						run := Run(index, Options{SelectedClasses: []string{"RecordTargetProbeTest"}, NoDiskCache: true, Parallelism: 1})
						if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
							t.Fatalf("missing named observation: %#v", run.Summary())
						}
						problem := run.Suites[0].Cases[0].Problem
						if problem == nil || !strings.HasSuffix(problem.Type, "AssertException") {
							t.Fatalf("missing assertion transport: %+v", problem)
						}
						_, payload, found := strings.Cut(problem.Message, "OBSERVED|namedRoute|")
						if !found || json.Unmarshal([]byte(payload), &got) != nil {
							t.Fatalf("invalid named observation: %s", problem.Message)
						}
					}
					// Compare every native row, including both directions of the
					// cast between a record and a same-named user class.
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("native=%v; local=%v", want, got)
					}
				})
			}
			for _, route := range []string{"anonymous", "isTest"} {
				t.Run(route+"/tokenArgumentReturnAssignmentAllocation", func(t *testing.T) {
					program, err := vm.CompileAnonymousWithOptions(`for (Integer i = 0; i < 128; i++) {
    Schema.SObjectType token = RecordTargetProbe.echo(Schema.Account.SObjectType);
}`, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					machine := runner.newMachine()
					if route == "isTest" {
						machine.EnableTestContext()
					}
					allocations := testing.AllocsPerRun(3, func() {
						if route == "isTest" {
							_, err = machine.ExecuteTestInClass(program, "RecordTargetProbeTest")
						} else {
							_, err = machine.Execute(program)
						}
						if err != nil {
							t.Fatal(err)
						}
					})
					const limit = 144
					t.Logf("allocations/token argument-return-assignment=%.0f budget=%d", allocations/128, limit)
					if allocations > 128*limit {
						t.Fatalf("token binding probed source: %.0f allocations/iteration, budget %d", allocations/128, limit)
					}
				})
			}
		})
	}
}
