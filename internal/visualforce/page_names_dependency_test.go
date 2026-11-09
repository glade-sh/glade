package visualforce_test

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// A dependency page that fails local validation still exists in the package.
// While the consumer has any loadable page, Page.ns__Name for that page must
// resolve, as it does for its loadable siblings.
func TestDependencyPageNamesIncludePagesThatFailLocalValidation(t *testing.T) {
	for _, api := range []string{"59.0", "62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			parent := t.TempDir()
			dep := filepath.Join(parent, "dep")
			app := filepath.Join(parent, "app")
			meta := `{"packageDirectories":[{"path":"src","default":true}],"sourceApiVersion":"` + api + `","namespace":"%s"}`
			writePageNameFixture(t, dep, "sfdx-project.json", `{"packageDirectories":[{"path":"src","default":true}],"sourceApiVersion":"`+api+`","namespace":"gladedep"}`)
			writePageNameFixture(t, dep, "src/classes/DepCtl.cls", `global class DepCtl { global String msg { get { return 'hi'; } } }`)
			writePageNameFixture(t, dep, "src/pages/Broken.page", `<apex:page controller="DepCtl">{!missing}</apex:page>`)
			writePageNameFixture(t, dep, "src/pages/Plain.page", `<apex:page>x</apex:page>`)
			_ = meta
			writePageNameFixture(t, app, "sfdx-project.json", `{"packageDirectories":[{"path":"src","default":true}],"sourceApiVersion":"`+api+`","namespace":"gladeown"}`)
			writePageNameFixture(t, app, "glade.yml", "project:\n  managedPackageDependencies: [\"gladedep:../dep\"]\n")
			writePageNameFixture(t, app, "src/pages/Own.page", `<apex:page>own</apex:page>`)

			var index typesys.Index
			index.Project.Root = app
			runtime, err := apextest.CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
			if err != nil {
				t.Fatal(err)
			}
			machine := vm.New(nil)
			if err := apextest.RegisterCompiledProjectRuntimeForRequest(machine, runtime); err != nil {
				t.Fatal(err)
			}
			for _, expr := range []string{
				"Page.gladedep__Broken.getUrl()",
				"Page.gladedep__Plain.getUrl()",
				"Page.gladeown__Own.getUrl()",
				"Page.Own.getUrl()",
			} {
				program, err := vm.CompileAnonymousWithOptions("System.assertNotEquals(null, "+expr+");", vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := machine.Execute(program); err != nil {
					t.Errorf("%s: %v", expr, err)
				}
			}
		})
	}
}
