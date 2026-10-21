package apextest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestRecordLoopAliasNativeConformance(t *testing.T) {
	const dir = "testdata/conformance/record_loop_alias"
	sources := map[string]string{}
	for _, name := range []string{"AliasLoopProbe", "AliasLoopProbeTest"} {
		raw, err := os.ReadFile(filepath.Join(dir, name+".cls"))
		if err != nil {
			t.Fatal(err)
		}
		sources[name] = string(raw)
	}
	anonymous, err := os.ReadFile(filepath.Join(dir, "anonymous.apex"))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 5 || lines[0] != "source_api\troute\tobservation" {
		t.Fatal("incomplete native loop matrix")
	}
	seen := map[string]bool{}
	for _, line := range lines[1:] {
		row := strings.SplitN(line, "\t", 3)
		if len(row) != 3 || (row[0] != "62.0" && row[0] != "67.0") || (row[1] != "anonymous" && row[1] != "isTest") {
			t.Fatalf("invalid native loop cell: %s", line)
		}
		key := row[0] + "/" + row[1]
		if seen[key] {
			t.Fatalf("duplicate native loop cell: %s", key)
		}
		seen[key] = true
		t.Run(key, func(t *testing.T) {
			index := nativeBoundaryIndex(t, row[0], sources, nil)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native source compiled; Glade diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			var observed string
			if row[1] == "anonymous" {
				program, err := vm.CompileAnonymousWithOptions(string(anonymous), vm.CompileOptions{APIVersion: row[0]})
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				org.APIVersion = row[0]
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				result, err := runner.execute(program)
				if err != nil {
					t.Fatal(err)
				}
				for _, line := range result.Debug {
					if _, payload, ok := strings.Cut(line, "OBS|"); ok {
						if observed != "" {
							t.Fatal("duplicate loop observation")
						}
						observed = payload
					}
				}
			} else {
				result := Run(index, Options{SelectedClasses: []string{"AliasLoopProbeTest"}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
				if len(result.Suites) != 1 || len(result.Suites[0].Cases) != 1 {
					t.Fatalf("expected one loop observation: %+v", result.Summary())
				}
				problem := result.Suites[0].Cases[0].Problem
				if problem == nil || !strings.HasSuffix(problem.Type, "AssertException") {
					t.Fatalf("missing native loop assertion transport: %+v", problem)
				}
				_, observed, _ = strings.Cut(problem.Message, "OBS|")
			}
			var got, want any
			if json.Unmarshal([]byte(observed), &got) != nil || json.Unmarshal([]byte(row[2]), &want) != nil {
				t.Fatalf("invalid loop observation: %q", observed)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("native=%v; Glade=%v", want, got)
			}
		})
	}
	if !seen["62.0/anonymous"] || !seen["67.0/anonymous"] {
		t.Fatal("missing source API loop observations")
	}
	assertRecordFieldLoopLinearWork(t, sources)
}

func assertRecordFieldLoopLinearWork(t *testing.T, sources map[string]string) {
	t.Helper()
	index := nativeBoundaryIndex(t, "62.0", sources, nil)
	org := orgFromIndex(index)
	org.APIVersion = "62.0"
	for _, mutation := range []string{"assignment", "put"} {
		for _, reverse := range []bool{false, true} {
			order := "forward"
			if reverse {
				order = "reverse"
			}
			visitsByCount := make(map[int]uint64, 2)
			for _, count := range []int{200, 800} {
				write := "row.Description = 'processed';"
				if mutation == "put" {
					write = "row.put('Description', 'processed');"
				}
				source := "for (Account row : rows) { " + write + " }"
				if reverse {
					source = "for (Integer i = rows.size() - 1; i >= 0; i--) { Account row = rows[i]; " + write + " }"
				}
				program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: "62.0"})
				if err != nil {
					t.Fatal(err)
				}
				machine := vm.New(nil)
				machine.SetOrg(&org)
				rows := vm.List()
				rows.Type = "List<Account>"
				for range count {
					row := vm.Object("Account")
					row.Fields["Name"] = vm.String("Synthetic")
					rows.List = append(rows.List, row)
				}
				machine.Globals["rows"] = rows
				recorder := vm.NewPerfRecorder()
				machine.SetPerfRecorder(recorder)
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
				for i, row := range machine.Globals["rows"].List {
					if got := row.Fields["Description"]; got.Kind != vm.ValueString || got.Text != "processed" {
						t.Fatalf("%s/%s/%d row %d Description=%#v, want processed", mutation, order, count, i, got)
					}
				}
				stats := recorder.Snapshot().ScopeAlias
				visits := stats.Roots + stats.RecursiveVisits + stats.ShortcutVisits + stats.ElisionFallbackWalks
				visitsByCount[count] = visits
				if limit := uint64(40 * count); visits > limit {
					t.Fatalf("%s/%s records=%d counted_visits=%d, want <= %d", mutation, order, count, visits, limit)
				}
			}
			if visitsByCount[800] > 5*visitsByCount[200] {
				t.Fatalf("%s/%s counted visits grew from %d to %d; want at most 5x", mutation, order, visitsByCount[200], visitsByCount[800])
			}
		}
	}
}
