package apextest

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/lsp"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestStaticRecordAliasLoopNativeRoutes(t *testing.T) {
	const dir = "testdata/conformance/static_alias_loop"
	file, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma, reader.LazyQuotes = '\t', true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 25 {
		t.Fatalf("incomplete native matrix: %d rows, %v", len(rows), err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			project := filepath.Join(dir, "project")
			err := filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				raw, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				name, err := filepath.Rel(project, path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, name), strings.ReplaceAll(string(raw), "67.0", api))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native source compiled: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			check := func(route, variant, payload string) {
				t.Helper()
				var got, want any
				if err := json.Unmarshal([]byte(payload), &got); err != nil {
					t.Fatalf("invalid %s/%s payload: %q", route, variant, payload)
				}
				for _, row := range rows[1:] {
					if len(row) != 4 || row[0] != api || row[1] != route || row[2] != variant {
						continue
					}
					if err := json.Unmarshal([]byte(row[3]), &want); err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("%s/%s native=%s local=%s", route, variant, row[3], payload)
					}
					canonical, _ := json.Marshal(got)
					t.Logf("RESULT|%s/%s/%s|%s", api, route, variant, canonical)
					return
				}
				t.Fatalf("missing native row %s/%s/%s", api, route, variant)
			}
			t.Run("anonymous", func(t *testing.T) {
				source, err := os.ReadFile(filepath.Join(project, "anonymous.apex"))
				if err != nil {
					t.Fatal(err)
				}
				program, err := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				org.APIVersion = api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				recorder := vm.NewPerfRecorder()
				_, err = runner.execute(program, func(machine *vm.VM) { machine.SetPerfRecorder(recorder) })
				if err == nil {
					t.Fatal("missing anonymous assertion observation")
				}
				_, payload, ok := strings.Cut(err.Error(), "OBS|")
				observed := map[string]json.RawMessage{}
				if !ok || json.Unmarshal([]byte(payload), &observed) != nil || len(observed) != 6 {
					t.Fatalf("invalid anonymous observation: %v", err)
				}
				for variant, body := range observed {
					check("anonymous", variant, string(body))
				}
				assertStaticRecordAliasReplacementMeasured(t, recorder.Snapshot())
			})
			t.Run("named_runner_lsp", func(t *testing.T) {
				result := Run(index, Options{SelectedClasses: []string{"StaticAliasProbeTest"}, NoDiskCache: true, Parallelism: 1, PerfCounters: true})
				if len(result.Suites) != 1 || len(result.Suites[0].Cases) != 6 {
					t.Fatalf("missing native methods: %+v", result.Summary())
				}
				for _, cell := range result.Suites[0].Cases {
					if cell.Problem == nil || !strings.HasSuffix(cell.Problem.Type, "AssertException") {
						t.Fatalf("missing observation: %+v", cell)
					}
					_, payload, ok := strings.Cut(cell.Problem.Message, "OBS|"+cell.MethodName+"|")
					if !ok {
						t.Fatalf("missing payload: %s", cell.Problem.Message)
					}
					check("istest", cell.MethodName, payload)
				}
				diagnostics := lsp.TestDiagnostics(result)
				if len(diagnostics) != 6 {
					t.Fatalf("LSP observation diagnostics=%d, want 6", len(diagnostics))
				}
				assertStaticRecordAliasReplacementMeasured(t, SnapshotPerfCounters().VMPerf)
				for _, diag := range diagnostics {
					if diag.Code != "GLADETEST001" || !strings.Contains(diag.Message, "OBS|") {
						t.Fatalf("LSP lost assertion observation: %+v", diag)
					}
				}
			})
			t.Run("linear_project_work", func(t *testing.T) {
				org := orgFromIndex(index)
				org.APIVersion = api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				for _, mutation := range []string{"assignment", "put"} {
					var last uint64
					for _, count := range []int{200, 800, 4000} {
						machine := runner.newMachine()
						setup := fmt.Sprintf(`StaticAliasProbe.rows = new List<Account>();
							for (Integer i = 0; i < %d; i++) {
								Account row = (Account)Account.SObjectType.newSObject();
								row.Name = 'Synthetic';
								StaticAliasProbe.rows.add(row);
								StaticAliasProbe.rows.add(row);
							}`, count)
						execute := func(source string) {
							t.Helper()
							program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
							if err != nil {
								t.Fatal(err)
							}
							if _, err := machine.Execute(program); err != nil {
								t.Fatal(err)
							}
						}
						execute(setup)
						recorder := vm.NewPerfRecorder()
						machine.SetPerfRecorder(recorder)
						write := "row.Description = 'processed';"
						if mutation == "put" {
							write = "row.put('Description', 'processed');"
						}
						execute("for (Account row : StaticAliasProbe.rows) { " + write + " }")
						machine.SetPerfRecorder(nil)
						execute(fmt.Sprintf(`System.assertEquals(%d, StaticAliasProbe.rows.size());
							for (Account row : StaticAliasProbe.rows) { System.assertEquals('processed', row.Description); }`, 2*count))
						stats := recorder.Snapshot()
						replacement := assertStaticRecordAliasReplacementMeasured(t, stats)
						visits := replacement + stats.StaticAlias.RefScanVisits + stats.StaticAlias.LocationVisits +
							stats.ScopeAlias.Roots + stats.ScopeAlias.RecursiveVisits + stats.ScopeAlias.ShortcutVisits + stats.ScopeAlias.ElisionFallbackWalks
						t.Logf("%s records=%d replacement=%d counted_visits=%d", mutation, count, replacement, visits)
						if visits > uint64(200*count) || (last != 0 && visits > 6*last) {
							t.Errorf("%s records=%d counted_visits=%d prior=%d; want linear work", mutation, count, visits, last)
						}
						last = visits
					}
				}
			})
		})
	}
}

func assertStaticRecordAliasReplacementMeasured(t *testing.T, stats vm.PerfCounters) uint64 {
	t.Helper()
	// Keep this native project route accountable for static replacement work,
	// including occurrence-index preflight and maintenance.
	raw, err := json.Marshal(stats.StaticAlias)
	if err != nil {
		t.Fatal(err)
	}
	counters := map[string]uint64{}
	if err := json.Unmarshal(raw, &counters); err != nil {
		t.Fatal(err)
	}
	t.Logf("static_replacement_visits=%d", counters["replacementVisits"])
	if counters["replacementVisits"] == 0 {
		t.Error("static replacement work is unmeasured")
	}
	return counters["replacementVisits"]
}
