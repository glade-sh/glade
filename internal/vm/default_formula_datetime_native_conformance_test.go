package vm_test

import (
	"encoding/csv"
	"html"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// NATIVE: org.tsv lines 2-5 capture both routes at APIs 62 and 67.
func TestDefaultFormulaDatetimeNativeConformance(t *testing.T) {
	const fixture = "testdata/default_formula_datetime"
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	file, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 5 {
		t.Fatalf("native matrix has %d rows", len(rows))
	}
	for _, row := range rows[1:] {
		row := row
		t.Run(row[1]+"/"+row[2], func(t *testing.T) {
			root := t.TempDir()
			if err := os.CopyFS(root, os.DirFS(filepath.Join(fixture, "project"))); err != nil {
				t.Fatal(err)
			}
			for _, class := range []string{"DefaultFormulaProbe", "DefaultFormulaProbeTest"} {
				path := filepath.Join(root, "force-app/main/default/classes", class+".cls-meta.xml")
				b, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(strings.ReplaceAll(string(b), "__API__", row[1])), 0600); err != nil {
					t.Fatal(err)
				}
			}
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			s, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			index := typesys.Build(p, s)
			want := map[string]string{}
			if row[2] == "anonymous" {
				if row[3] != "True" || row[4] != "True" || row[6] != row[1] || !strings.HasPrefix(row[8], "cleaned:") {
					t.Fatalf("invalid capture: %v", row)
				}
				for _, entry := range strings.Split(html.UnescapeString(row[5]), ";") {
					name, value, ok := strings.Cut(entry, "=")
					if !ok {
						t.Fatalf("invalid row %q", entry)
					}
					want[name] = value
				}
				org := apextest.OrgFromIndex(index)
				org.APIVersion = row[1]
				machine := vm.New(nil)
				machine.SetOrg(&org)
				if err := apextest.RegisterProjectRuntime(machine, index); err != nil {
					t.Fatal(err)
				}
				source, err := os.ReadFile(filepath.Join(fixture, "probe.apex"))
				if err != nil {
					t.Fatal(err)
				}
				program, err := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: row[1]})
				if err != nil {
					t.Fatal(err)
				}
				result, err := machine.Execute(program)
				if err != nil {
					t.Fatal(err)
				}
				got := map[string]string{}
				for _, line := range result.Debug {
					name, value, ok := strings.Cut(line, "=")
					if ok {
						got[name] = value
					}
				}
				for name, value := range want {
					if !equalDefaultFormulaObservation(got[name], value) {
						t.Errorf("%s: native=%q Glade=%q", name, value, got[name])
					}
				}
				if len(want) != 5 || len(got) != 5 {
					t.Fatalf("native=%d Glade=%d observations", len(want), len(got))
				}
			} else if row[2] == "istest" {
				if row[3] != "true" || row[4] != "Failed" || row[6] != row[1] || row[8] != "n/a" {
					t.Fatalf("invalid capture: %v", row)
				}
				for _, entry := range strings.Split(row[5], "; ") {
					name, tail, ok := strings.Cut(entry, "=Fail ")
					if !ok {
						t.Fatalf("invalid row %q", entry)
					}
					_, value, ok := strings.Cut(tail, "Actual: ")
					if !ok {
						t.Fatalf("invalid assertion %q", entry)
					}
					want[name] = value
				}
				run := apextest.Run(index, apextest.Options{SelectedClasses: []string{"DefaultFormulaProbeTest"}, NoDiskCache: true, Parallelism: 1})
				got := map[string]string{}
				for _, suite := range run.Suites {
					for _, c := range suite.Cases {
						if c.Problem == nil {
							t.Fatalf("missing assertion: %s", c.MethodName)
						}
						_, value, ok := strings.Cut(c.Problem.Message, "Actual: ")
						if !ok {
							t.Fatalf("%s: %s", c.MethodName, c.Problem.Message)
						}
						got[c.MethodName] = value
					}
				}
				for name, value := range want {
					if !equalDefaultFormulaObservation(got[name], value) {
						t.Errorf("%s: native=%q Glade=%q", name, value, got[name])
					}
				}
				if len(want) != 5 || len(got) != 5 {
					t.Fatalf("native=%d Glade=%d observations", len(want), len(got))
				}
			} else {
				t.Fatalf("unknown route %q", row[2])
			}
		})
	}
}

// The same expression on an uncaptured, non-default formula surface retains
// the base behavior; this is a differential control, not a Salesforce claim.
func TestDefaultFormulaNeighborUnchanged(t *testing.T) {
	org := storage.NewOrgState()
	field := storage.Field{Type: storage.FieldDecimal}
	_, _, ok := dml.EvaluateRecordFormulaValueInOrg("MINUTE(TIMEVALUE(NOW()))", field, &org, storage.ObjectDefinition{}, storage.Record{})
	if ok {
		t.Fatal("uncaptured non-default expression unexpectedly evaluated")
	}
}

func equalDefaultFormulaObservation(got, want string) bool { return got == want }
