package lwcshell

import (
	"os"
	"path/filepath"
	"strings"
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

func TestResolvePageTargetReportsVisibilityRuleApproximation(t *testing.T) {
	root := t.TempDir()
	pagePath := writeProjectFile(t, root, "force-app/main/default/flexipages/Account_Record_Page.flexipage-meta.xml", `<FlexiPage xmlns="http://soap.sforce.com/2006/04/metadata">
  <sobjectType>Account</sobjectType>
  <type>RecordPage</type>
  <flexiPageRegions>
    <name>main</name>
    <componentInstances>
      <componentName>contextProbe</componentName>
      <visibilityRule>
        <booleanFilter>1 AND 2</booleanFilter>
        <criteria>
          <leftValue>{!Record.Status__c}</leftValue>
          <operator>EQUAL</operator>
          <rightValue>Active</rightValue>
        </criteria>
        <criteria>
          <leftValue>{!$Permission.CustomPermission.MemberAccess}</leftValue>
          <operator>EQUAL</operator>
          <rightValue>true</rightValue>
        </criteria>
      </visibilityRule>
    </componentInstances>
  </flexiPageRegions>
</FlexiPage>`)
	p := project.Project{Root: root, Namespace: "c", FlexiPageFiles: []string{pagePath}}

	shell, diagnostics, err := ResolvePageTarget(p, PageContext{
		Kind:          RenderTargetRecordPage,
		PageName:      "Account_Record_Page",
		ObjectAPIName: "Account",
	})
	if err != nil {
		t.Fatalf("ResolvePageTarget error = %v diagnostics=%#v", err, diagnostics)
	}
	if len(shell.Regions) != 1 || len(shell.Regions[0].Components) != 1 {
		t.Fatalf("regions = %#v", shell.Regions)
	}
	if len(diagnostics) != 1 || diagnostics[0].Code != "GLADELWC034" {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
	if diagnostics[0].Message == "" || !strings.Contains(diagnostics[0].Message, "c:contextProbe") || !strings.Contains(diagnostics[0].Message, "visibility") {
		t.Fatalf("diagnostic message = %#v", diagnostics[0])
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

func TestResolvePageTargetNormalizesCustomTabLabelToAPIName(t *testing.T) {
	root := t.TempDir()
	tabPath := writeProjectFile(t, root, "force-app/main/default/tabs/My_Custom_Tab.tab-meta.xml", `<CustomTab xmlns="http://soap.sforce.com/2006/04/metadata">
  <label>My Custom Tab</label>
  <flexiPage>Sales_Dashboard</flexiPage>
</CustomTab>`)
	p := project.Project{Root: root, TabFiles: []string{tabPath}}

	shell, diagnostics, err := ResolvePageTarget(p, PageContext{
		Kind:    RenderTargetTab,
		TabName: "My Custom Tab",
	})
	if err != nil {
		t.Fatalf("ResolvePageTarget error = %v diagnostics=%#v", err, diagnostics)
	}
	if shell.Tab.Name != "My_Custom_Tab" || shell.Tab.Target != "Sales_Dashboard" {
		t.Fatalf("tab = %#v", shell.Tab)
	}
}

func TestResolvePageTargetComponentUsesUrlAddressableMetadata(t *testing.T) {
	root := t.TempDir()
	metaPath := writeProjectFile(t, root, "force-app/main/default/lwc/urlProbe/urlProbe.js-meta.xml", `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata">
  <isExposed>true</isExposed>
  <targets>
    <target>lightning__UrlAddressable</target>
  </targets>
</LightningComponentBundle>`)
	p := project.Project{Root: root, LWCMetaFiles: []string{metaPath}}

	shell, diagnostics, err := ResolvePageTarget(p, PageContext{
		Kind:          RenderTargetComponent,
		ComponentName: "c:urlProbe",
	})
	if err != nil {
		t.Fatalf("ResolvePageTarget error = %v diagnostics=%#v", err, diagnostics)
	}
	if shell.Context.ComponentName != "c:urlProbe" {
		t.Fatalf("component = %q", shell.Context.ComponentName)
	}
	if len(shell.Regions) != 1 || shell.Regions[0].Components[0].ComponentName != "c:urlProbe" {
		t.Fatalf("regions = %#v", shell.Regions)
	}
}

func TestResolvePageTargetComponentReportsTargetMismatch(t *testing.T) {
	root := t.TempDir()
	metaPath := writeProjectFile(t, root, "force-app/main/default/lwc/recordOnly/recordOnly.js-meta.xml", `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata">
  <isExposed>true</isExposed>
  <targets>
    <target>lightning__RecordPage</target>
  </targets>
</LightningComponentBundle>`)
	p := project.Project{Root: root, LWCMetaFiles: []string{metaPath}}

	_, diagnostics, err := ResolvePageTarget(p, PageContext{
		Kind:          RenderTargetComponent,
		ComponentName: "c:recordOnly",
	})
	if err == nil {
		t.Fatalf("ResolvePageTarget error = nil, want target mismatch")
	}
	if len(diagnostics) != 1 || diagnostics[0].Code != "GLADELWC031" {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
}

func TestResolvePageTargetRecordQuickAction(t *testing.T) {
	root := t.TempDir()
	actionPath := writeProjectFile(t, root, "force-app/main/default/quickActions/Account.Update_Status.quickAction-meta.xml", `<QuickAction xmlns="http://soap.sforce.com/2006/04/metadata">
  <label>Update Status</label>
  <type>LightningComponent</type>
  <targetObject>Account</targetObject>
  <lightningComponent>c:actionProbe</lightningComponent>
</QuickAction>`)
	metaPath := writeProjectFile(t, root, "force-app/main/default/lwc/actionProbe/actionProbe.js-meta.xml", `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata">
  <isExposed>true</isExposed>
  <targets><target>lightning__RecordAction</target></targets>
  <targetConfigs>
    <targetConfig targets="lightning__RecordAction">
      <actionType>ScreenAction</actionType>
    </targetConfig>
  </targetConfigs>
</LightningComponentBundle>`)
	p := project.Project{Root: root, QuickActionFiles: []string{actionPath}, LWCMetaFiles: []string{metaPath}}

	shell, diagnostics, err := ResolvePageTarget(p, PageContext{
		Kind:          RenderTargetQuickAction,
		ObjectAPIName: "Account",
		RecordID:      "001000000000001AAA",
		ActionName:    "Update_Status",
	})
	if err != nil {
		t.Fatalf("ResolvePageTarget error = %v diagnostics=%#v", err, diagnostics)
	}
	if shell.Context.ComponentName != "c:actionProbe" {
		t.Fatalf("component = %q", shell.Context.ComponentName)
	}
	if shell.Context.ActionName != "Account.Update_Status" || shell.Context.ActionType != "ScreenAction" {
		t.Fatalf("action context = %#v", shell.Context)
	}
	if len(shell.Regions) != 1 || shell.Regions[0].Components[0].ComponentName != "c:actionProbe" {
		t.Fatalf("regions = %#v", shell.Regions)
	}
}

func TestResolvePageTargetGlobalQuickAction(t *testing.T) {
	root := t.TempDir()
	actionPath := writeProjectFile(t, root, "force-app/main/default/quickActions/Global_Status.quickAction-meta.xml", `<QuickAction xmlns="http://soap.sforce.com/2006/04/metadata">
  <label>Global Status</label>
  <type>LightningComponent</type>
  <lightningComponent>c:actionProbe</lightningComponent>
</QuickAction>`)
	metaPath := writeProjectFile(t, root, "force-app/main/default/lwc/actionProbe/actionProbe.js-meta.xml", `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata">
  <isExposed>true</isExposed>
  <targets><target>lightning__RecordAction</target></targets>
  <targetConfigs>
    <targetConfig targets="lightning__RecordAction">
      <actionType>Action</actionType>
    </targetConfig>
  </targetConfigs>
</LightningComponentBundle>`)
	p := project.Project{Root: root, QuickActionFiles: []string{actionPath}, LWCMetaFiles: []string{metaPath}}

	shell, diagnostics, err := ResolvePageTarget(p, PageContext{
		Kind:       RenderTargetQuickAction,
		ActionName: "Global_Status",
	})
	if err != nil {
		t.Fatalf("ResolvePageTarget error = %v diagnostics=%#v", err, diagnostics)
	}
	if shell.Context.ObjectAPIName != "" || shell.Context.RecordID != "" {
		t.Fatalf("global context = %#v", shell.Context)
	}
	if shell.Context.ActionName != "Global_Status" || shell.Context.ActionType != "Action" {
		t.Fatalf("action context = %#v", shell.Context)
	}
}

func TestResolvePageTargetReportsUnsupportedQuickAction(t *testing.T) {
	root := t.TempDir()
	actionPath := writeProjectFile(t, root, "force-app/main/default/quickActions/Account.Update_Status.quickAction-meta.xml", `<QuickAction xmlns="http://soap.sforce.com/2006/04/metadata">
  <label>Update Status</label>
  <type>LightningComponent</type>
  <targetObject>Account</targetObject>
</QuickAction>`)
	p := project.Project{Root: root, QuickActionFiles: []string{actionPath}}

	_, diagnostics, err := ResolvePageTarget(p, PageContext{
		Kind:          RenderTargetQuickAction,
		ObjectAPIName: "Account",
		RecordID:      "001000000000001AAA",
		ActionName:    "Update_Status",
	})
	if err == nil {
		t.Fatalf("ResolvePageTarget error = nil, want unsupported quick action")
	}
	if len(diagnostics) != 1 || diagnostics[0].Code != "GLADELWC070" {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
}

func TestResolvePageTargetReportsUnsupportedQuickActionActionType(t *testing.T) {
	root := t.TempDir()
	actionPath := writeProjectFile(t, root, "force-app/main/default/quickActions/Account.Update_Status.quickAction-meta.xml", `<QuickAction xmlns="http://soap.sforce.com/2006/04/metadata">
  <label>Update Status</label>
  <type>LightningComponent</type>
  <targetObject>Account</targetObject>
  <lightningComponent>c:actionProbe</lightningComponent>
</QuickAction>`)
	metaPath := writeProjectFile(t, root, "force-app/main/default/lwc/actionProbe/actionProbe.js-meta.xml", `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata">
  <isExposed>true</isExposed>
  <targets><target>lightning__RecordAction</target></targets>
  <targetConfigs>
    <targetConfig targets="lightning__RecordAction">
      <actionType>FlowAction</actionType>
    </targetConfig>
  </targetConfigs>
</LightningComponentBundle>`)
	p := project.Project{Root: root, QuickActionFiles: []string{actionPath}, LWCMetaFiles: []string{metaPath}}

	_, diagnostics, err := ResolvePageTarget(p, PageContext{
		Kind:          RenderTargetQuickAction,
		ObjectAPIName: "Account",
		RecordID:      "001000000000001AAA",
		ActionName:    "Update_Status",
	})
	if err == nil {
		t.Fatalf("ResolvePageTarget error = nil, want unsupported quick action action type")
	}
	if len(diagnostics) != 1 || diagnostics[0].Code != "GLADELWC015" {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
}

func TestResolvePageTargetReportsUnsupportedFormFactor(t *testing.T) {
	root := t.TempDir()
	pagePath := writeProjectFile(t, root, "force-app/main/default/flexipages/Account_Record_Page.flexipage-meta.xml", `<FlexiPage xmlns="http://soap.sforce.com/2006/04/metadata">
  <sobjectType>Account</sobjectType>
  <type>RecordPage</type>
  <flexiPageRegions>
    <name>main</name>
    <itemInstances><componentInstance><componentName>c:desktopOnly</componentName></componentInstance></itemInstances>
  </flexiPageRegions>
</FlexiPage>`)
	metaPath := writeProjectFile(t, root, "force-app/main/default/lwc/desktopOnly/desktopOnly.js-meta.xml", `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata">
  <isExposed>true</isExposed>
  <targets><target>lightning__RecordPage</target></targets>
  <targetConfigs>
    <targetConfig targets="lightning__RecordPage">
      <supportedFormFactors><supportedFormFactor type="Large"/></supportedFormFactors>
    </targetConfig>
  </targetConfigs>
</LightningComponentBundle>`)
	p := project.Project{Root: root, FlexiPageFiles: []string{pagePath}, LWCMetaFiles: []string{metaPath}}

	_, diagnostics, err := ResolvePageTarget(p, PageContext{
		Kind:          RenderTargetRecordPage,
		PageName:      "Account_Record_Page",
		ObjectAPIName: "Account",
		FormFactor:    "Small",
	})
	if err == nil {
		t.Fatalf("ResolvePageTarget error = nil, want unsupported form factor")
	}
	if len(diagnostics) != 1 || diagnostics[0].Code != "GLADELWC032" {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
}

func TestResolvePageTargetUsesApplicationDefaultLandingTab(t *testing.T) {
	root := t.TempDir()
	appPath := writeProjectFile(t, root, "force-app/main/default/applications/Sales.app-meta.xml", `<CustomApplication xmlns="http://soap.sforce.com/2006/04/metadata">
  <label>Sales</label>
  <navType>Standard</navType>
  <tabs>Lwc_Probe</tabs>
</CustomApplication>`)
	tabPath := writeProjectFile(t, root, "force-app/main/default/tabs/Lwc_Probe.tab-meta.xml", `<CustomTab xmlns="http://soap.sforce.com/2006/04/metadata">
  <label>LWC Probe</label>
  <lwcComponent>c:contextProbe</lwcComponent>
</CustomTab>`)
	p := project.Project{Root: root, ApplicationFiles: []string{appPath}, TabFiles: []string{tabPath}}

	shell, diagnostics, err := ResolvePageTarget(p, PageContext{
		Kind:    RenderTargetTab,
		AppName: "Sales",
	})
	if err != nil {
		t.Fatalf("ResolvePageTarget error = %v diagnostics=%#v", err, diagnostics)
	}
	if shell.Context.TabName != "Lwc_Probe" || shell.Tab.Target != "c:contextProbe" {
		t.Fatalf("shell = %#v", shell)
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
