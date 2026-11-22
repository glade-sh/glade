package sema

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

// NATIVE ../vm/testdata/community_case_anonymous/org.tsv:4-5 anonymous 62.0 67.0.
func TestCommunityCaseAnonymousCompileNativeConformance(t *testing.T) {
	base := filepath.Join("..", "vm", "testdata", "community_case_anonymous")
	f, err := os.Open(filepath.Join(base, "org.tsv"))
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
	source, err := os.ReadFile(filepath.Join(base, "delete.apex"))
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		if row[0] != "case_delete" {
			continue
		}
		row := row
		t.Run(row[1], func(t *testing.T) {
			body := strings.ReplaceAll(string(source), "__API__", strings.TrimSuffix(row[1], ".0"))
			result := AnalyzeAnonymousWithCommunityCaseDeletePolicy(typesys.Index{}, body, row[1], true)
			if len(result.Diagnostics) != 1 || result.Diagnostics[0].Message != row[5] {
				t.Fatalf("diagnostics=%v native=%q", result.Diagnostics, row[5])
			}
		})
	}
}
