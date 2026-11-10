package apextest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
)

// The subscriber capture shows that installed, unnamespaced package inner
// classes and interfaces are valid local types at APIs 62 and 67. The offline
// project indexes the captured provider while a separate package alias is unavailable.
func TestUnlockedSubscriberProjectRunnerRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, declared := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/declared=%v", api, declared), func(t *testing.T) {
				root := t.TempDir()
				dependency := ""
				if declared {
					dependency = `,"dependencies":[{"package":"v036-unlocked"}]`
				}
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true`+dependency+`}]}`)
				if declared {
					provider, err := os.ReadFile(filepath.Join("testdata", "conformance", "namespace_unlocked_subscriber", "provider", "package", "main", "default", "classes", "V036PkgOuter.cls"))
					if err != nil {
						t.Fatal(err)
					}
					path := filepath.Join(root, "force-app/main/default/classes/V036PkgOuter.cls")
					writeFile(t, path, string(provider))
					writeFile(t, path+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
				}
				class := `@IsTest private class UnlockedRouteV036Test {
    @IsTest static void run() {
        if (false) { V036PkgOuter.Box localBox; V036PkgOuter.Shape localShape; }
        System.assertEquals(7, 7);
    }
}`
				path := filepath.Join(root, "force-app/main/default/classes/UnlockedRouteV036Test.cls")
				writeFile(t, path, class)
				writeFile(t, path+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
				index := loadTestIndex(t, root)
				var errs []string
				for _, d := range sema.Analyze(index).Diagnostics {
					if d.Severity == diagnostic.Error && strings.Contains(d.Message, "V036PkgOuter") {
						errs = append(errs, d.Message)
					}
				}
				if declared && len(errs) != 0 {
					t.Fatalf("indexed package member rejected: %v", errs)
				}
				if !declared && len(errs) == 0 {
					t.Fatal("no-package native control must reject missing types")
				}
				for _, warm := range []bool{false, true} {
					run := Run(index, Options{SelectedClasses: []string{"UnlockedRouteV036Test"}, NoDiskCache: !warm, Parallelism: 1})
					if declared && run.Summary().Passed != 1 {
						t.Fatalf("warm=%v declared: %+v %s", warm, run.Summary(), firstRunProblem(run))
					}
					if !declared && run.Summary().CompileErrors == 0 {
						t.Fatalf("warm=%v absent package: %+v", warm, run.Summary())
					}
				}
			})
		}
	}
}
