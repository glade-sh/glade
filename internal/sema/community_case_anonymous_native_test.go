package sema

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"reflect"
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
	for _, row := range rows[1:] {
		if row[2] != "anonymous" || row[3] != "false" {
			continue
		}
		row := row
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
			name := row[0]
			if name == "case_delete" {
				name = "delete"
			}
			source, err := os.ReadFile(filepath.Join(base, name+".apex"))
			if err != nil {
				t.Fatal(err)
			}
			body := strings.ReplaceAll(string(source), "__API__", strings.TrimSuffix(row[1], ".0"))
			result := AnalyzeAnonymousWithCommunityCaseDeletePolicy(typesys.Index{}, body, row[1], true)
			if len(result.Diagnostics) != 1 || result.Diagnostics[0].Message != row[5] {
				t.Fatalf("diagnostics=%v native=%q", result.Diagnostics, row[5])
			}
		})
	}
}

// Uncaptured DML modes and operand shapes keep ordinary anonymous diagnostics.
func TestCommunityCaseCompileUncapturedNeighbors(t *testing.T) {
	base := filepath.Join("..", "vm", "testdata", "community_case_anonymous", "delete.apex")
	source, err := os.ReadFile(base)
	if err != nil {
		t.Fatal(err)
	}
	original := strings.ReplaceAll(string(source), "__API__", "67")
	for _, tc := range []struct {
		name, body string
		policy     bool
	}{
		{"policy-off", original, false},
		{"default-mode", strings.Replace(original, "delete as user cases[0]", "delete cases[0]", 1), true},
		{"whole-list", strings.Replace(original, "delete as user cases[0]", "delete as user cases", 1), true},
		{"other-object", strings.ReplaceAll(strings.ReplaceAll(original, "Case", "Account"), "cases", "accounts"), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want := AnalyzeAnonymous(typesys.Index{}, tc.body, "67.0")
			got := AnalyzeAnonymousWithCommunityCaseDeletePolicy(typesys.Index{}, tc.body, "67.0", tc.policy)
			if !reflect.DeepEqual(got.Diagnostics, want.Diagnostics) {
				t.Fatalf("diagnostics=%v base=%v", got.Diagnostics, want.Diagnostics)
			}
		})
	}
	// The shared anonymous setup must not retain a prior request's policy.
	_ = AnalyzeAnonymousWithCommunityCaseDeletePolicy(typesys.Index{}, original, "67.0", true)
	got := AnalyzeAnonymousWithCommunityCaseDeletePolicy(typesys.Index{}, original, "67.0", false)
	want := AnalyzeAnonymous(typesys.Index{}, original, "67.0")
	if !reflect.DeepEqual(got.Diagnostics, want.Diagnostics) {
		t.Fatalf("policy survived request: %v vs %v", got.Diagnostics, want.Diagnostics)
	}
}
