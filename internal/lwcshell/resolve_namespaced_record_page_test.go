package lwcshell

import (
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

// A namespaced package's record page names its object without the prefix
// (Widget__c). Navigation by record Id resolves the namespaced object
// (acme__Widget__c); both name the same object.
func TestResolvePageTargetRecordPageMatchesNamespacedObject(t *testing.T) {
	root := t.TempDir()
	pagePath := writeProjectFile(t, root, "force-app/main/default/flexipages/Widget_Record_Page.flexipage-meta.xml", `<FlexiPage xmlns="http://soap.sforce.com/2006/04/metadata">
  <masterLabel>Widget Record Page</masterLabel>
  <sobjectType>Widget__c</sobjectType>
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
	p := project.Project{Root: root, Namespace: "acme", FlexiPageFiles: []string{pagePath}}

	shell, diagnostics, err := ResolvePageTarget(p, PageContext{
		Kind:          RenderTargetRecordPage,
		RecordID:      "a0W000000000001AAA",
		ObjectAPIName: "acme__Widget__c",
	})
	if err != nil {
		t.Fatalf("ResolvePageTarget error = %v diagnostics=%#v", err, diagnostics)
	}
	if shell.Context.ObjectAPIName != "acme__Widget__c" || len(shell.Regions) != 1 {
		t.Fatalf("context = %#v regions = %#v", shell.Context, shell.Regions)
	}
	if !SameObjectInNamespace("Widget__c", "acme__widget__c", "acme") || SameObjectInNamespace("Widget__c", "other__Widget__c", "acme") {
		t.Fatal("namespace comparison")
	}
}
