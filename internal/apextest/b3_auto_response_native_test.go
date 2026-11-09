package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func TestB3AutoResponseNativeRoutes(t *testing.T) {
	const fixture = "testdata/conformance/b3_auto_response"
	for _, object := range []string{"lead", "case"} {
		for _, state := range []string{"active", "inactive"} {
			for _, api := range []string{"62.0", "67.0"} {
				for _, route := range []string{"anonymous", "istest"} {
					t.Run(object+"/"+state+"/"+api+"/"+route, func(t *testing.T) {
						captureRoot := fixture
						if object == "case" {
							captureRoot = filepath.Join(fixture, "case")
						}
						capture := filepath.Join(captureRoot, state, "api"+strings.TrimSuffix(api, ".0"), route)
						rows, err := os.ReadFile(filepath.Join(capture, "org.tsv"))
						if err != nil {
							t.Fatal(err)
						}
						facts, err := os.ReadFile(filepath.Join(capture, "RUN-FACTS.txt"))
						if err != nil || !strings.Contains(string(facts), "native_complete=true") {
							t.Fatalf("missing capture %v", err)
						}
						var observations []string
						for _, line := range strings.Split(strings.TrimSpace(string(rows)), "\n")[1:] {
							key, value, ok := strings.Cut(line, "\t")
							if !ok {
								t.Fatal(line)
							}
							observations = append(observations, key+"="+value)
						}
						if len(observations) != 3 {
							t.Fatal("incomplete native observations")
						}
						expected := strings.Join(observations, "|")
						root := t.TempDir()
						err = filepath.Walk(filepath.Join(fixture, "project"), func(path string, info os.FileInfo, walkErr error) error {
							if walkErr != nil || info.IsDir() {
								return walkErr
							}
							rel, e := filepath.Rel(filepath.Join(fixture, "project"), path)
							if e != nil {
								return e
							}
							data, e := os.ReadFile(path)
							if e != nil {
								return e
							}
							s := strings.ReplaceAll(strings.ReplaceAll(string(data), "62.0", api), "67.0", api)
							if state == "inactive" && strings.Contains(rel, "autoResponseRules") {
								s = strings.ReplaceAll(s, "<active>true</active>", "<active>false</active>")
							}
							writeFile(t, filepath.Join(root, rel), s)
							return nil
						})
						if err != nil {
							t.Fatal(err)
						}
						probe := "B3RuleProbe"
						if object == "case" {
							probe = "B3CaseProbe"
						}
						code := `System.assertEquals('` + expected + `',` + probe + `.run());`
						cls := filepath.Join(root, "force-app/main/default/classes/B3RuleAssertion.cls")
						writeFile(t, cls, `@IsTest private class B3RuleAssertion { @IsTest static void check() { `+code+` } }`)
						writeFile(t, cls+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
						index := loadTestIndex(t, root)
						if route == "istest" {
							report := Run(index, Options{SelectedClasses: []string{"B3RuleAssertion"}, NoDiskCache: true, Parallelism: 1})
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
	}
}
