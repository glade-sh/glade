package lwcshell

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestResolvePageTargetRecordPage(t *testing.T) {
	root := t.TempDir()
	pagePath := writeProjectFile(t, root, "force-app/main/default/flexipages/Account_Record_Page.flexipage-meta.xml", `<FlexiPage xmlns="http://soap.sforce.com/2006/04/metadata">
  <masterLabel>Account Record Page</masterLabel>
  <sobjectType>Account</sobjectType>
  <type>RecordPage</type>
  <flexiPageRegions>
    <name>main</name>
    <itemInstances>
      <componentInstance>
        <componentName>c:contextProbe</componentName>
      </componentInstance>
    </itemInstances>
  </flexiPageRegions>
</FlexiPage>`)
	p := project.Project{Root: root, FlexiPageFiles: []string{pagePath}}

	shell, diagnostics, err := ResolvePageTarget(p, PageContext{
		Kind:          RenderTargetRecordPage,
		PageName:      "Account_Record_Page",
		RecordID:      "001000000000001AAA",
		ObjectAPIName: "Account",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
	if shell.Context.RecordID != "001000000000001AAA" || shell.Context.ObjectAPIName != "Account" {
		t.Fatalf("context = %#v", shell.Context)
	}
	if len(shell.Regions) != 1 || len(shell.Regions[0].Components) != 1 {
		t.Fatalf("shell regions = %#v", shell.Regions)
	}
}

func TestResolvePageTargetRecordPageKeepsRouteObjectWhenMetadataOmitsSObject(t *testing.T) {
	root := t.TempDir()
	pagePath := writeProjectFile(t, root, "force-app/main/default/flexipages/Account_Record_Page.flexipage-meta.xml", `<FlexiPage xmlns="http://soap.sforce.com/2006/04/metadata">
  <masterLabel>Account Record Page</masterLabel>
  <type>RecordPage</type>
  <flexiPageRegions>
    <name>main</name>
    <componentInstances>
      <componentName>c:contextProbe</componentName>
    </componentInstances>
  </flexiPageRegions>
</FlexiPage>`)
	p := project.Project{Root: root, FlexiPageFiles: []string{pagePath}}

	shell, diagnostics, err := ResolvePageTarget(p, PageContext{
		Kind:          RenderTargetRecordPage,
		PageName:      "Account_Record_Page",
		RecordID:      "001000000000001AAA",
		ObjectAPIName: "Account",
	})
	if err != nil {
		t.Fatalf("ResolvePageTarget error = %v diagnostics=%#v", err, diagnostics)
	}
	if shell.Context.ObjectAPIName != "Account" {
		t.Fatalf("objectApiName = %q", shell.Context.ObjectAPIName)
	}
}

func TestResolvePageTargetQualifiesUnnamespacedFlexiPageComponents(t *testing.T) {
	root := t.TempDir()
	pagePath := writeProjectFile(t, root, "force-app/main/default/flexipages/Account_Record_Page.flexipage-meta.xml", `<FlexiPage xmlns="http://soap.sforce.com/2006/04/metadata">
  <sobjectType>Account</sobjectType>
  <type>RecordPage</type>
  <flexiPageRegions>
    <name>main</name>
    <componentInstances>
      <componentName>contextProbe</componentName>
    </componentInstances>
  </flexiPageRegions>
</FlexiPage>`)
	p := project.Project{Root: root, Namespace: "pkg", FlexiPageFiles: []string{pagePath}}

	shell, _, err := ResolvePageTarget(p, PageContext{
		Kind:          RenderTargetRecordPage,
		PageName:      "Account_Record_Page",
		ObjectAPIName: "Account",
	})
	if err != nil {
		t.Fatal(err)
	}
	got := shell.Regions[0].Components[0].ComponentName
	if got != "pkg:contextProbe" {
		t.Fatalf("component = %q", got)
	}
}

func TestResolvePageTargetAllowsVisualforceCustomTab(t *testing.T) {
	root := t.TempDir()
	tabPath := writeProjectFile(t, root, "force-app/main/default/tabs/Legacy_VF.tab-meta.xml", `<CustomTab xmlns="http://soap.sforce.com/2006/04/metadata">
  <label>Legacy VF</label>
  <page>LegacyPage</page>
</CustomTab>`)
	p := project.Project{Root: root, TabFiles: []string{tabPath}}

	shell, diagnostics, err := ResolvePageTarget(p, PageContext{
		Kind:    RenderTargetTab,
		TabName: "Legacy_VF",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
	if shell.Tab.Type != TabTypeVisualforce || shell.Tab.Target != "LegacyPage" {
		t.Fatalf("tab = %#v", shell.Tab)
	}
}

func writeProjectFile(t *testing.T, root string, rel string, body string) string {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
