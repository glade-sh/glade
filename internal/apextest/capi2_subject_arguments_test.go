package apextest

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/testreport"
)

func TestCapi2SubjectArgumentFormat(t *testing.T) {
	for _, entry := range []struct{ dir, api string }{{"capi2_subject_arguments", "62.0"}, {"capi2_subject_arguments", "67.0"}, {"capi2_subject_arguments", "68.0"}} {
		t.Run(entry.api, func(t *testing.T) {
			dir := filepath.Join("testdata", "conformance", entry.dir)
			raw, err := os.ReadFile(filepath.Join(dir, "cases.json"))
			if err != nil {
				t.Fatal(err)
			}
			var cases []struct{ API, Group, ID, Code string }
			if err = json.Unmarshal(raw, &cases); err != nil {
				t.Fatal(err)
			}
			f, err := os.Open(filepath.Join(dir, "org.tsv"))
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			native := map[string]string{}
			s := bufio.NewScanner(f)
			for s.Scan() {
				p := strings.SplitN(s.Text(), "\t", 5)
				if len(p) == 5 && p[0] == entry.api && p[2] == "subject-format" {
					native[p[3]] = p[4]
				}
			}
			if err = s.Err(); err != nil {
				t.Fatal(err)
			}
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+entry.api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			classes := map[string]string{}
			for i, row := range cases {
				if row.Group != "subject-format" || (row.API != "" && row.API != entry.api) {
					continue
				}
				if _, ok := native[row.ID]; !ok {
					t.Fatalf("missing row %s", row.ID)
				}
				name := fmt.Sprintf("CapiSubject%04d", i)
				classes[name] = row.ID
				path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
				writeFile(t, path, "@IsTest private class "+name+" { @IsTest static void probe() {\n"+row.Code+"\nSystem.assert(false, 'ROW|'+o); } }")
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+entry.api+"</apiVersion><status>Active</status></ApexClass>")
			}
			run := Run(loadTestIndex(t, root), Options{Parallelism: 1, NoDiskCache: true})
			seen, failures := 0, 0
			for _, suite := range run.Suites {
				for _, c := range suite.Cases {
					id, ok := classes[c.ClassName]
					if !ok {
						continue
					}
					seen++
					msg := ""
					if c.Problem != nil {
						msg = c.Problem.Message
					}
					_, got, ok := strings.Cut(msg, "ROW|")
					if c.Status != testreport.StatusFail || !ok || strings.TrimSpace(got) != native[id] {
						failures++
						if failures <= 5 {
							t.Errorf("%s: native=%s glade=%s %s", id, native[id], c.Status, msg)
						}
					}
				}
			}
			if seen != len(classes) || failures > 0 {
				t.Errorf("cases=%d observed=%d mismatches=%d", len(classes), seen, failures)
			}
		})
	}
}
