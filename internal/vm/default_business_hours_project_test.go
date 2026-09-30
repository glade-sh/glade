package vm_test

import (
	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
	"os"
	"path/filepath"
	"testing"
)

// SF193 admits the original API65 indexed default BusinessHours query without test data.
func TestDefaultBusinessHoursAPI65Project(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"sfdx-project.json": `{"namespace": "", "sourceApiVersion": "65.0", "packageDirectories": [{"path": "force-app", "default": true}]}`,
		"force-app/main/default/classes/GladeDefaultBusinessHours65Proof.cls": `@IsTest private class GladeDefaultBusinessHours65Proof {
 @IsTest static void originalDefaultLookupNeedsNoInsertedFixture() {
  Id defaultBusinessHoursId=[SELECT Id FROM BusinessHours WHERE IsDefault=TRUE][0].Id;
  System.assertNotEquals(null,defaultBusinessHoursId,'original indexed default lookup');
  BusinessHours selected=[SELECT Id,IsDefault FROM BusinessHours WHERE Id=:defaultBusinessHoursId];
  System.assertEquals(true,selected.IsDefault,'selected row is default');
  System.assertEquals(defaultBusinessHoursId,selected.Id,'identity retained');
 }
}
`,
		"force-app/main/default/classes/GladeDefaultBusinessHours65Proof.cls-meta.xml": `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>65.0</apiVersion><status>Active</status></ApexClass>
`,
	}
	for name, content := range files {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	sch, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	report := apextest.Run(typesys.Build(p, sch), apextest.Options{NoDiskCache: true})
	if got := report.Summary(); got.Total != 1 || got.Passed != 1 || got.Errors != 0 {
		for _, suite := range report.Suites {
			for _, c := range suite.Cases {
				if c.Problem != nil {
					t.Logf("%s: %s", c.MethodName, c.Problem.Message)
				}
			}
		}
		t.Fatalf("default BusinessHours contract: %+v", got)
	}
}
