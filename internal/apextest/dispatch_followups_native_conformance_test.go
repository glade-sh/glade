package apextest

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDispatchFollowupsNativeConformance(t *testing.T) {
	const fixtures = "testdata/conformance/dispatch_followups"
	raw, err := os.ReadFile(filepath.Join(fixtures, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(string(raw)))
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) < 2 {
		t.Fatalf("native rows: %v", err)
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		if len(row) != 6 || (row[0] != "62.0" && row[0] != "67.0") || row[4] == "" || (row[5] != "true" && row[5] != "false") {
			t.Fatalf("invalid native row: %v", row)
		}
		key := row[0] + "/" + row[1]
		if seen[key] {
			t.Fatalf("duplicate native row: %s", key)
		}
		seen[key] = true
		t.Run(key, func(t *testing.T) {
			source, err := os.ReadFile(filepath.Join(fixtures, row[2]+".cls"))
			if err != nil {
				t.Fatal(err)
			}
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+row[0]+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			body := ""
			if row[5] == "true" {
				body = "insert new Account(Name='Aster');"
			}
			body += "System.assertEquals(" + conformanceApexString(row[4]) + "," + row[3] + ");"
			for name, code := range map[string]string{
				row[2]:                    string(source),
				"DispatchObservationTest": "@IsTest private class DispatchObservationTest { @IsTest static void observe(){" + body + "} }",
			} {
				path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
				writeFile(t, path, code)
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+row[0]+"</apiVersion></ApexClass>")
			}
			run := Run(loadTestIndex(t, root), Options{})
			summary := run.Summary()
			if summary.Total != 1 || summary.Passed != 1 {
				for _, suite := range run.Suites {
					for _, test := range suite.Cases {
						if test.Problem != nil {
							t.Logf("%s: %s", test.Name, test.Problem.Message)
						}
					}
				}
				t.Fatalf("native %q; summary=%#v excluded=%+v", row[4], summary, run.ExcludedClasses)
			}
		})
	}
}
