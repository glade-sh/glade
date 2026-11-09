package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func TestB3ResponseCriteriaNativeRoutes(t *testing.T) {
	const fixture = "testdata/conformance/b3_auto_response"
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "istest"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				dir := filepath.Join(fixture, "review-criteria", "api"+strings.TrimSuffix(api, ".0"), route)
				raw, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
				if err != nil {
					t.Fatal(err)
				}
				var observations []string
				for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
					k, v, ok := strings.Cut(line, "\t")
					if !ok {
						t.Fatal(line)
					}
					observations = append(observations, k+"="+v)
				}
				if len(observations) != 4 {
					t.Fatal("incomplete native capture")
				}
				expected := strings.Join(observations, "|")
				root := t.TempDir()
				err = filepath.Walk(filepath.Join(fixture, "project"), func(path string, info os.FileInfo, e error) error {
					if e != nil || info.IsDir() {
						return e
					}
					rel, e := filepath.Rel(filepath.Join(fixture, "project"), path)
					if e != nil {
						return e
					}
					b, e := os.ReadFile(path)
					if e != nil {
						return e
					}
					s := strings.ReplaceAll(strings.ReplaceAll(string(b), "62.0", api), "67.0", api)
					writeFile(t, filepath.Join(root, rel), s)
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
				extra := filepath.Join(fixture, "review-criteria", "metadata")
				if err := filepath.Walk(extra, func(path string, info os.FileInfo, e error) error {
					if e != nil || info.IsDir() {
						return e
					}
					rel, e := filepath.Rel(extra, path)
					if e != nil {
						return e
					}
					body, e := os.ReadFile(path)
					if e != nil {
						return e
					}
					writeFile(t, filepath.Join(root, "force-app/main/default", rel), string(body))
					return nil
				}); err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(root, "force-app/main/default/classes/B3Criteria.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
				code := `System.assertEquals('` + expected + `',B3Criteria.run());`
				cls := filepath.Join(root, "force-app/main/default/classes/B3CriteriaAssertion.cls")
				writeFile(t, cls, `@IsTest private class B3CriteriaAssertion { @IsTest static void check() { `+code+` } }`)
				writeFile(t, cls+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
				index := loadTestIndex(t, root)
				if route == "istest" {
					report := Run(index, Options{SelectedClasses: []string{"B3CriteriaAssertion"}, NoDiskCache: true, Parallelism: 1})
					if report.Summary().Total != 1 || report.Summary().Passed != 1 {
						t.Fatalf("native %s: %+v", expected, report)
					}
					return
				}
				org := orgFromIndex(index)
				org.APIVersion = api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				program, err := vm.CompileAnonymousWithOptions(code, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err = runner.execute(program); err != nil {
					t.Fatalf("native %s: %v", expected, err)
				}
			})
		}
	}
}
