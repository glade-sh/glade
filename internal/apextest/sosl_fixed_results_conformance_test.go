package apextest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// NATIVE fixed search results in async jobs, through a live list reference and
// across the term rules: sosl_fixed_results/org.tsv, istest 62.0 and 67.0, two
// scratch orgs with identical cells. The Apex classes under classes/ are the
// ones the native run deployed (renamed); each method reports its observation
// through a deliberate assertion message.
func TestSOSLFixedResultsConformance(t *testing.T) {
	const dir = "testdata/conformance/sosl_fixed_results"
	raw, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	type row struct {
		want string
		line int
	}
	rows := map[string]map[string]row{}
	for i, line := range strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")[1:] {
		f := strings.SplitN(line, "\t", 4)
		if len(f) != 4 {
			t.Fatalf("malformed row %q", line)
		}
		if rows[f[1]] == nil {
			rows[f[1]] = map[string]row{}
		}
		rows[f[1]][f[0]] = row{f[3], i + 2}
	}
	classes, err := filepath.Glob(filepath.Join(dir, "classes", "*.cls"))
	if err != nil || len(classes) != 5 {
		t.Fatalf("classes: %v %v", classes, err)
	}
	message := regexp.MustCompile(`(?s)FU\|(.*)\|END`)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			for _, class := range classes {
				body, err := os.ReadFile(class)
				if err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(root, "force-app/main/default/classes", filepath.Base(class))
				writeFile(t, path, string(body))
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			}
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			index := loadTestIndex(t, root)
			report := Run(index, Options{SelectedClasses: []string{"FixedSearchAsyncTest", "FixedSearchTermsTest"}, NoDiskCache: true, Parallelism: 1})
			seen := 0
			for _, suite := range report.Suites {
				for _, c := range suite.Cases {
					want, ok := rows[api][c.MethodName]
					if !ok {
						t.Errorf("%s has no native row", c.MethodName)
						continue
					}
					seen++
					if c.Problem == nil {
						t.Errorf("org.tsv:%d %s: expected the deliberate assertion, status %s", want.line, c.MethodName, c.Status)
						continue
					}
					m := message.FindStringSubmatch(c.Problem.Message)
					if m == nil {
						t.Errorf("org.tsv:%d %s: no observation in %q", want.line, c.MethodName, c.Problem.Message)
						continue
					}
					got := m[1]
					if strings.HasPrefix(want.want, "{") {
						var g, w map[string]string
						if json.Unmarshal([]byte(got), &g) != nil || json.Unmarshal([]byte(want.want), &w) != nil || !reflect.DeepEqual(g, w) {
							t.Errorf("org.tsv:%d %s: got %s, want %s", want.line, c.MethodName, got, want.want)
						}
						continue
					}
					got = strings.TrimPrefix(got, c.MethodName+"|")
					if got != want.want {
						t.Errorf("org.tsv:%d %s: got %q, want %q", want.line, c.MethodName, got, want.want)
					}
				}
			}
			if seen != len(rows[api]) {
				t.Errorf("ran %d methods, %d native rows", seen, len(rows[api]))
			}
		})
	}
}
