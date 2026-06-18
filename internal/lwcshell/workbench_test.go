package lwcshell

import (
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestBuildWorkbenchModelUsesCustomApplicationNavigation(t *testing.T) {
	root := t.TempDir()
	appPath := writeProjectFile(t, root, "force-app/main/default/applications/Lwc_Shell.app-meta.xml", `<CustomApplication xmlns="http://soap.sforce.com/2006/04/metadata">
  <label>LWC Shell</label>
  <navType>Standard</navType>
  <defaultLandingTab>Lwc_Probe</defaultLandingTab>
  <tabs>Lwc_Probe</tabs>
  <tabs>standard-Account</tabs>
</CustomApplication>`)
	tabPath := writeProjectFile(t, root, "force-app/main/default/tabs/Lwc_Probe.tab-meta.xml", `<CustomTab xmlns="http://soap.sforce.com/2006/04/metadata">
  <label>LWC Probe</label>
  <lwcComponent>c:contextProbe</lwcComponent>
</CustomTab>`)
	p := project.Project{Root: root, ApplicationFiles: []string{appPath}, TabFiles: []string{tabPath}}

	model := BuildWorkbenchModel(p, ShellPage{Context: PageContext{Kind: RenderTargetTab, AppName: "Lwc_Shell", TabName: "Lwc_Probe"}}, "/lwc/preview/tab/Lwc_Probe")

	if model.Mode != "standard" || len(model.Apps) != 1 {
		t.Fatalf("model = %#v", model)
	}
	app := model.Apps[0]
	if app.Name != "Lwc_Shell" || app.Label != "LWC Shell" || app.Mode != "standard" {
		t.Fatalf("app = %#v", app)
	}
	if app.DefaultURL != "/lwc/preview/tab/Lwc_Probe" {
		t.Fatalf("DefaultURL = %q", app.DefaultURL)
	}
	wantNav := []string{"Lwc_Probe", "standard-Account"}
	if !reflect.DeepEqual(app.NavItems, wantNav) {
		t.Fatalf("NavItems = %#v, want %#v", app.NavItems, wantNav)
	}
}

func TestBuildWorkbenchModelUsesConsoleModeForConsoleApplication(t *testing.T) {
	root := t.TempDir()
	appPath := writeProjectFile(t, root, "force-app/main/default/applications/Support_Console.app-meta.xml", `<CustomApplication xmlns="http://soap.sforce.com/2006/04/metadata">
  <label>Support Console</label>
  <navType>Console</navType>
  <tabs>standard-Case</tabs>
  <tabs>Lwc_Probe</tabs>
</CustomApplication>`)
	p := project.Project{Root: root, ApplicationFiles: []string{appPath}}

	model := BuildWorkbenchModel(p, ShellPage{Context: PageContext{Kind: RenderTargetAppPage, AppName: "Support_Console", PageName: "Support_Page"}}, "/lwc/preview/app/Support_Page?app=Support_Console")

	if model.Mode != "console" || len(model.Apps) != 1 {
		t.Fatalf("model = %#v", model)
	}
	app := model.Apps[0]
	if app.Mode != "console" || !reflect.DeepEqual(app.NavItems, []string{"standard-Case", "Lwc_Probe"}) {
		t.Fatalf("app = %#v", app)
	}
	if !model.Active.Context.Workspace.Console || len(model.Active.Context.Workspace.Tabs) != 1 {
		t.Fatalf("workspace = %#v", model.Active.Context.Workspace)
	}
}

func TestDiscoverShellRoutesIncludesUtilityBarFlexiPages(t *testing.T) {
	root := t.TempDir()
	writeProjectFile(t, root, "force-app/main/default/flexipages/Support_Utility.flexipage-meta.xml", `<FlexiPage xmlns="http://soap.sforce.com/2006/04/metadata">
  <masterLabel>Support Utility</masterLabel>
  <type>UtilityBar</type>
  <flexiPageRegions>
    <name>utilityItems</name>
    <type>Region</type>
    <itemInstances>
      <componentInstance>
        <componentName>c:utilityProbe</componentName>
        <identifier>utilityProbe</identifier>
      </componentInstance>
    </itemInstances>
  </flexiPageRegions>
</FlexiPage>`)
	p := project.Project{Root: root, FlexiPageFiles: []string{filepath.Join(root, "force-app/main/default/flexipages/Support_Utility.flexipage-meta.xml")}}

	routes := DiscoverShellRoutes(p)

	if !slices.ContainsFunc(routes, func(route ShellRoute) bool {
		return route.Kind == RenderTargetUtilityBar &&
			route.Label == "Support Utility" &&
			route.URL == "/lwc/preview/utility/Support_Utility" &&
			route.PageName == "Support_Utility"
	}) {
		t.Fatalf("routes = %#v", routes)
	}
}

func TestDiscoverShellRoutesIncludesCommunityContextPresets(t *testing.T) {
	root := t.TempDir()
	writeProjectFile(t, root, "glade.lwc.json", `{
  "contexts": {
    "communityAccount": {
      "target": "communityPage",
      "component": "c:communityProbe",
      "page": "Account",
      "community": {
        "site": "Partner_Portal",
        "basePath": "/partners",
        "siteId": "0DM000000000001",
        "networkId": "0DB000000000001"
      },
      "state": {"c__view": "summary"}
    }
  }
}`)
	p := project.Project{Root: root}

	routes := DiscoverShellRoutes(p)

	if !slices.ContainsFunc(routes, func(route ShellRoute) bool {
		return route.Kind == RenderTargetCommunityPage &&
			route.Label == "Partner_Portal / Account" &&
			route.URL == "/lwc/preview/community/Partner_Portal/Account?state.c__view=summary" &&
			route.Component == "c:communityProbe" &&
			route.PageName == "Account"
	}) {
		t.Fatalf("routes = %#v", routes)
	}
}

func TestDiscoverShellRoutesIncludesUrlAddressableAndQuickActions(t *testing.T) {
	root := t.TempDir()
	metaPath := writeProjectFile(t, root, "force-app/main/default/lwc/actionProbe/actionProbe.js-meta.xml", `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata">
  <apiVersion>61.0</apiVersion>
  <isExposed>true</isExposed>
  <targets>
    <target>lightning__RecordAction</target>
    <target>lightning__UrlAddressable</target>
  </targets>
  <targetConfigs>
    <targetConfig targets="lightning__RecordAction">
      <actionType>ScreenAction</actionType>
    </targetConfig>
  </targetConfigs>
</LightningComponentBundle>`)
	writeProjectFile(t, root, "force-app/main/default/lwc/actionProbe/actionProbe.js", "export default class ActionProbe {}")
	actionPath := writeProjectFile(t, root, "force-app/main/default/quickActions/Account.Update_Status.quickAction-meta.xml", `<QuickAction xmlns="http://soap.sforce.com/2006/04/metadata">
  <actionSubtype>ScreenAction</actionSubtype>
  <label>Update Status</label>
  <lightningWebComponent>actionProbe</lightningWebComponent>
  <type>LightningWebComponent</type>
</QuickAction>`)
	flowMetaPath := writeProjectFile(t, root, "force-app/main/default/lwc/flowActionProbe/flowActionProbe.js-meta.xml", `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata">
  <apiVersion>61.0</apiVersion>
  <isExposed>true</isExposed>
  <targets>
    <target>lightning__FlowAction</target>
  </targets>
</LightningComponentBundle>`)
	writeProjectFile(t, root, "force-app/main/default/lwc/flowActionProbe/flowActionProbe.js", "export default class FlowActionProbe {}")
	flowActionPath := writeProjectFile(t, root, "force-app/main/default/quickActions/Account.Start_Flow.quickAction-meta.xml", `<QuickAction xmlns="http://soap.sforce.com/2006/04/metadata">
  <actionSubtype>FlowAction</actionSubtype>
  <label>Start Flow</label>
  <lightningWebComponent>flowActionProbe</lightningWebComponent>
  <type>LightningWebComponent</type>
</QuickAction>`)
	p := project.Project{
		Root:             root,
		LWCMetaFiles:     []string{metaPath, flowMetaPath},
		QuickActionFiles: []string{actionPath, flowActionPath},
	}

	routes := DiscoverShellRoutes(p)

	if !slices.ContainsFunc(routes, func(route ShellRoute) bool {
		return route.Kind == RenderTargetURLAddressable &&
			route.Label == "c:actionProbe URL" &&
			route.URL == "/lwc/preview/cmp/c/actionProbe" &&
			route.Component == "c:actionProbe"
	}) {
		t.Fatalf("routes missing URL-addressable actionProbe: %#v", routes)
	}
	if !slices.ContainsFunc(routes, func(route ShellRoute) bool {
		return route.Kind == RenderTargetQuickAction &&
			route.Label == "Update Status" &&
			route.URL == "/lwc/preview/action/Account/001000000000001AAA/Update_Status" &&
			route.Component == "c:actionProbe" &&
			route.ObjectName == "Account" &&
			route.RecordID == "001000000000001AAA"
	}) {
		t.Fatalf("routes missing quick action: %#v", routes)
	}
	if !slices.ContainsFunc(routes, func(route ShellRoute) bool {
		return route.Kind == RenderTargetFlowAction &&
			route.Label == "Start Flow" &&
			route.URL == "/lwc/preview/action/Account/001000000000001AAA/Start_Flow" &&
			route.Component == "c:flowActionProbe" &&
			route.ActionType == "FlowAction"
	}) {
		t.Fatalf("routes missing flow action: %#v", routes)
	}
}
