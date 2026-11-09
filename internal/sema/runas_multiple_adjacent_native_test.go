package sema

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunAsMultipleAdjacentNativeRows(t *testing.T) {
	root := "testdata/runas_multiple_adjacent"
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
	if len(rows) != 7 {
		t.Fatalf("native rows=%d, want 6", len(rows)-1)
	}
	for _, row := range rows[1:] {
		row := row
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
			version := strings.TrimSuffix(row[0], ".0")
			name := "RunAsAdj" + version + row[1]
			source, err := os.ReadFile(filepath.Join(root, "api"+version+"_"+row[1], name+".cls"))
			if err != nil {
				t.Fatal(err)
			}
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{name + ".cls": string(source)}, row[0])
			if row[2] == "accepted" {
				if result.HasErrors() {
					t.Fatalf("native accepted: %#v", result.Diagnostics)
				}
				return
			}
			for _, d := range result.Diagnostics {
				if strings.Contains(d.Message, row[3]) {
					return
				}
			}
			t.Fatalf("native rejection %q missing: %#v", row[3], result.Diagnostics)
		})
	}
}

func TestRunAsMultipleRecoveryPreservesOtherSyntax(t *testing.T) {
	for _, body := range []string{
		`System.runAs(new User(), new Version(1,0)) {} Integer n = ;`,
		`String text = 'System.runAs(x,y)'; Integer n = ;`,
		`// System.runAs(x,y) {}
 Integer n = ;`,
	} {
		result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"Probe.cls": `@IsTest private class Probe { @IsTest static void check() { ` + body + ` } }`}, "67.0")
		if !hasDiagnosticCode(result.Diagnostics, "APEXPARSE001") {
			t.Fatalf("unrelated syntax error lost: %#v", result.Diagnostics)
		}
	}
}
