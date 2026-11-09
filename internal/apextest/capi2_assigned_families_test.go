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

// org.tsv records the native argument error preceding the data-silo boundary.
func TestCapi2AssignedOutputFamilies(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "capi2_assigned_outputs")
	data, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct{ API, ID, Code string }
	if err = json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	expected := map[string]string{}
	scan := bufio.NewScanner(f)
	for scan.Scan() {
		parts := strings.SplitN(scan.Text(), "\t", 4)
		if len(parts) == 4 && parts[0] != "api" {
			expected[parts[0]+"/"+parts[2]] = parts[3]
		}
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			byClass := map[string]struct{ id, want string }{}
			for i, row := range rows {
				if row.API != api {
					continue
				}
				want, ok := expected[api+"/"+row.ID]
				if !ok {
					t.Fatalf("missing native row %s", row.ID)
				}
				name := fmt.Sprintf("CapiAssigned%04d", i)
				byClass[name] = struct{ id, want string }{row.ID, want}
				path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
				writeFile(t, path, "@IsTest private class "+name+" { @IsTest static void probe() {\n"+row.Code+"\nSystem.assert(false, 'ROW|'+o); } }")
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
			}
			run := Run(loadTestIndex(t, root), Options{Parallelism: 1, NoDiskCache: true})
			seen := map[string]bool{}
			failures := 0
			for _, suite := range run.Suites {
				for _, c := range suite.Cases {
					row, ok := byClass[c.ClassName]
					if !ok {
						continue
					}
					seen[c.ClassName] = true
					msg := ""
					if c.Problem != nil {
						msg = c.Problem.Message
					}
					_, got, found := strings.Cut(msg, "ROW|")
					matched := c.Status == testreport.StatusFail && found && strings.TrimSpace(got) == row.want
					if strings.HasPrefix(row.want, "COMPILE|") {
						matched = c.Status == testreport.StatusCompileError
					}
					if !matched {
						failures++
						if failures <= 8 {
							t.Errorf("%s: native=%s glade=%s %s", row.id, row.want, c.Status, msg)
						}
					}
				}
			}
			if len(seen) != len(byClass) || failures > 0 {
				t.Errorf("cases=%d observed=%d mismatches=%d", len(byClass), len(seen), failures)
			}
		})
	}
}
