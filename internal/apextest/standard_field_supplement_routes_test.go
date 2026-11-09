package apextest

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// Native field membership and describe facets are recorded in
// standard_field_supplement/org.tsv. The SOQL assertion is a local route
// regression. The separate QueryNativeRoutes test compares direct Salesforce
// SOQL observations against the anonymous and test-runner routes.
func TestStandardFieldSupplementRoutes(t *testing.T) {
	body := `System.assert(Schema.SObjectType.Lead.fields.getMap().containsKey('LastTransferDate'));
System.assertEquals(Schema.DisplayType.Date, Schema.SObjectType.Lead.fields.getMap().get('LastTransferDate').getDescribe().getType());
List<Lead> found = [SELECT LastTransferDate FROM Lead LIMIT 1];
System.assertEquals(0, found.size());`
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/FieldSupplementTest.cls"), "@IsTest private class FieldSupplementTest { @IsTest static void checkFields() {\n"+body+"\n} }")
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/FieldSupplementTest.cls-meta.xml"), fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api))
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("project diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := runner.newMachine().Execute(program); err != nil {
				t.Fatalf("anonymous: %v", err)
			}
			run := Run(index, Options{SelectedClasses: []string{"FieldSupplementTest"}, SelectedMethod: "checkFields", NoDiskCache: true, Parallelism: 1})
			if run.Summary().Total != 1 || len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem != nil {
				t.Fatalf("project test: %+v", run)
			}
		})
	}
}
