package sema

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunAsTwoArgumentNativeRejection(t *testing.T) {
	root := "testdata/runas_two_argument"
	f, err := os.Open(filepath.Join(root, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("native rows = %d, want 2", len(rows)-1)
	}
	for _, row := range rows[1:] {
		row := row
		t.Run(row[0], func(t *testing.T) {
			version := strings.TrimSuffix(row[0], ".0")
			source, err := os.ReadFile(filepath.Join(root, "api"+version, "RunAsOverload"+version+".cls"))
			if err != nil {
				t.Fatal(err)
			}
			if row[2] != "rejected" {
				t.Fatalf("unexpected native result %q", row[2])
			}
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"RunAsOverload" + version + ".cls": string(source)}, row[0])
			for _, d := range result.Diagnostics {
				if strings.Contains(d.Message, row[3]) {
					return
				}
			}
			t.Fatalf("native rejection %q missing: %#v", row[3], result.Diagnostics)
		})
	}
}
