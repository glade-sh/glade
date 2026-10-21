package sema

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestDeclarationLayoutNativeConformance(t *testing.T) {
	const fixtures = "testdata/declaration_layout"
	raw, err := os.ReadFile(filepath.Join(fixtures, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(string(raw)))
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 33 {
		t.Fatalf("native rows=%d err=%v", len(rows), err)
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		if len(row) != 4 || (row[0] != "62.0" && row[0] != "67.0") {
			t.Fatalf("invalid native row: %v", row)
		}
		accepted, err := strconv.ParseBool(row[2])
		if err != nil || (!accepted && row[3] == "") {
			t.Fatalf("invalid outcome: %v", row)
		}
		key := row[0] + "/" + row[1]
		if seen[key] {
			t.Fatalf("duplicate native row: %s", key)
		}
		seen[key] = true
		t.Run(key, func(t *testing.T) {
			source, err := os.ReadFile(filepath.Join(fixtures, row[1]+".cls"))
			if err != nil {
				t.Fatal(err)
			}
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{row[1] + ".cls": string(source)}, row[0])
			if got := !result.HasErrors(); got != accepted {
				t.Fatalf("native accepted=%t (%s); Glade accepted=%t diagnostics=%+v", accepted, row[3], got, result.Diagnostics)
			}
		})
	}
}
