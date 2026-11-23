package soql_test

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

func TestChildJoinConjunctionNativeConformance(t *testing.T) {
	runChildJoinCapture(t, "testdata/child_join_conjunction", "combined", "CombinedProbe", "rows=1;contacts=1;opportunities=0")
}

func TestChildJoinWideNativeConformance(t *testing.T) {
	runChildJoinCapture(t, "testdata/child_join_wide", "wide", "WideProbe", "rows=1;contacts=1;name=Hit")
}

func TestChildJoinStaticBindNativeConformance(t *testing.T) {
	runChildJoinCapture(t, "testdata/child_join_bound", "static", "StaticBindProbe", "rows=1;contacts=1;name=Hit")
}

func TestChildJoinDynamicBindNativeConformance(t *testing.T) {
	runChildJoinCapture(t, "testdata/child_join_bound", "dynamic", "DynamicBindProbe", "rows=1;contacts=1;name=Hit")
}

func runChildJoinCapture(t *testing.T, fixture, stem, class, want string) {
	t.Helper()
	f, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	caseFile, err := os.Open(filepath.Join(fixture, "cases.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer caseFile.Close()
	caseReader := csv.NewReader(caseFile)
	caseReader.Comma = '\t'
	cases, err := caseReader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) == 0 {
		t.Fatal("empty case table")
	}
	validCases := make(map[string]bool, len(cases)-1)
	for _, c := range cases[1:] {
		validCases[c[0]] = true
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		if len(row) != 9 {
			t.Fatalf("invalid native row: %v", row)
		}
		id, api, route := row[0], row[1], row[2]
		if !validCases[id] {
			t.Fatalf("unexpected case: %s", id)
		}
		if id != stem+"_anon" && id != stem+"_test" {
			continue
		}
		if !strings.EqualFold(row[3], "true") || (id == stem+"_anon" && (route != "anonymous" || !strings.EqualFold(row[4], "true"))) || (id == stem+"_test" && (route != "istest" || row[4] != "Failed" || row[6] != api)) {
			t.Fatalf("invalid route or execution status: %v", row)
		}
		key := api + "/" + route
		if seen[key] {
			t.Fatalf("duplicate native row %s", key)
		}
		seen[key] = true
		if !strings.Contains(row[5], "OBS:"+want) || strings.HasPrefix(row[8], "tainted:") {
			t.Fatalf("invalid native outcome: %v", row)
		}
		t.Run(key, func(t *testing.T) {
			root := t.TempDir()
			write := func(name string, b []byte) {
				t.Helper()
				p := filepath.Join(root, name)
				if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, b, 0644); err != nil {
					t.Fatal(err)
				}
			}
			write("sfdx-project.json", []byte(fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api)))
			cls, err := os.ReadFile(filepath.Join(fixture, "force-app", "main", "default", "classes", class+".cls"))
			if err != nil {
				t.Fatal(err)
			}
			write("force-app/main/default/classes/"+class+".cls", cls)
			write("force-app/main/default/classes/"+class+".cls-meta.xml", []byte(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`))
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			s, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			index := typesys.Build(p, s)
			if index.HasErrors() {
				t.Fatalf("index: %v", index.Diagnostics)
			}
			if route == "istest" {
				report := apextest.Run(index, apextest.Options{SelectedClasses: []string{class}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
				summary := report.Summary()
				var observations []string
				for _, suite := range report.Suites {
					for _, test := range suite.Cases {
						if test.Problem != nil {
							observations = append(observations, test.Problem.Message)
							if summary.Total == 1 && summary.Failed == 1 && strings.Contains(test.Problem.Message, "OBS:"+want) {
								return
							}
						}
					}
				}
				t.Fatalf("missing native observation: summary=%+v problems=%v", summary, observations)
				return
			}
			if route != "anonymous" {
				t.Fatalf("unknown route: %s", route)
			}
			source, err := os.ReadFile(filepath.Join(fixture, stem+".apex"))
			if err != nil {
				t.Fatal(err)
			}
			runtime, err := apextest.CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
			if err != nil {
				t.Fatal(err)
			}
			machine := vm.New(nil)
			if err := apextest.RegisterCompiledProjectRuntimeForRequest(machine, runtime); err != nil {
				t.Fatal(err)
			}
			org := apextest.OrgFromIndex(index)
			org.APIVersion = api
			machine.SetOrg(&org)
			program, err := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			result, err := machine.Execute(program)
			if err != nil {
				t.Fatal(err)
			}
			for _, line := range result.Debug {
				if strings.Contains(line, "OBS:"+want) {
					return
				}
			}
			t.Fatalf("missing native observation: %v", result.Debug)
		})
	}
	if len(seen) != 4 {
		t.Fatalf("expected four native route/version rows, got %v", seen)
	}
	for _, key := range []string{"62.0/anonymous", "67.0/anonymous", "62.0/istest", "67.0/istest"} {
		if !seen[key] {
			t.Fatalf("missing native route/version row %s", key)
		}
	}
}

func TestChildJoinConjunctionNeighbor(t *testing.T) {
	for name, query := range map[string]string{
		"offset":      "SELECT Id, (SELECT Id FROM Contacts WHERE LastName != null AND Id IN (SELECT ContactId FROM Case WHERE ContactId != null) LIMIT 1 OFFSET 1), (SELECT Id FROM Opportunities LIMIT 1) FROM Account",
		"parent_join": "SELECT Id, (SELECT Id FROM Contacts WHERE LastName != null AND Id IN (SELECT ContactId FROM Case WHERE ContactId != null) LIMIT 1), (SELECT Id FROM Opportunities LIMIT 1) FROM Account WHERE Id IN (SELECT AccountId FROM Case)",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := soql.Parse(query)
			if err == nil || !strings.Contains(err.Error(), "semi-join and anti-join subqueries are allowed only in the main WHERE clause") {
				t.Fatalf("uncaptured neighbor changed: %v", err)
			}
		})
	}
}

// The three shapes below were uncaptured-neighbor pins on the earlier narrow
// rule. Rows internal/apextest/testdata/conformance/child_semijoin_matrix:
// a/org.tsv sib0_conj_limit1 (a child alone with LIMIT 1), c/org.tsv neqtext
// (text !=) and bool (boolean =) show Salesforce accepts them at 62.0 and 67.0.
func TestChildJoinConjunctionCapturedNeighborsAccepted(t *testing.T) {
	for name, query := range map[string]string{
		"single_child":   "SELECT Id, (SELECT Id FROM Contacts WHERE LastName != null AND Id IN (SELECT ContactId FROM Case WHERE ContactId != null) LIMIT 1) FROM Account",
		"not_equal_text": "SELECT Id, (SELECT Id FROM Contacts WHERE LastName != 'Missing' AND Id IN (SELECT ContactId FROM Case WHERE ContactId != null) LIMIT 1), (SELECT Id FROM Opportunities LIMIT 1) FROM Account",
		"boolean_equal":  "SELECT Id, (SELECT Id FROM Contacts WHERE HasOptedOutOfEmail = true AND Id IN (SELECT ContactId FROM Case WHERE ContactId != null) LIMIT 1), (SELECT Id FROM Opportunities LIMIT 1) FROM Account",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := soql.Parse(query); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// Scalar operands with no captured row keep origin/main's rejection.
func TestChildJoinConjunctionUncapturedScalarsRejected(t *testing.T) {
	const join = "Id IN (SELECT ContactId FROM Case WHERE ContactId != null)"
	for name, where := range map[string]string{
		"equals_null": "LastName = null AND " + join,
		"integer":     "NumberOfEmployees = 5 AND " + join,
		"less_than":   "LastName < 'M' AND " + join,
		"four_terms":  "LastName != null AND FirstName != null AND Email != null AND " + join,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := soql.Parse("SELECT Id, (SELECT Id FROM Contacts WHERE " + where + " LIMIT 1), (SELECT Id FROM Opportunities LIMIT 1) FROM Account")
			if err == nil || !strings.Contains(err.Error(), "semi-join and anti-join subqueries are allowed only in the main WHERE clause") {
				t.Fatalf("uncaptured scalar changed: %v", err)
			}
		})
	}
}

func BenchmarkChildJoinControl(b *testing.B) {
	for _, tc := range []struct{ name, query, rejection string }{
		{"accepted", "SELECT Id, (SELECT Id FROM Contacts WHERE Id IN (SELECT ContactId FROM Case WHERE ContactId != null)) FROM Account", ""},
		{"neighbor", "SELECT Id, (SELECT Id FROM Contacts WHERE LastName != null AND Id IN (SELECT ContactId FROM Case WHERE ContactId != null) LIMIT 1 OFFSET 1), (SELECT Id FROM Opportunities LIMIT 1) FROM Account", "soql: semi-join and anti-join subqueries are allowed only in the main WHERE clause"},
	} {
		b.Run(tc.name, func(b *testing.B) {
			parsed, err := soql.Parse(tc.query)
			var outcome []byte
			if tc.rejection != "" {
				if err == nil || err.Error() != tc.rejection {
					b.Fatalf("control rejection: %v", err)
				}
				outcome = []byte(err.Error())
			} else {
				if err != nil || len(parsed.ChildQueries) != 1 || parsed.Object != "Account" {
					b.Fatalf("control outcome: %v %v", parsed.Object, err)
				}
				outcome, err = json.Marshal(parsed)
				if err != nil {
					b.Fatal(err)
				}
			}
			b.Logf("control_sha256=%x", sha256.Sum256(outcome))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				parsed, err := soql.Parse(tc.query)
				if tc.rejection != "" {
					if err == nil || err.Error() != tc.rejection {
						b.Fatal("control rejection changed")
					}
				} else if err != nil || len(parsed.ChildQueries) != 1 || parsed.Object != "Account" {
					b.Fatal("control outcome changed")
				}
			}
		})
	}
}
