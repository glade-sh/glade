package sema

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"
)

func TestImplicitNamespaceMemberReceiverNativeConformance(t *testing.T) {
	const dir = "testdata/implicit_namespace_collision"
	file, err := os.Open(filepath.Join(dir, "org.tsv"))
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
	if len(rows) != 3 || len(rows[0]) != 4 || rows[0][0] != "source_api" || rows[0][1] != "case" || rows[0][2] != "compiled" || rows[0][3] != "execution_success" {
		t.Fatalf("invalid native table: %#v", rows)
	}

	classes := filepath.Join(dir, "project", "force-app", "main", "default", "classes")
	files := make(map[string]string, 3)
	for _, name := range []string{"NamespaceMemberContainer.cls", "NamespaceMemberProbe.cls", "NamespaceMemberService.cls"} {
		contents, err := os.ReadFile(filepath.Join(classes, name))
		if err != nil {
			t.Fatal(err)
		}
		files[name] = string(contents)
	}

	seen := make(map[string]bool)
	for _, row := range rows[1:] {
		if len(row) != 4 || row[1] != "member_receiver" || row[2] != "true" || row[3] != "true" || seen[row[0]] {
			t.Fatalf("invalid native member receiver row: %v", row)
		}
		seen[row[0]] = true
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, files, row[0])
			if result.HasErrors() {
				t.Fatalf("native accepted and executed member receiver; local diagnostics: %#v", result.Diagnostics)
			}
		})
	}
	if !seen["62.0"] || !seen["67.0"] {
		t.Fatalf("missing API floor/ceiling rows: %#v", seen)
	}
}
