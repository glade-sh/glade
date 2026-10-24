package apextest

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// numeric_routes/org.tsv, grapheme-boundary captures this delimiter at API
// 62/67 through anonymous and named tests; keep both split entry points.
func TestLiteralSplitGraphemePlanProjectAnonymousAndIsTestRoutes(t *testing.T) {
	prefix := `String mark=String.fromCharArray(new List<Integer>{769});
 String text='e'+mark;
 String separator='e'+String.fromCharArray(new List<Integer>{92})+'b{g}';
 System.assertEquals(false,Pattern.compile(separator).matcher(text).find());`
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "isTest"} {
			for _, split := range []string{"String.split", "Pattern.split"} {
				t.Run(api+"/"+route+"/"+split, func(t *testing.T) {
					expression := `text.split(separator,-1)`
					if split == "Pattern.split" {
						expression = `Pattern.compile(separator).split(text,-1)`
					}
					body := prefix + `System.assertEquals(new List<String>{text},` + expression + `);`
					root := t.TempDir()
					writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
					classes := filepath.Join(root, "force-app/main/default/classes")
					writeFile(t, filepath.Join(classes, "GraphemeSplitProbeTest.cls"), `@IsTest private class GraphemeSplitProbeTest { @IsTest static void verify() {`+body+`} }`)
					writeFile(t, filepath.Join(classes, "GraphemeSplitProbeTest.cls-meta.xml"), `<ApexClass><apiVersion>`+api+`</apiVersion></ApexClass>`)
					index := loadTestIndex(t, root)
					if route == "anonymous" {
						program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true})
						if _, err := runner.execute(program); err != nil {
							t.Fatal(err)
						}
					} else {
						run := Run(index, Options{SelectedClasses: []string{"GraphemeSplitProbeTest"}, SelectedMethod: "verify", NoDiskCache: true, Parallelism: 1})
						if s := run.Summary(); s.Total != 1 || s.Passed != 1 {
							t.Fatalf("summary=%+v problem=%s", s, firstRunProblem(run))
						}
					}
				})
			}
		}
	}
}
