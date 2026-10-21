package sema

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
)

func TestConstructorInitializerNativeConformance(t *testing.T) {
	const fixtures = "testdata/field_initializer"
	raw, err := os.ReadFile(filepath.Join(fixtures, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(string(raw)))
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 15 {
		t.Fatalf("native rows=%d err=%v", len(rows), err)
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		if len(row) != 4 || (row[0] != "62.0" && row[0] != "67.0") {
			t.Fatalf("invalid native row: %v", row)
		}
		accepted, err := strconv.ParseBool(row[2])
		if err != nil || (!accepted && row[3] == "-") {
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
			var problems []string
			for _, d := range result.Diagnostics {
				if d.Severity == diagnostic.Error {
					message := d.NativeMessage
					if message == "" {
						message = d.Message
					}
					problems = append(problems, message)
				}
			}
			want := ""
			if !accepted {
				want = row[3]
			}
			if got := strings.Join(problems, " | "); got != want {
				t.Fatalf("native accepted=%t problems=%q; Glade problems=%q", accepted, want, got)
			}
		})
	}
}
