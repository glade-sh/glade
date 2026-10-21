package vm

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/resource"
)

// The Apex Reference Guide defines isCustom independently from getSobjectName.
// Imported component tabs retain their absent associated object while selecting
// the custom-tab branch in a consumer.
// https://resources.docs.salesforce.com/latest/latest/en-us/sfdc/pdf/salesforce_apex_reference_guide.pdf
func TestImportedComponentTabUsesCustomBranch(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "HarborDashboard.tab-meta.xml")
	appPath := filepath.Join(root, "Harbor.app-meta.xml")
	if err := os.WriteFile(path, []byte(`<CustomTab xmlns="http://soap.sforce.com/2006/04/metadata"><flexiPage>HarborDashboard</flexiPage><label>Harbor Dashboard</label><motif>Custom9: Lightning</motif></CustomTab>`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(appPath, []byte(`<CustomApplication xmlns="http://soap.sforce.com/2006/04/metadata"><label>Harbor</label><tabs>HarborDashboard</tabs></CustomApplication>`), 0o600); err != nil {
		t.Fatal(err)
	}
	org := testDataOrg()
	if err := resource.ApplyProject(&org, project.Project{Root: root, TabFiles: []string{path}, ApplicationFiles: []string{appPath}}); err != nil {
		t.Fatal(err)
	}
	if len(org.Metadata.Tabs) != 1 {
		t.Fatalf("imported custom tabs = %v", org.Metadata.Tabs)
	}
	machine := New(nil)
	machine.SetOrg(&org)
	program, err := CompileAnonymous(`
Boolean found=false;
for (Schema.DescribeTabSetResult app : Schema.describeTabs()) {
    for (Schema.DescribeTabResult tab : app.getTabs()) {
        if (tab.getName()=='HarborDashboard') {
            found=true;
            System.assertEquals(true,tab.isCustom());
            System.assertEquals(null,tab.getSobjectName());
        }
        if (!tab.isCustom()) {
            String associated=tab.getSobjectName().toLowerCase();
            System.assertNotEquals(null,associated);
        }
    }
}
System.assert(found,'Expected imported component tab');
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
