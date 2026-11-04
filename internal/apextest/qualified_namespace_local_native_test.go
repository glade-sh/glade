package apextest

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

// Salesforce rejects these qualified local value types while accepting their
// unqualified forms and the qualified static receiver in namespace_diagnostics.
func TestQualifiedNamespaceLocalNativeConformance(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "namespace_diagnostics", "qualified_locals")
	var cases []struct {
		ID, Code          string
		NamedCompilations map[string]struct{ Name, Source string }
	}
	data, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	sources := map[string]struct {
		ID, Code          string
		NamedCompilations map[string]struct{ Name, Source string }
	}{}
	for _, c := range cases {
		sources[c.ID] = c
	}
	f, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 17 {
		t.Fatalf("native matrix rows=%d", len(rows))
	}
	for _, row := range rows[1:] {
		row := row
		t.Run(row[0]+"/"+row[1]+"/"+row[2], func(t *testing.T) {
			cell, ok := sources[row[2]]
			if !ok {
				t.Fatal("missing source")
			}
			var result sema.Result
			if row[1] == "anonymous" {
				index := nativeBoundaryIndex(t, row[0], map[string]string{"P": "public class P { public void out(String id,Object value) {} }"}, nil)
				result = sema.AnalyzeAnonymous(index, "P pq=new P(); Object r; "+cell.Code, row[0])
			} else {
				named := cell.NamedCompilations[row[0]]
				if named.Name == "" {
					t.Fatal("missing named source")
				}
				index := nativeBoundaryIndex(t, row[0], map[string]string{named.Name: named.Source}, nil)
				result = sema.Analyze(index)
			}
			found := false
			for _, d := range result.Diagnostics {
				if d.NativeMessage == row[4] {
					found = true
				}
			}
			if !found {
				t.Fatalf("native diagnostic %q; local=%+v", row[4], result.Diagnostics)
			}
			if row[1] == "istest" {
				named := cell.NamedCompilations[row[0]]
				index := nativeBoundaryIndex(t, row[0], map[string]string{named.Name: named.Source}, nil)
				for _, warm := range []bool{false, true} {
					run := Run(index, Options{Parallelism: 1, SelectedClasses: []string{named.Name}})
					summary := run.Summary()
					if summary.CompileErrors == 0 && summary.ExcludedClasses == 0 {
						t.Fatalf("warm=%v native compile rejection, local=%+v", warm, summary)
					}
				}
			}
		})
	}
}
