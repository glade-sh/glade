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

// SF196 admits the original spaced comparison at API62/project40.
func TestSpacedComparisonAPI62Project40(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"sfdx-project.json": `{"namespace": "", "sourceApiVersion": "40.0", "packageDirectories": [{"path": "force-app", "default": true}]}`,
		"force-app/main/default/classes/GladeSpacedComparison62Proof.cls": `@IsTest private class GladeSpacedComparison62Proof {
 private static Id extractErrorTarget(String strErr) {
  Id objId=null;
  Integer ich=strErr.lastIndexOf(' for id : ');
  if (ich > = 0) {
   objId=strErr.substring(ich+10);
  }
  return objId;
 }
 @IsTest static void spacedComparisonKeepsOriginalExtractionBranches() {
  Id expected='003000000000001';
  System.assertEquals(expected,extractErrorTarget(' for id : 003000000000001'),'zero index must satisfy comparison');
  System.assertEquals(expected,extractErrorTarget('failure for id : 003000000000001'),'positive index');
  System.assertEquals(null,extractErrorTarget('unmatched failure'),'negative index must skip assignment');
 }
}
`,
		"force-app/main/default/classes/GladeSpacedComparison62Proof.cls-meta.xml": `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>
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
		t.Fatalf("spaced comparison contract: %+v", got)
	}
}
