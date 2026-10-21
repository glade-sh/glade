package apextest

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
)

func TestThrowReachabilityNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/throw_reachability"
	raw, err := os.ReadFile(filepath.Join(fixture, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct{ ID, Name, Source string }
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
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
	if len(cases) == 0 || len(rows) != 1+2*len(cases) || strings.Join(rows[0], ",") != "id,api,route,message,line" {
		t.Fatal("incomplete native reachability matrix")
	}
	seen := make(map[string]bool)
	for _, row := range rows[1:] {
		if len(row) != 5 || (row[1] != "62.0" && row[1] != "67.0") || row[2] != "class" {
			t.Fatalf("invalid native row: %v", row)
		}
		key := row[1] + "/" + row[0]
		if seen[key] {
			t.Fatalf("duplicate native row: %s", key)
		}
		seen[key] = true
		t.Run(key, func(t *testing.T) {
			for _, c := range cases {
				if c.ID != row[0] {
					continue
				}
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+row[1]+`"}`)
				path := filepath.Join(root, "force-app/main/default/classes", c.Name+".cls")
				writeFile(t, path, c.Source)
				writeFile(t, path+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+row[1]+`</apiVersion><status>Active</status></ApexClass>`)
				index := loadTestIndex(t, root)
				if index.HasErrors() {
					t.Fatal(index.Diagnostics)
				}
				result := sema.Analyze(index)
				var errors []diagnostic.Diagnostic
				for _, d := range result.Diagnostics {
					if d.Severity == diagnostic.Error {
						errors = append(errors, d)
					}
				}
				if row[3] == "compiled" {
					if len(errors) != 0 {
						t.Fatalf("native compiled; Glade: %+v", errors)
					}
					return
				}
				line, err := strconv.Atoi(row[4])
				if err != nil || row[3] != "Unreachable statement" || line < 1 {
					t.Fatalf("invalid native rejection: %v", row)
				}
				if len(errors) != 1 || errors[0].Code != "GLADESEMA030" || errors[0].NativeMessage != row[3] || errors[0].Range == nil || errors[0].Range.Start.Line != line {
					t.Fatalf("native %q at line %d; Glade: %+v", row[3], line, errors)
				}
				return
			}
			t.Fatalf("missing synthetic source for %s", row[0])
		})
	}
	for _, c := range cases {
		for _, api := range []string{"62.0", "67.0"} {
			if !seen[api+"/"+c.ID] {
				t.Errorf("missing native row: %s/%s", api, c.ID)
			}
		}
	}
}
