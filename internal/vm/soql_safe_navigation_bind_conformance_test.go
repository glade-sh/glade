package vm

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

const soqlSafeNavigationBindFixture = "../soql/testdata/soql_safe_navigation_bind"

type soqlSafeNavigationBindCase struct {
	id, source, bind, outcome, observation string
	replay                                 bool
}

type soqlSafeNavigationBindRow struct {
	api, id, source, outcome, observation string
}

func TestSOQLSafeNavigationBindNativeConformance(t *testing.T) {
	cases := loadSOQLSafeNavigationBindCases(t)
	if len(cases) != 15 {
		t.Fatalf("case table has %d rows, want 15", len(cases))
	}
	for _, api := range []string{"62.0", "67.0"} {
		rows := loadSOQLSafeNavigationBindRows(t, api)
		if len(rows) != len(cases) {
			t.Fatalf("API %s has %d native rows, want %d", api, len(rows), len(cases))
		}
		run, err := os.ReadFile(filepath.Join(soqlSafeNavigationBindFixture, "native", "api"+api[:2], "anonymous", "RUN-FACTS.txt"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(run), "source_api="+api) || !strings.Contains(string(run), "context=anonymous") {
			t.Fatalf("API %s run metadata is incomplete", api)
		}
		for _, tc := range cases {
			row, ok := rows[tc.id]
			if !ok || row.api != api || row.source != tc.source || row.outcome != tc.outcome {
				t.Fatalf("API %s native row for %s = %+v; want source %s and outcome %s", api, tc.id, row, tc.source, tc.outcome)
			}
			if tc.outcome == "pass" {
				if row.observation != tc.observation {
					t.Fatalf("API %s native observation for %s = %q, want %q", api, tc.id, row.observation, tc.observation)
				}
			} else if !strings.Contains(row.observation, tc.observation) {
				t.Fatalf("API %s native rejection for %s = %q, want it to contain %q", api, tc.id, row.observation, tc.observation)
			}
			t.Run(api+"/"+tc.id, func(t *testing.T) {
				if !tc.replay {
					t.Skip("native boundary is captured but outside this bind replay")
				}
				source, err := os.ReadFile(filepath.Join(soqlSafeNavigationBindFixture, "native", "sources", tc.source))
				if err != nil {
					t.Fatal(err)
				}
				program, compileErr := CompileAnonymousWithOptions(string(source), CompileOptions{APIVersion: api})
				if tc.outcome == "compile_rejected" {
					if compileErr == nil {
						t.Fatal("Glade compiled a native-rejected static query")
					}
					return
				}
				if compileErr != nil {
					t.Fatalf("Glade compile: %v", compileErr)
				}
				machine := newSOQLSafeNavigationBindMachine(t)
				_, executeErr := machine.Execute(program)
				if tc.outcome == "runtime_rejected" {
					if executeErr == nil {
						t.Fatal("Glade executed a native-rejected dynamic query")
					}
					if !strings.Contains(executeErr.Error(), tc.observation) {
						t.Fatalf("Glade dynamic rejection = %v, want it to contain %q", executeErr, tc.observation)
					}
					return
				}
				if executeErr != nil {
					t.Fatalf("Glade execution: %v", executeErr)
				}
			})
		}
	}
}

func loadSOQLSafeNavigationBindCases(t *testing.T) []soqlSafeNavigationBindCase {
	t.Helper()
	f, err := os.Open(filepath.Join(soqlSafeNavigationBindFixture, "case-table.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	reader.FieldsPerRecord = 6
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 || strings.Join(rows[0], ",") != "case,source,bind,native_outcome,native_observation,local_replay" {
		t.Fatal("case table header changed")
	}
	cases := make([]soqlSafeNavigationBindCase, 0, len(rows)-1)
	for _, row := range rows[1:] {
		cases = append(cases, soqlSafeNavigationBindCase{row[0], row[1], row[2], row[3], row[4], row[5] == "true"})
	}
	return cases
}

func loadSOQLSafeNavigationBindRows(t *testing.T, api string) map[string]soqlSafeNavigationBindRow {
	t.Helper()
	f, err := os.Open(filepath.Join(soqlSafeNavigationBindFixture, "native", "api"+api[:2], "anonymous", "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	reader.FieldsPerRecord = 5
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 || strings.Join(rows[0], ",") != "api,case,source,outcome,observation" {
		t.Fatal("native table header changed")
	}
	out := make(map[string]soqlSafeNavigationBindRow, len(rows)-1)
	for _, row := range rows[1:] {
		if _, exists := out[row[1]]; exists {
			t.Fatalf("duplicate native row for %s/%s", api, row[1])
		}
		out[row[1]] = soqlSafeNavigationBindRow{row[0], row[1], row[2], row[3], row[4]}
	}
	return out
}

func newSOQLSafeNavigationBindMachine(t *testing.T) *VM {
	t.Helper()
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	machine := New(nil)
	machine.SetOrg(&org)
	if err := machine.RegisterClass(Class{
		Name: "OwnedSafeNavigationBindHolder",
		Fields: map[string]Field{
			"b":    {Name: "b", Type: "OwnedSafeNavigationBindHolder"},
			"Name": {Name: "Name", Type: "String"},
			"ids":  {Name: "ids", Type: "List<Id>"},
			"n":    {Name: "n", Type: "Integer"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	return machine
}
