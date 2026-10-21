package apextest

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpportunityPermissionNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/opportunity_describe"
	file, err := os.Open(filepath.Join(fixture, "org.tsv"))
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
	count := 0
	for _, row := range rows[1:] {
		if len(row) != 4 || row[1] != "istest" {
			t.Fatalf("invalid native row: %v", row)
		}
		if row[2] != "chatter" {
			continue // Picklist entries are configurable org metadata.
		}
		count++
		t.Run(row[0], func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join("force-app", "main", "default", "classes", "OpportunityDescribeObservation.cls")
			raw, err := os.ReadFile(filepath.Join(fixture, "project", path))
			if err != nil {
				t.Fatal(err)
			}
			source := string(raw)
			transport := "System.assert(false, 'OBSERVED|chatter|' + observed + '|END');"
			if strings.Count(source, transport) != 1 {
				t.Fatal("missing observation transport")
			}
			source = strings.Replace(source, transport, "System.assertEquals("+conformanceApexString(row[3])+", observed);", 1)
			writeFile(t, filepath.Join(root, path), source)
			writeFile(t, filepath.Join(root, path)+"-meta.xml", "<ApexClass><apiVersion>"+row[0]+"</apiVersion></ApexClass>")
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+row[0]+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			run := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{"OpportunityDescribeObservation"}, SelectedMethod: "chatter", NoDiskCache: true})
			if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
				t.Fatalf("native permission mismatch: %#v: %s", summary, firstRunProblem(run))
			}
		})
	}
	if count != 2 {
		t.Fatalf("expected two native permission rows, got %d", count)
	}
}
