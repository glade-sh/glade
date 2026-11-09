package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func TestB3ResponseReviewBoundariesNativeRoutes(t *testing.T) {
	const fixture = "testdata/conformance/b3_auto_response"
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "istest"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				dir := filepath.Join(fixture, "review-boundaries", "api"+strings.TrimSuffix(api, ".0"), route)
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
				if len(observations) != 3 {
					t.Fatal("incomplete native capture")
				}
				// The third captured row uses a malformed ID. Its status-code
				// mismatch is independent of response headers; valid identity
				// failures are covered by the separate alignment capture.
				expected := strings.Join(observations[:2], "|")
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
				for _, spec := range []struct{ source, dest string }{
					{"B3Review.cls", "classes/B3Review.cls"},
					{"B3ReviewTrigger.trigger", "triggers/B3ReviewTrigger.trigger"},
				} {
					body, err := os.ReadFile(filepath.Join(fixture, "review-boundaries", "source", spec.source))
					if err != nil {
						t.Fatal(err)
					}
					path := filepath.Join(root, "force-app/main/default", spec.dest)
					writeFile(t, path, string(body))
					kind := "ApexClass"
					if strings.HasSuffix(spec.dest, ".trigger") {
						kind = "ApexTrigger"
					}
					writeFile(t, path+"-meta.xml", `<`+kind+` xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></`+kind+`>`)
				}
				code := `String actual=B3Review.run();System.assertEquals('` + expected + `',actual.substring(0,actual.indexOf('|partial=')));`
				cls := filepath.Join(root, "force-app/main/default/classes/B3ReviewAssertion.cls")
				writeFile(t, cls, `@IsTest private class B3ReviewAssertion { @IsTest static void check() { `+code+` } }`)
				writeFile(t, cls+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
				index := loadTestIndex(t, root)
				if route == "istest" {
					report := Run(index, Options{SelectedClasses: []string{"B3ReviewAssertion"}, NoDiskCache: true, Parallelism: 1})
					if report.Summary().Total != 1 || report.Summary().Passed != 1 {
						t.Fatalf("native %s: %+v", expected, report)
					}
					return
				}
				org := orgFromIndex(index)
				org.APIVersion = api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				triggers, diagnostics := compileProjectTriggers(index)
				if len(diagnostics) > 0 {
					t.Fatal(diagnostics)
				}
				for _, trigger := range triggers {
					if err := runner.base.RegisterTrigger(trigger); err != nil {
						t.Fatal(err)
					}
				}
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
