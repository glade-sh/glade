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

// NATIVE child semi-join matrix: testdata/conformance/child_semijoin_matrix/{a,b}/org.tsv,
// 62.0 and 67.0, anonymous and istest. Accepted cells: ID or reference field IN or NOT IN
// a single-field subquery in a child WHERE, with 0, 1, 2 or 5 sibling child queries, no
// LIMIT, LIMIT 1 or LIMIT 2, with or without scalar conjuncts, and two joins in one child
// WHERE. Rejected cells: a semi-join with OR, and inside a nested group.
func TestChildSemiJoinMatrixNativeConformance(t *testing.T) {
	const base = "testdata/conformance/child_semijoin_matrix"
	classes := map[string]string{}
	cases, err := readTSV(filepath.Join(base, "cases.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cases[1:] {
		classes[c[0]] = c[2]
	}
	seen := map[string]bool{}
	for _, part := range []string{"a", "b"} {
		rows, err := readTSV(filepath.Join(base, part, "org.tsv"))
		if err != nil {
			t.Fatal(err)
		}
		for i, row := range rows[1:] {
			id, api, route := row[0], row[1], row[2]
			if strings.HasPrefix(row[8], "tainted:") {
				continue
			}
			key := id + "/" + api
			_ = i
			if seen[key] {
				t.Fatalf("duplicate clean native row %s", key)
			}
			seen[key] = true
			accepted := row[3] == "True" || row[3] == "true"
			name := fmt.Sprintf("%s/%s:%d NATIVE %s %s %s", part, id, i+2, route, api, map[bool]string{true: "accepted", false: "rejected"}[accepted])
			t.Run(name, func(t *testing.T) {
				if accepted {
					if (route == "anonymous" && (row[4] != "True" || row[5] != "R=pass")) || (route == "istest" && (row[4] != "Passed" || row[5] != "run=Pass")) {
						t.Fatalf("unexpected native row: %q", row)
					}
				}
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
				var source string
				class := ""
				if route == "istest" {
					class = classes[id]
					b, err := os.ReadFile(filepath.Join(base, "force-app/main/default/classes", class+".cls"))
					if err != nil {
						t.Fatal(err)
					}
					writeFile(t, filepath.Join(root, "force-app/main/default/classes", class+".cls"), string(b))
					writeFile(t, filepath.Join(root, "force-app/main/default/classes", class+".cls-meta.xml"), fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api))
				} else {
					b, err := os.ReadFile(filepath.Join(base, classes[id]))
					if err != nil {
						t.Fatal(err)
					}
					source = string(b)
				}
				index := loadTestIndex(t, root)
				if route == "istest" {
					run := Run(index, Options{SelectedClasses: []string{class}, NoDiskCache: true, Parallelism: 1})
					s := run.Summary()
					if accepted && (s.Total != 1 || s.Passed != 1) {
						t.Fatalf("result: %+v; %s", s, firstRunProblem(run))
					}
					if !accepted && s.Passed != 0 {
						t.Fatalf("native rejected this class, Glade passed: %+v", s)
					}
					return
				}
				program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
				if err == nil {
					org := orgFromIndex(index)
					org.APIVersion = api
					runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
					_, err = runner.newMachine().Execute(program)
				}
				if accepted && err != nil {
					t.Fatal(err)
				}
				if !accepted && err == nil {
					t.Fatal("native rejected this source, Glade accepted it")
				}
			})
		}
	}
	if len(seen) != 100 {
		t.Fatalf("clean native cells: %d, want 100 (50 cases and routes, two APIs)", len(seen))
	}
}

func readTSV(path string) ([][]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	return r.ReadAll()
}
