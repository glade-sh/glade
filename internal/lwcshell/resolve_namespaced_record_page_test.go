package lwcshell

import (
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

// Source in a namespaced project names its own objects without the prefix: a
// record page declares <sobjectType>Widget__c</sobjectType>, while navigation
// by record Id carries the namespaced object acme__Widget__c. Both name the
// same object. Another namespace's object, or a project without that
// namespace, still does not match.
func TestResolvePageTargetRecordPageMatchesOwnNamespaceObject(t *testing.T) {
	cases := []struct {
		name      string
		namespace string
		object    string
		wantError bool
	}{
		{name: "own namespace accepts", namespace: "acme", object: "acme__Widget__c"},
		{name: "own namespace any case accepts", namespace: "acme", object: "ACME__widget__c"},
		{name: "unprefixed boundary accepts", namespace: "acme", object: "Widget__c"},
		{name: "other namespace rejects", namespace: "acme", object: "other__Widget__c", wantError: true},
		{name: "no project namespace rejects", namespace: "", object: "acme__Widget__c", wantError: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
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
			p := project.Project{Root: root, Namespace: tc.namespace, FlexiPageFiles: []string{pagePath}}

			shell, diagnostics, err := ResolvePageTarget(p, PageContext{
				Kind:          RenderTargetRecordPage,
				RecordID:      "a0W00000001AAAAAAA",
				ObjectAPIName: tc.object,
				PageName:      "Widget_Record_Page",
			})
			if tc.wantError {
				if err == nil || len(diagnostics) != 1 || diagnostics[0].Code != "GLADELWC004" {
					t.Fatalf("err = %v diagnostics = %#v, want GLADELWC004", err, diagnostics)
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolvePageTarget error = %v diagnostics = %#v", err, diagnostics)
			}
			if shell.Context.ObjectAPIName != tc.object || len(shell.Regions) != 1 {
				t.Fatalf("context object = %q regions = %d, want %q and 1 region", shell.Context.ObjectAPIName, len(shell.Regions), tc.object)
			}
		})
	}
}
