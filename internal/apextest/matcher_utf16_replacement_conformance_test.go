package apextest

import (
	"encoding/csv"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// NATIVE ../vm/testdata/conformance/matcher_utf16_replacement/org.tsv:6-9 istest 62.0, 67.0
func TestMatcherUTF16ReplacementRunnerNativeConformance(t *testing.T) {
	dir := filepath.Join("..", "vm", "testdata", "conformance", "matcher_utf16_replacement")
	f, err := os.Open(filepath.Join(dir, "org.tsv"))
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
	source, err := os.ReadFile(filepath.Join(dir, "force-app/main/default/classes/MatcherReplaceTest.cls"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			var want string
			for _, row := range rows[1:] {
				if row[1] == api && row[2] == "istest" {
					_, want, _ = strings.Cut(html.UnescapeString(row[5]), "Actual: ")
				}
			}
			if want == "" {
				t.Fatal("missing native observation")
			}
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/MatcherReplaceTest.cls"), strings.Replace(string(source), "System.assertEquals('ROW',", "System.assertEquals('"+want+"',", 1))
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/MatcherReplaceTest.cls-meta.xml"), fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api))
			report := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{"MatcherReplaceTest"}, NoDiskCache: true, Parallelism: 1})
			if summary := report.Summary(); summary.Total != 1 || summary.Passed != 1 || summary.Errors != 0 {
				t.Fatalf("runner summary %+v; suites %+v", summary, report.Suites)
			}
		})
	}
}
