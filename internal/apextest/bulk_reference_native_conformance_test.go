package apextest

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestBulkReferenceNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/bulk_reference"
	file, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma, reader.LazyQuotes = '\t', true
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 5 || strings.Join(rows[0], ",") != "source_api,route,observation" {
		t.Fatal("incomplete native matrix")
	}
	seen := make(map[string]bool)
	for _, row := range rows[1:] {
		if len(row) != 3 || (row[0] != "62.0" && row[0] != "67.0") || (row[1] != "anonymous" && row[1] != "isTest") {
			t.Fatalf("invalid native cell: %v", row)
		}
		key := row[0] + "/" + row[1]
		if seen[key] {
			t.Fatalf("duplicate native cell: %s", key)
		}
		seen[key] = true
		t.Run(key, func(t *testing.T) {
			root := t.TempDir()
			project := filepath.Join(fixture, "project")
			err := filepath.WalkDir(project, func(path string, entry os.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				name, err := filepath.Rel(project, path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, name), strings.ReplaceAll(string(body), "62.0", row[0]))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native source compiled; diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
			}
			var observed string
			var work vm.ScopeAliasPerfCounters
			if row[1] == "anonymous" {
				source, err := os.ReadFile(filepath.Join(fixture, "anonymous.apex"))
				if err != nil {
					t.Fatal(err)
				}
				program, err := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: row[0]})
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				org.APIVersion = row[0]
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				triggers, failures := compileProjectTriggers(index)
				if len(failures) != 0 {
					t.Fatal(failures)
				}
				for _, trigger := range triggers {
					if err := runner.base.RegisterTrigger(trigger); err != nil {
						t.Fatal(err)
					}
				}
				recorder := vm.NewPerfRecorder()
				result, err := runner.execute(program, func(machine *vm.VM) { machine.SetPerfRecorder(recorder) })
				if err != nil {
					t.Fatal(err)
				}
				for _, line := range result.Debug {
					if _, payload, ok := strings.Cut(line, "OBS|"); ok {
						if observed != "" {
							t.Fatal("duplicate observation")
						}
						observed = payload
					}
				}
				work = recorder.Snapshot().ScopeAlias
			} else {
				run := Run(index, Options{SelectedClasses: []string{"BulkReferenceProbeTest"}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1, PerfCounters: true})
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
					t.Fatalf("expected one observation: %+v", run.Summary())
				}
				result := run.Suites[0].Cases[0]
				if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
					t.Fatalf("missing assertion transport: %+v", result)
				}
				_, observed, _ = strings.Cut(result.Problem.Message, "OBS|")
				work = SnapshotPerfCounters().VMPerf.ScopeAlias
			}
			var got, want any
			if json.Unmarshal([]byte(observed), &got) != nil || json.Unmarshal([]byte(row[2]), &want) != nil {
				t.Fatalf("invalid observation: %q", observed)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("native=%v; Glade=%v", want, got)
			}
			t.Logf("native-matched bulk work: calls=%d roots=%d recursive=%d", work.Calls, work.Roots, work.RecursiveVisits)
			// Reading a receiver with current nested lists must not refresh
			// every list against the caller scope on each method return.
			if work.Calls > 64 {
				t.Fatalf("bulk read-only receiver returns rescanned current collections: calls=%d; want <=64", work.Calls)
			}
		})
	}
}
