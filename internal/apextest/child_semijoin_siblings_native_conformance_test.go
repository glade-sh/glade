package apextest

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// NATIVE: testdata/conformance/child_semijoin_siblings/org.tsv:2-5,
// exact/org.tsv:2-5, relationship/initial/org.tsv:2, and
// relationship/repeat/org.tsv:2-4, and fullshape/org.tsv:2-5.
// Tainted rows are not used.
func TestChildSemiJoinSiblingRoutesNativeConformance(t *testing.T) {
	const base = "testdata/conformance/child_semijoin_siblings"
	for _, tc := range []struct {
		name, source, class string
		tables              []string
	}{
		{"seeded", "seeded.apex", "SeededQueryTest", []string{"org.tsv"}},
		{"exact", "exact.apex", "ExactQueryTest", []string{"exact/org.tsv"}},
		{"relationship", "relationship.apex", "RelationshipQueryTest", []string{"relationship/initial/org.tsv", "relationship/repeat/org.tsv"}},
		{"fullshape", "fullshape.apex", "FullShapeQueryTest", []string{"fullshape/org.tsv"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := base
			if tc.name != "seeded" {
				fixture = filepath.Join(base, tc.name)
			}
			seen := make(map[string]bool)
			for _, table := range tc.tables {
				f, err := os.Open(filepath.Join(base, table))
				if err != nil {
					t.Fatal(err)
				}
				reader := csv.NewReader(f)
				reader.Comma = '\t'
				rows, err := reader.ReadAll()
				f.Close()
				if err != nil {
					t.Fatal(err)
				}
				for _, row := range rows[1:] {
					api, route := row[1], row[2]
					if strings.HasPrefix(row[8], "tainted:") {
						continue
					}
					key := api + "/" + route
					if seen[key] {
						t.Fatalf("duplicate clean native row %s", key)
					}
					seen[key] = true
					if route == "anonymous" {
						if row[3] != "True" || row[4] != "True" || row[5] != "SEEDED=pass" {
							t.Fatalf("unexpected row: %q", row)
						}
					} else if route == "istest" {
						if row[3] != "true" || row[4] != "Passed" || row[5] != "run=Pass" {
							t.Fatalf("unexpected row: %q", row)
						}
					} else {
						t.Fatalf("unexpected route: %q", row)
					}
					t.Run(key, func(t *testing.T) {
						root := t.TempDir()
						writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
						source, err := os.ReadFile(filepath.Join(fixture, tc.source))
						if err != nil {
							t.Fatal(err)
						}
						if route == "istest" {
							classPath := filepath.Join(fixture, "force-app/main/default/classes", tc.class+".cls")
							class, err := os.ReadFile(classPath)
							if err != nil {
								t.Fatal(err)
							}
							writeFile(t, filepath.Join(root, "force-app/main/default/classes", tc.class+".cls"), string(class))
							writeFile(t, filepath.Join(root, "force-app/main/default/classes", tc.class+".cls-meta.xml"), fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api))
						}
						index := loadTestIndex(t, root)
						if route == "istest" {
							run := Run(index, Options{SelectedClasses: []string{tc.class}, NoDiskCache: true, Parallelism: 1})
							if s := run.Summary(); s.Total != 1 || s.Passed != 1 {
								t.Fatalf("result: %+v; %s", s, firstRunProblem(run))
							}
							return
						}
						program, err := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						org := orgFromIndex(index)
						org.APIVersion = api
						runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
						if _, err := runner.newMachine().Execute(program); err != nil {
							t.Fatal(err)
						}
					})
				}
			}
			if len(seen) != 4 || !seen["62.0/anonymous"] || !seen["67.0/anonymous"] || !seen["62.0/istest"] || !seen["67.0/istest"] {
				t.Fatalf("missing clean native rows: %v", seen)
			}
		})
	}
}
