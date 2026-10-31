package sema

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"
)

// Salesforce compiler outcomes for an interface requirement inherited through
// an abstract class, with and without a superclass abstract declaration.
func TestInterfaceOverrideNativeCompilerConformance(t *testing.T) {
	root := filepath.Join("testdata", "interface_override")
	f, err := os.Open(filepath.Join(root, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil || len(rows) != 9 {
		t.Fatalf("native rows: %d, %v", len(rows), err)
	}
	for _, row := range rows[1:] {
		t.Run(row[0]+"/"+row[2], func(t *testing.T) {
			files := map[string]string{}
			for _, name := range []string{"ActionHost.cls", "ActionHostTest.cls"} {
				body, err := os.ReadFile(filepath.Join(root, row[0], row[2], name))
				if err != nil {
					t.Fatal(err)
				}
				files[name] = string(body)
			}
			result := analyzeDeclarationProjectWithAPIVersion(t, files, row[0])
			if got, want := !result.HasErrors(), row[3] == "true"; got != want {
				t.Fatalf("compiled = %v, Salesforce = %v; diagnostics: %#v", got, want, result.Diagnostics)
			}
		})
	}
}
