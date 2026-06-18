package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/lwcshell"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

func TestLWCShellRootRendersWorkbenchRoutePicker(t *testing.T) {
	root, err := lightningTestRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	fixture := filepath.Join(root, "testdata", "local-tests", "lwc-shell")
	p, err := project.Load(fixture)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewWithSource(&storage.OrgState{}, SourceMetadata{Project: p})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/lwc", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		"Glade LWC Shell",
		"data-glade-shell=\"workbench\"",
		"data-glade-workbench-builder",
		"data-glade-component-catalog",
		"data-glade-page-canvas",
		"data-glade-region-drop=\"main\"",
		"data-glade-add-component=\"c:contextProbe\"",
		"class=\"glade-shell-button\"",
		"/lwc/preview/record/Account/",
		"/lwc/preview/app/Sales_Dashboard",
		"/lwc/preview/home/Custom_Home",
		"/lwc/preview/tab/Lwc_Probe",
		"data-glade-context-panel",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in:\n%s", want, body)
		}
	}
	if !strings.Contains(body, `"components":[`) || !strings.Contains(body, `"qualifiedName":"c:contextProbe"`) {
		t.Fatalf("workbench model missing component catalog in:\n%s", body)
	}
	if !strings.Contains(body, `"target":"lightning__AppPage"`) || !strings.Contains(body, `"target":"lightning__RecordPage"`) {
		t.Fatalf("workbench model missing target support in:\n%s", body)
	}
}

func TestServerRootRendersLWCWorkbenchWhenProjectHasLWCs(t *testing.T) {
	root, err := lightningTestRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	fixture := filepath.Join(root, "testdata", "local-tests", "lwc-shell")
	p, err := project.Load(fixture)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewWithSource(&storage.OrgState{}, SourceMetadata{Project: p})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		"Glade LWC Shell",
		"data-glade-workbench-builder",
		"data-glade-add-component=\"c:contextProbe\"",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in:\n%s", want, body)
		}
	}
}

func TestLWCShellRendersApplicationNavAndConsoleMode(t *testing.T) {
	root := t.TempDir()
	writeLWCShellServerTestFile(t, root, "force-app/main/default/applications/Support_Console.app-meta.xml", `<CustomApplication xmlns="http://soap.sforce.com/2006/04/metadata">
	  <label>Support Console</label>
	  <navType>Console</navType>
	  <tabs>standard-Case</tabs>
	  <tabs>Lwc_Probe</tabs>
	</CustomApplication>`)
	writeLWCShellServerTestFile(t, root, "force-app/main/default/flexipages/Support_Page.flexipage-meta.xml", `<FlexiPage xmlns="http://soap.sforce.com/2006/04/metadata">
	  <masterLabel>Support Page</masterLabel>
	  <type>AppPage</type>
	  <flexiPageRegions><name>main</name></flexiPageRegions>
	</FlexiPage>`)
	writeLWCShellServerTestFile(t, root, "force-app/main/default/tabs/Lwc_Probe.tab-meta.xml", `<CustomTab xmlns="http://soap.sforce.com/2006/04/metadata">
	  <label>LWC Probe</label>
	  <lwcComponent>c:contextProbe</lwcComponent>
	</CustomTab>`)
	writeLWCShellServerTestFile(t, root, "force-app/main/default/lwc/contextProbe/contextProbe.js", `import { LightningElement } from "lwc"; export default class ContextProbe extends LightningElement {}`)
	writeLWCShellServerTestFile(t, root, "force-app/main/default/lwc/contextProbe/contextProbe.html", `<template><p>Context</p></template>`)
	writeLWCShellServerTestFile(t, root, "force-app/main/default/lwc/contextProbe/contextProbe.js-meta.xml", `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata">
	  <apiVersion>64.0</apiVersion>
	  <isExposed>true</isExposed>
	  <targets>
	    <target>lightning__Tab</target>
	  </targets>
	</LightningComponentBundle>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewWithSource(&storage.OrgState{}, SourceMetadata{Project: p})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/lwc/preview/app/Support_Page?app=Support_Console", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		`data-glade-app-mode="console"`,
		`class="glade-console-rail"`,
		`Support Console`,
		`standard-Case`,
		`Lwc_Probe`,
		`GLADELWC072`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in:\n%s", want, body)
		}
	}
	if count := strings.Count(body, "GLADELWC072:"); count != 1 {
		t.Fatalf("visible GLADELWC072 count = %d in:\n%s", count, body)
	}
}

func TestLightningRuntimeServesShellAndSLDSAssets(t *testing.T) {
	handler := New(&storage.OrgState{})
	cases := []struct {
		path string
		want string
	}{
		{path: "/lightning/runtime/shell/app.js", want: "bootGladeShell"},
		{path: "/lightning/runtime/shell/community-host.js", want: "applyCommunityHost"},
		{path: "/lightning/runtime/shims/community.js", want: "readCommunityValue"},
		{path: "/lightning/runtime/shims/site.js", want: "readSiteId"},
		{path: "/lightning/runtime/shell/glade-shell.css", want: ".glade-shell"},
		{path: "/lightning/runtime/slds/slds-loader.js", want: "loadSLDS"},
		{path: "/lightning/runtime/slds/glade-slds.css", want: ".slds-button"},
		{path: "/assets/icons/utility-sprite/svg/symbols.svg", want: "<symbol"},
		{path: "/lightning/shims/core/apex.js", want: "refreshApex"},
		{path: "/lightning/shims/lightning/actions.js", want: "CloseActionScreenEvent"},
		{path: "/lightning/shims/lightning/empApi.js", want: "subscribe"},
		{path: "/lightning/shims/lightning/flowSupport.js", want: "FlowAttributeChangeEvent"},
		{path: "/lightning/shims/lightning/refresh.js", want: "registerRefreshHandler"},
		{path: "/lightning/shims/lightning/uiListApi.js", want: "GLADELWC050"},
		{path: "/lightning/shims/lightning/platformWorkspaceApi.js", want: "getFocusedTabInfo"},
		{path: "/lightning/shims/lightning/treeGrid.js", want: "lightning-tree-grid"},
		{path: "/lightning/shims/community/basePath.js", want: `readCommunityValue("basePath", "/s")`},
		{path: "/lightning/shims/community/Id.js", want: `readCommunityValue("networkId", "")`},
		{path: "/lightning/shims/site/Id.js", want: `readSiteId`},
	}
	for _, tc := range cases {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d body=%s", tc.path, rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), tc.want) {
			t.Fatalf("%s missing %q in %s", tc.path, tc.want, rec.Body.String())
		}
	}
}

func TestLightningRuntimeCSSKeepsShellControlsOutOfSLDS(t *testing.T) {
	handler := New(&storage.OrgState{})

	shellCSS := lightningRuntimeTestAsset(t, handler, "/lightning/runtime/shell/glade-shell.css")
	for _, selector := range []string{
		".glade-builder-toolbar button",
		".glade-component-actions button",
		".glade-draft-component button",
	} {
		if strings.Contains(shellCSS, selector) {
			t.Fatalf("shell CSS should not target all buttons with %q:\n%s", selector, shellCSS)
		}
	}
	if !strings.Contains(shellCSS, ".glade-shell-button") {
		t.Fatalf("shell CSS missing dedicated shell button class:\n%s", shellCSS)
	}
	builderJS := lightningRuntimeTestAsset(t, handler, "/lightning/runtime/shell/workbench-builder.js")
	if !strings.Contains(builderJS, "glade-shell-button") {
		t.Fatalf("shell builder JS missing dedicated shell button class:\n%s", builderJS)
	}

	sldsCSS := lightningRuntimeTestAsset(t, handler, "/lightning/runtime/slds/glade-slds.css")
	for _, want := range []string{
		".slds-badge",
		".slds-badge_lightest",
		".slds-theme_success",
		".slds-theme_error",
	} {
		if !strings.Contains(sldsCSS, want) {
			t.Fatalf("SLDS CSS missing %q:\n%s", want, sldsCSS)
		}
	}
}

func lightningRuntimeTestAsset(t *testing.T, handler http.Handler, path string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("%s status = %d body=%s", path, rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

func TestLightningRuntimePrefersRepoAssetsForSourceCheckout(t *testing.T) {
	root, err := lightningTestRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	dirs := lightningRuntimeAssetDirs("shell")
	if len(dirs) == 0 {
		t.Fatal("no shell runtime asset dirs")
	}
	want := filepath.Join(root, "lwcruntime", "src", "shell")
	if dirs[0] != want {
		t.Fatalf("first shell runtime asset dir = %q, want %q; dirs=%#v", dirs[0], want, dirs)
	}
}

func TestLightningLocalContextJSONReportsActiveShellState(t *testing.T) {
	root, err := lightningTestRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	fixture := filepath.Join(root, "testdata", "local-tests", "lwc-shell")
	p, err := project.Load(fixture)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewWithSource(&storage.OrgState{}, SourceMetadata{Project: p})

	activeRoute := "/lwc/preview/record/Account/001000000000001AAA?app=Sales&formFactor=Large&page=Account_Record_Page&state.c__mode=demo"
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/lightning/local/context.json?url="+url.QueryEscape(activeRoute), nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var got struct {
		ActiveRoute     string               `json:"activeRoute"`
		PageReference   map[string]any       `json:"pageReference"`
		Context         lwcshell.PageContext `json:"context"`
		Mounts          []lwcShellMount      `json:"mounts"`
		Apps            []map[string]any     `json:"apps"`
		Routes          []map[string]any     `json:"routes"`
		DefaultContext  string               `json:"defaultContext"`
		SelectedContext string               `json:"selectedContext"`
		Contexts        []struct {
			Name        string               `json:"name"`
			SelectedURL string               `json:"selectedUrl"`
			Context     lwcshell.PageContext `json:"context"`
		} `json:"contexts"`
		Services    map[string]string     `json:"services"`
		Diagnostics []lwcshell.Diagnostic `json:"diagnostics"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, rec.Body.String())
	}
	if got.ActiveRoute != activeRoute {
		t.Fatalf("active route = %q", got.ActiveRoute)
	}
	if got.Context.Kind != lwcshell.RenderTargetRecordPage || got.Context.RecordID != "001000000000001AAA" || got.Context.ObjectAPIName != "Account" {
		t.Fatalf("context = %#v", got.Context)
	}
	if got.PageReference["type"] != "standard__recordPage" {
		t.Fatalf("page reference = %#v", got.PageReference)
	}
	if len(got.Mounts) == 0 {
		t.Fatalf("mounts = %#v", got.Mounts)
	}
	if len(got.Routes) == 0 {
		t.Fatalf("routes = %#v", got.Routes)
	}
	if len(got.Apps) == 0 {
		t.Fatalf("apps = %#v", got.Apps)
	}
	if got.DefaultContext != "accountRecord" {
		t.Fatalf("default context = %q", got.DefaultContext)
	}
	if got.SelectedContext != "accountRecord" {
		t.Fatalf("selected context = %q", got.SelectedContext)
	}
	foundRecordPreset := false
	for _, preset := range got.Contexts {
		if preset.Name != "accountRecord" {
			continue
		}
		foundRecordPreset = true
		if preset.SelectedURL != "/lwc/preview/record/Account/001000000000001AAA?app=Sales&formFactor=Large&page=Account_Record_Page&state.c__mode=demo" {
			t.Fatalf("accountRecord selected URL = %q", preset.SelectedURL)
		}
		if preset.Context.Kind != lwcshell.RenderTargetRecordPage || preset.Context.RecordID != "001000000000001AAA" {
			t.Fatalf("accountRecord context = %#v", preset.Context)
		}
	}
	if !foundRecordPreset {
		t.Fatalf("accountRecord preset not found in %#v", got.Contexts)
	}
	for service, want := range map[string]string{
		"apex":                   "supported",
		"apexWire":               "supported",
		"imperativeApex":         "supported",
		"lds":                    "supported",
		"uiRecordApi":            "supported",
		"uiObjectInfoApi":        "supported",
		"uiLayoutApi":            "supported",
		"uiRelatedListApi":       "supported",
		"navigation":             "supported",
		"labels":                 "supported",
		"resources":              "supported",
		"schema":                 "supported",
		"user":                   "supported",
		"i18n":                   "supported",
		"toast":                  "supported",
		"lms":                    "supported",
		"platformResourceLoader": "supported",
		"urlAddressable":         "supported-local",
		"quickActions":           "supported-local",
		"platformWorkspaceApi":   "partial",
		"baseComponents":         "supported-local",
		"slds":                   "partial",
		"visualforceHost":        "supported-local",
	} {
		if got.Services[service] != want {
			t.Fatalf("service %s = %q, want %q in %#v", service, got.Services[service], want, got.Services)
		}
	}
}

func TestLWCShellAppRouteFallsBackToApplicationDefaultTab(t *testing.T) {
	root := t.TempDir()
	writeLWCShellServerTestFile(t, root, "force-app/main/default/applications/Support_Console.app-meta.xml", `<CustomApplication xmlns="http://soap.sforce.com/2006/04/metadata">
	  <label>Support Console</label>
	  <navType>Console</navType>
	  <tabs>Lwc_Probe</tabs>
	</CustomApplication>`)
	writeLWCShellServerTestFile(t, root, "force-app/main/default/tabs/Lwc_Probe.tab-meta.xml", `<CustomTab xmlns="http://soap.sforce.com/2006/04/metadata">
	  <label>LWC Probe</label>
	  <lwcComponent>c:contextProbe</lwcComponent>
	</CustomTab>`)
	writeLWCShellServerTestFile(t, root, "force-app/main/default/lwc/contextProbe/contextProbe.js", `import { LightningElement } from "lwc"; export default class ContextProbe extends LightningElement {}`)
	writeLWCShellServerTestFile(t, root, "force-app/main/default/lwc/contextProbe/contextProbe.html", `<template><p>Context</p></template>`)
	writeLWCShellServerTestFile(t, root, "force-app/main/default/lwc/contextProbe/contextProbe.js-meta.xml", `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata">
	  <apiVersion>64.0</apiVersion>
	  <isExposed>true</isExposed>
	  <targets>
	    <target>lightning__Tab</target>
	  </targets>
	</LightningComponentBundle>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewWithSource(&storage.OrgState{}, SourceMetadata{Project: p})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/lwc/preview/app/Support_Console", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		`data-glade-app-mode="console"`,
		`Support Console`,
		`Lwc_Probe`,
		`c:contextProbe`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in:\n%s", want, body)
		}
	}
}

func TestLWCShellAppRouteFallsBackToFlexiPageDefaultTab(t *testing.T) {
	root, err := lightningTestRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	fixture := filepath.Join(root, "testdata", "local-tests", "lwc-shell")
	p, err := project.Load(fixture)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewWithSource(&storage.OrgState{}, SourceMetadata{Project: p})

	shell, _, diagnostics, err := handler.resolveLWCShellRequest(httptest.NewRequest(http.MethodGet, "/lwc/preview/app/Lwc_Shell", nil), []string{"app", "Lwc_Shell"})
	if err != nil {
		t.Fatalf("resolve error = %v diagnostics=%#v", err, diagnostics)
	}
	mounts := lwcShellMounts(shell)
	if len(mounts) == 0 {
		t.Fatalf("mounts = %#v shell=%#v diagnostics=%#v", mounts, shell, diagnostics)
	}
	if shell.Context.TabName != "Lwc_Probe" || shell.Context.PageName != "Sales_Dashboard" {
		t.Fatalf("context = %#v", shell.Context)
	}
}

func TestResolveLWCShellRequestServesCommunityPageRouteFromContextPreset(t *testing.T) {
	root := t.TempDir()
	writeCommunityProbeBundle(t, root, "lightningCommunity__Page")
	writeLWCShellServerTestFile(t, root, "force-app/main/default/lwc/themeLayout/themeLayout.js", `import { LightningElement } from 'lwc';
export default class ThemeLayout extends LightningElement {}`)
	writeLWCShellServerTestFile(t, root, "force-app/main/default/lwc/themeLayout/themeLayout.html", `<template><slot></slot></template>`)
	writeLWCShellServerTestFile(t, root, "force-app/main/default/lwc/themeLayout/themeLayout.js-meta.xml", `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata">
	  <isExposed>true</isExposed>
	  <targets><target>lightningCommunity__Theme_Layout</target></targets>
	</LightningComponentBundle>`)
	writeLWCShellServerTestFile(t, root, "glade.lwc.json", `{
	  "contexts": {
	    "communityAccount": {
	      "target": "communityPage",
	      "component": "c:communityProbe",
	      "page": "Account",
	      "community": {
	        "site": "Partner_Portal",
	        "basePath": "/partners",
	        "siteId": "0DM000000000001",
	        "networkId": "0DB000000000001",
	        "guest": true,
	        "language": "en-US"
	      },
	      "pageReference": {
	        "type": "comm__namedPage",
	        "attributes": {"name": "Account"},
	        "state": {"c__view": "summary"}
	      }
	    }
	  }
	}`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewWithSource(&storage.OrgState{}, SourceMetadata{Project: p})

	shell, _, diagnostics, err := handler.resolveLWCShellRequest(httptest.NewRequest(http.MethodGet, "/lwc/preview/community/Partner_Portal/Account", nil), []string{"community", "Partner_Portal", "Account"})
	if err != nil {
		t.Fatalf("resolve error = %v diagnostics=%#v", err, diagnostics)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
	if shell.Context.Kind != lwcshell.RenderTargetCommunityPage ||
		shell.Context.ComponentName != "c:communityProbe" ||
		shell.Context.PageName != "Account" ||
		shell.Context.Community.Site != "Partner_Portal" ||
		shell.Context.Community.BasePath != "/partners" ||
		!shell.Context.Community.Guest {
		t.Fatalf("context = %#v", shell.Context)
	}
	if shell.ThemeLayout == nil || shell.ThemeLayout.ComponentName != "c:themeLayout" {
		t.Fatalf("theme layout = %#v", shell.ThemeLayout)
	}
	ref := lwcShellPageReference(shell)
	if ref["type"] != "comm__namedPage" {
		t.Fatalf("page reference = %#v", ref)
	}
	mounts := lwcShellMounts(shell)
	if len(mounts) != 2 || mounts[0].Qualified != "c:themeLayout" || mounts[1].Qualified != "c:communityProbe" {
		t.Fatalf("mounts = %#v", mounts)
	}
	body := renderLWCShellHTML(lwcbrowser.PageConfig{}, shell)
	for _, want := range []string{
		`data-glade-community-shell`,
		`data-glade-community-site="Partner_Portal"`,
		`data-glade-community-guest="true"`,
		`c:themeLayout`,
		`c:communityProbe`,
		`"basePath":"/partners"`,
		`"type":"comm__namedPage"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in:\n%s", want, body)
		}
	}
}

func TestLightningLocalContextJSONReportsCommunityContextAndDiagnostics(t *testing.T) {
	root := t.TempDir()
	writeCommunityProbeBundle(t, root, "lightningCommunity__Page")
	writeLWCShellServerTestFile(t, root, "glade.lwc.json", `{
	  "contexts": {
	    "communityAccount": {
	      "target": "communityPage",
	      "component": "c:communityProbe",
	      "page": "Account",
	      "community": {"site": "Partner_Portal", "guest": true}
	    }
	  }
	}`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewWithSource(&storage.OrgState{}, SourceMetadata{Project: p})

	activeRoute := "/lwc/preview/community/Partner_Portal/Account"
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/lightning/local/context.json?url="+url.QueryEscape(activeRoute), nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var got struct {
		PageReference map[string]any        `json:"pageReference"`
		Context       lwcshell.PageContext  `json:"context"`
		Mounts        []lwcShellMount       `json:"mounts"`
		Services      map[string]string     `json:"services"`
		Diagnostics   []lwcshell.Diagnostic `json:"diagnostics"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, rec.Body.String())
	}
	if got.Context.Kind != lwcshell.RenderTargetCommunityPage ||
		got.Context.Community.Site != "Partner_Portal" ||
		got.Context.Community.BasePath != "/s" ||
		!got.Context.Community.Guest {
		t.Fatalf("context = %#v", got.Context)
	}
	if got.PageReference["type"] != "comm__namedPage" {
		t.Fatalf("page reference = %#v", got.PageReference)
	}
	if len(got.Mounts) != 1 || got.Mounts[0].Attrs["community"] == nil {
		t.Fatalf("mounts = %#v", got.Mounts)
	}
	if got.Services["community"] != "supported-local" {
		t.Fatalf("services = %#v", got.Services)
	}
	foundMissingIDs := false
	for _, diag := range got.Diagnostics {
		if diag.Code == "GLADELWC102" {
			foundMissingIDs = true
		}
	}
	if !foundMissingIDs {
		t.Fatalf("diagnostics = %#v", got.Diagnostics)
	}
}

func TestResolveLWCShellRequestServesDirectCommunityComponentRoute(t *testing.T) {
	root := t.TempDir()
	writeCommunityProbeBundle(t, root, "lightningCommunity__Default")
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewWithSource(&storage.OrgState{}, SourceMetadata{Project: p})

	shell, _, diagnostics, err := handler.resolveLWCShellRequest(httptest.NewRequest(http.MethodGet, "/lwc/preview/community/Partner_Portal/cmp/c/communityProbe", nil), []string{"community", "Partner_Portal", "cmp", "c", "communityProbe"})
	if err != nil {
		t.Fatalf("resolve error = %v diagnostics=%#v", err, diagnostics)
	}
	if shell.Context.Kind != lwcshell.RenderTargetCommunityPage ||
		shell.Context.ComponentName != "c:communityProbe" ||
		shell.Context.Community.Site != "Partner_Portal" ||
		shell.Context.Community.BasePath != "/s" {
		t.Fatalf("context = %#v", shell.Context)
	}
	if len(diagnostics) != 1 || diagnostics[0].Code != "GLADELWC102" {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
}

func TestResolveLWCShellRequestReportsUnsupportedExperienceBuilderFeature(t *testing.T) {
	root := t.TempDir()
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewWithSource(&storage.OrgState{}, SourceMetadata{Project: p})

	_, _, diagnostics, err := handler.resolveLWCShellRequest(httptest.NewRequest(http.MethodGet, "/lwc/preview/community/Partner_Portal/managed-content/welcome", nil), []string{"community", "Partner_Portal", "managed-content", "welcome"})
	if err == nil {
		t.Fatal("expected unsupported Experience Builder feature error")
	}
	if len(diagnostics) != 1 || diagnostics[0].Code != "GLADELWC101" {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
}

func TestLightningLocalContextJSONReportsDirectComponentContext(t *testing.T) {
	root, err := lightningTestRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	fixture := filepath.Join(root, "testdata", "local-tests", "lwc-shell")
	p, err := project.Load(fixture)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewWithSource(&storage.OrgState{}, SourceMetadata{Project: p})

	activeRoute := "/lwc/preview/component/c/recordProbe?app=Sales&formFactor=Large&objectApiName=Account&recordId=001000000000001AAA"
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/lightning/local/context.json?url="+url.QueryEscape(activeRoute), nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var got struct {
		Context lwcshell.PageContext `json:"context"`
		Mounts  []lwcShellMount      `json:"mounts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, rec.Body.String())
	}
	if got.Context.Kind != lwcshell.RenderTargetComponent ||
		got.Context.ComponentName != "c:recordProbe" ||
		got.Context.AppName != "Sales" ||
		got.Context.FormFactor != "Large" ||
		got.Context.ObjectAPIName != "Account" ||
		got.Context.RecordID != "001000000000001AAA" {
		t.Fatalf("context = %#v", got.Context)
	}
	if len(got.Mounts) != 1 || got.Mounts[0].Attrs["recordId"] != "001000000000001AAA" || got.Mounts[0].Attrs["objectApiName"] != "Account" {
		t.Fatalf("mounts = %#v", got.Mounts)
	}
}

func TestRenderLWCShellHTMLMountsDirectComponentWithContext(t *testing.T) {
	html := renderLWCShellHTML(lwcbrowser.PageConfig{}, lwcshell.ShellPage{
		Context: lwcshell.PageContext{
			Kind:          lwcshell.RenderTargetComponent,
			ComponentName: "c:contextProbe",
			RecordID:      "001000000000001AAA",
			ObjectAPIName: "Account",
			FormFactor:    "Large",
		},
	})

	for _, want := range []string{
		"glade-lightning-config",
		"glade-lwc-context",
		"c:contextProbe",
		`"recordId":"001000000000001AAA"`,
		`"objectApiName":"Account"`,
		`"standard__component"`,
		`data-glade-region="main"`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing %q in:\n%s", want, html)
		}
	}
}

func TestRenderLWCShellHTMLMountsFlexiPageRegions(t *testing.T) {
	html := renderLWCShellHTML(lwcbrowser.PageConfig{}, lwcshell.ShellPage{
		Context: lwcshell.PageContext{
			Kind:          lwcshell.RenderTargetRecordPage,
			PageName:      "Account_Record_Page",
			RecordID:      "001000000000001AAA",
			ObjectAPIName: "Account",
		},
		Regions: []lwcshell.PageRegion{{
			Name: "main",
			Components: []lwcshell.PageComponent{{
				ComponentName: "c:recordProbe",
				Properties:    map[string]string{"title": "From metadata"},
			}},
		}},
	})

	for _, want := range []string{
		`data-glade-region="main"`,
		`c:recordProbe`,
		`"title":"From metadata"`,
		`"recordId":"001000000000001AAA"`,
		`"standard__recordPage"`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing %q in:\n%s", want, html)
		}
	}
}

func TestResolveLWCShellRequestAppliesMetadataDefaultsAndDiagnostics(t *testing.T) {
	root := t.TempDir()
	writeLWCShellServerTestFile(t, root, "force-app/main/default/lwc/contextProbe/contextProbe.js-meta.xml", `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata">
  <isExposed>true</isExposed>
  <targets><target>lightning__RecordPage</target></targets>
  <targetConfigs>
    <targetConfig targets="lightning__RecordPage">
      <property name="title" type="String" default="Metadata Title"/>
      <objects><object>Account</object></objects>
    </targetConfig>
  </targetConfigs>
</LightningComponentBundle>`)
	writeLWCShellServerTestFile(t, root, "force-app/main/default/flexipages/Account_Record_Page.flexipage-meta.xml", `<FlexiPage xmlns="http://soap.sforce.com/2006/04/metadata">
  <sobjectType>Account</sobjectType>
  <type>RecordPage</type>
  <flexiPageRegions>
    <name>main</name>
    <componentInstances>
      <componentName>contextProbe</componentName>
    </componentInstances>
    <componentInstances>
      <componentName>missingProbe</componentName>
    </componentInstances>
  </flexiPageRegions>
</FlexiPage>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewWithSource(&storage.OrgState{}, SourceMetadata{Project: p})

	shell, _, diagnostics, err := handler.resolveLWCShellRequest(httptest.NewRequest(http.MethodGet, "/lwc/preview/record/Account/001000000000001AAA?page=Account_Record_Page", nil), []string{"record", "Account", "001000000000001AAA"})
	if err != nil {
		t.Fatal(err)
	}
	if got := shell.Regions[0].Components[0].Properties["title"]; got != "Metadata Title" {
		t.Fatalf("metadata default title = %q", got)
	}
	if len(diagnostics) != 1 || diagnostics[0].Code != "GLADELWC005" {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
}

func TestResolveLWCShellRequestRejectsWrongSupportedObject(t *testing.T) {
	root := t.TempDir()
	writeLWCShellServerTestFile(t, root, "force-app/main/default/lwc/contextProbe/contextProbe.js-meta.xml", `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata">
  <isExposed>true</isExposed>
  <targets><target>lightning__RecordPage</target></targets>
  <targetConfigs>
    <targetConfig targets="lightning__RecordPage">
      <objects><object>Contact</object></objects>
    </targetConfig>
  </targetConfigs>
</LightningComponentBundle>`)
	writeLWCShellServerTestFile(t, root, "force-app/main/default/flexipages/Account_Record_Page.flexipage-meta.xml", `<FlexiPage xmlns="http://soap.sforce.com/2006/04/metadata">
  <sobjectType>Account</sobjectType>
  <type>RecordPage</type>
  <flexiPageRegions>
    <name>main</name>
    <componentInstances>
      <componentName>contextProbe</componentName>
    </componentInstances>
  </flexiPageRegions>
</FlexiPage>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewWithSource(&storage.OrgState{}, SourceMetadata{Project: p})

	_, _, diagnostics, err := handler.resolveLWCShellRequest(httptest.NewRequest(http.MethodGet, "/lwc/preview/record/Account/001000000000001AAA?page=Account_Record_Page", nil), []string{"record", "Account", "001000000000001AAA"})
	if err == nil {
		t.Fatal("expected supported object error")
	}
	if len(diagnostics) != 1 || diagnostics[0].Code != "GLADELWC004" {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
}

func TestResolveLWCShellRequestValidatesDirectComponentRoute(t *testing.T) {
	root := t.TempDir()
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewWithSource(&storage.OrgState{}, SourceMetadata{Project: p})

	_, _, diagnostics, err := handler.resolveLWCShellRequest(httptest.NewRequest(http.MethodGet, "/lwc/preview/component/c/missingProbe", nil), []string{"component", "c", "missingProbe"})
	if err == nil {
		t.Fatal("expected missing direct component error")
	}
	if len(diagnostics) != 1 || diagnostics[0].Code != "GLADELWC005" {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
}

func TestResolveLWCShellRequestAppliesDirectComponentMetadataDefaults(t *testing.T) {
	root := t.TempDir()
	writeLWCShellServerTestFile(t, root, "force-app/main/default/lwc/contextProbe/contextProbe.js-meta.xml", `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata">
	  <isExposed>true</isExposed>
	  <targets><target>lightning__RecordPage</target></targets>
	  <targetConfigs>
	    <targetConfig targets="lightning__RecordPage">
	      <property name="title" type="String" default="Metadata Title"/>
	    </targetConfig>
	  </targetConfigs>
	</LightningComponentBundle>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewWithSource(&storage.OrgState{}, SourceMetadata{Project: p})

	shell, _, diagnostics, err := handler.resolveLWCShellRequest(httptest.NewRequest(http.MethodGet, "/lwc/preview/component/c/contextProbe", nil), []string{"component", "c", "contextProbe"})
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
	mounts := lwcShellMounts(shell)
	if len(mounts) != 1 {
		t.Fatalf("mounts = %#v", mounts)
	}
	if got := mounts[0].Attrs["title"]; got != "Metadata Title" {
		t.Fatalf("metadata title = %#v", got)
	}
}

func TestResolveLWCShellRequestServesUrlAddressableRoute(t *testing.T) {
	root := t.TempDir()
	writeLWCShellServerTestFile(t, root, "force-app/main/default/lwc/urlProbe/urlProbe.js-meta.xml", `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata">
	  <isExposed>true</isExposed>
	  <targets><target>lightning__UrlAddressable</target></targets>
	</LightningComponentBundle>`)
	writeLWCShellServerTestFile(t, root, "force-app/main/default/lwc/urlProbe/urlProbe.js", `import { LightningElement } from 'lwc';
export default class UrlProbe extends LightningElement {}`)
	writeLWCShellServerTestFile(t, root, "force-app/main/default/lwc/urlProbe/urlProbe.html", `<template><p>url</p></template>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewWithSource(&storage.OrgState{}, SourceMetadata{Project: p})

	shell, _, diagnostics, err := handler.resolveLWCShellRequest(httptest.NewRequest(http.MethodGet, "/lwc/preview/cmp/c/urlProbe?app=Sales&c__name=value", nil), []string{"cmp", "c", "urlProbe"})
	if err != nil {
		t.Fatalf("resolve error = %v diagnostics=%#v", err, diagnostics)
	}
	if shell.Context.ComponentName != "c:urlProbe" || shell.Context.AppName != "Sales" || shell.Context.State["c__name"] != "value" {
		t.Fatalf("context = %#v", shell.Context)
	}
	body := renderLWCShellHTML(lwcbrowser.PageConfig{}, shell)
	for _, want := range []string{
		`"componentName":"c:urlProbe"`,
		`"c__name":"value"`,
		`"standard__component"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in:\n%s", want, body)
		}
	}
}

func TestResolveLWCShellRequestRejectsInvalidUrlAddressableState(t *testing.T) {
	root := t.TempDir()
	writeLWCShellServerTestFile(t, root, "force-app/main/default/lwc/urlProbe/urlProbe.js-meta.xml", `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata">
	  <isExposed>true</isExposed>
	  <targets><target>lightning__UrlAddressable</target></targets>
	</LightningComponentBundle>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewWithSource(&storage.OrgState{}, SourceMetadata{Project: p})

	_, _, diagnostics, err := handler.resolveLWCShellRequest(httptest.NewRequest(http.MethodGet, "/lwc/preview/cmp/c/urlProbe?name=value", nil), []string{"cmp", "c", "urlProbe"})
	if err == nil {
		t.Fatalf("resolve error = nil, want invalid URL-addressable state")
	}
	if len(diagnostics) != 1 || diagnostics[0].Code != "GLADELWC071" {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
}

func TestResolveLWCShellRequestServesRecordQuickAction(t *testing.T) {
	root := t.TempDir()
	writeLWCShellServerTestFile(t, root, "force-app/main/default/quickActions/Account.Update_Status.quickAction-meta.xml", `<QuickAction xmlns="http://soap.sforce.com/2006/04/metadata">
	  <label>Update Status</label>
	  <type>LightningComponent</type>
	  <targetObject>Account</targetObject>
	  <lightningComponent>c:actionProbe</lightningComponent>
	</QuickAction>`)
	writeLWCShellServerTestFile(t, root, "force-app/main/default/lwc/actionProbe/actionProbe.js-meta.xml", `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata">
	  <isExposed>true</isExposed>
	  <targets><target>lightning__RecordAction</target></targets>
	  <targetConfigs>
	    <targetConfig targets="lightning__RecordAction">
	      <actionType>ScreenAction</actionType>
	    </targetConfig>
	  </targetConfigs>
	</LightningComponentBundle>`)
	writeLWCShellServerTestFile(t, root, "force-app/main/default/lwc/actionProbe/actionProbe.js", `import { LightningElement } from 'lwc';
export default class ActionProbe extends LightningElement {}`)
	writeLWCShellServerTestFile(t, root, "force-app/main/default/lwc/actionProbe/actionProbe.html", `<template><p>action</p></template>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewWithSource(&storage.OrgState{}, SourceMetadata{Project: p})

	shell, _, diagnostics, err := handler.resolveLWCShellRequest(httptest.NewRequest(http.MethodGet, "/lwc/preview/action/Account/001000000000001AAA/Update_Status", nil), []string{"action", "Account", "001000000000001AAA", "Update_Status"})
	if err != nil {
		t.Fatalf("resolve error = %v diagnostics=%#v", err, diagnostics)
	}
	mounts := lwcShellMounts(shell)
	if len(mounts) != 1 {
		t.Fatalf("mounts = %#v", mounts)
	}
	attrs := mounts[0].Attrs
	for key, want := range map[string]any{
		"recordId":      "001000000000001AAA",
		"objectApiName": "Account",
		"actionName":    "Account.Update_Status",
		"actionType":    "ScreenAction",
	} {
		if attrs[key] != want {
			t.Fatalf("attr %s = %#v, want %#v in %#v", key, attrs[key], want, attrs)
		}
	}
	body := renderLWCShellHTML(lwcbrowser.PageConfig{}, shell)
	if !strings.Contains(body, `"standard__quickAction"`) || !strings.Contains(body, `"actionName":"Account.Update_Status"`) {
		t.Fatalf("quick action context missing in:\n%s", body)
	}
}

func TestResolveLWCShellRequestServesGlobalQuickAction(t *testing.T) {
	root := t.TempDir()
	writeLWCShellServerTestFile(t, root, "force-app/main/default/quickActions/Global_Status.quickAction-meta.xml", `<QuickAction xmlns="http://soap.sforce.com/2006/04/metadata">
	  <label>Global Status</label>
	  <type>LightningComponent</type>
	  <lightningComponent>c:actionProbe</lightningComponent>
	</QuickAction>`)
	writeLWCShellServerTestFile(t, root, "force-app/main/default/lwc/actionProbe/actionProbe.js-meta.xml", `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata">
	  <isExposed>true</isExposed>
	  <targets><target>lightning__RecordAction</target></targets>
	  <targetConfigs>
	    <targetConfig targets="lightning__RecordAction">
	      <actionType>Action</actionType>
	    </targetConfig>
	  </targetConfigs>
	</LightningComponentBundle>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewWithSource(&storage.OrgState{}, SourceMetadata{Project: p})

	shell, _, diagnostics, err := handler.resolveLWCShellRequest(httptest.NewRequest(http.MethodGet, "/lwc/preview/action/global/Global_Status", nil), []string{"action", "global", "Global_Status"})
	if err != nil {
		t.Fatalf("resolve error = %v diagnostics=%#v", err, diagnostics)
	}
	mounts := lwcShellMounts(shell)
	if len(mounts) != 1 {
		t.Fatalf("mounts = %#v", mounts)
	}
	if mounts[0].Attrs["actionName"] != "Global_Status" || mounts[0].Attrs["actionType"] != "Action" {
		t.Fatalf("attrs = %#v", mounts[0].Attrs)
	}
}

func TestResolveLWCShellRequestAddsConsoleApproximationDiagnostic(t *testing.T) {
	root := t.TempDir()
	writeLWCShellServerTestFile(t, root, "force-app/main/default/applications/Support_Console.app-meta.xml", `<CustomApplication xmlns="http://soap.sforce.com/2006/04/metadata">
	  <label>Support Console</label>
	  <navType>Console</navType>
	  <tabs>Support_Page</tabs>
	</CustomApplication>`)
	writeLWCShellServerTestFile(t, root, "force-app/main/default/flexipages/Support_Page.flexipage-meta.xml", `<FlexiPage xmlns="http://soap.sforce.com/2006/04/metadata">
	  <type>AppPage</type>
	  <flexiPageRegions><name>main</name></flexiPageRegions>
	</FlexiPage>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewWithSource(&storage.OrgState{}, SourceMetadata{Project: p})

	shell, _, diagnostics, err := handler.resolveLWCShellRequest(httptest.NewRequest(http.MethodGet, "/lwc/preview/app/Support_Page?app=Support_Console", nil), []string{"app", "Support_Page"})
	if err != nil {
		t.Fatalf("resolve error = %v diagnostics=%#v", err, diagnostics)
	}
	if len(diagnostics) != 1 || diagnostics[0].Code != "GLADELWC072" {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
	body := renderLWCShellHTML(lwcbrowser.PageConfig{}, shell)
	if !strings.Contains(body, "GLADELWC072") {
		t.Fatalf("console diagnostic missing in:\n%s", body)
	}
}

func TestLWCShellDiagnosticsTreatConsoleApproximationAsNonFatal(t *testing.T) {
	if lwcShellDiagnosticsBlockEmptyMounts([]lwcshell.Diagnostic{{Code: "GLADELWC072"}}) {
		t.Fatalf("GLADELWC072 blocked an empty-mount console approximation")
	}
	if !lwcShellDiagnosticsBlockEmptyMounts([]lwcshell.Diagnostic{{Code: "GLADELWC005"}}) {
		t.Fatalf("missing component diagnostic did not block empty mounts")
	}
}

func TestResolveLWCShellRequestKeepsLWCTabPageReference(t *testing.T) {
	root := t.TempDir()
	writeLWCShellServerTestFile(t, root, "force-app/main/default/lwc/contextProbe/contextProbe.js-meta.xml", `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata">
	  <isExposed>true</isExposed>
	  <targets><target>lightning__Tab</target></targets>
	</LightningComponentBundle>`)
	writeLWCShellServerTestFile(t, root, "force-app/main/default/tabs/Lwc_Probe.tab-meta.xml", `<CustomTab xmlns="http://soap.sforce.com/2006/04/metadata">
	  <label>LWC Probe</label>
	  <lwcComponent>c:contextProbe</lwcComponent>
	</CustomTab>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewWithSource(&storage.OrgState{}, SourceMetadata{Project: p})

	shell, _, diagnostics, err := handler.resolveLWCShellRequest(httptest.NewRequest(http.MethodGet, "/lwc/preview/tab/Lwc_Probe", nil), []string{"tab", "Lwc_Probe"})
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
	if shell.Context.Kind != lwcshell.RenderTargetTab || shell.Context.ComponentName != "c:contextProbe" {
		t.Fatalf("context = %#v", shell.Context)
	}
	ref := lwcShellPageReference(shell)
	if ref["type"] != "standard__navItemPage" {
		t.Fatalf("page reference = %#v", ref)
	}
	attrs, ok := ref["attributes"].(map[string]any)
	if !ok || attrs["apiName"] != "Lwc_Probe" {
		t.Fatalf("page reference attrs = %#v", ref["attributes"])
	}
	mounts := lwcShellMounts(shell)
	if len(mounts) != 1 || mounts[0].Qualified != "c:contextProbe" {
		t.Fatalf("mounts = %#v", mounts)
	}
}

func TestResolveLWCShellRequestRejectsLWCTabWithoutTabTarget(t *testing.T) {
	root := t.TempDir()
	writeLWCShellServerTestFile(t, root, "force-app/main/default/lwc/contextProbe/contextProbe.js-meta.xml", `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata">
	  <isExposed>true</isExposed>
	  <targets><target>lightning__RecordPage</target></targets>
	</LightningComponentBundle>`)
	writeLWCShellServerTestFile(t, root, "force-app/main/default/lwc/contextProbe/contextProbe.js", `import { LightningElement } from 'lwc';
export default class ContextProbe extends LightningElement {}`)
	writeLWCShellServerTestFile(t, root, "force-app/main/default/lwc/contextProbe/contextProbe.html", `<template><p>context</p></template>`)
	writeLWCShellServerTestFile(t, root, "force-app/main/default/tabs/Lwc_Probe.tab-meta.xml", `<CustomTab xmlns="http://soap.sforce.com/2006/04/metadata">
	  <label>LWC Probe</label>
	  <lwcComponent>c:contextProbe</lwcComponent>
	</CustomTab>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewWithSource(&storage.OrgState{}, SourceMetadata{Project: p})

	_, _, diagnostics, err := handler.resolveLWCShellRequest(httptest.NewRequest(http.MethodGet, "/lwc/preview/tab/Lwc_Probe", nil), []string{"tab", "Lwc_Probe"})
	if err == nil {
		t.Fatal("expected unsupported LWC tab target error")
	}
	if len(diagnostics) != 1 || diagnostics[0].Code != "GLADELWC005" || !strings.Contains(diagnostics[0].Message, "lightning__Tab") {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
}

func TestResolveLWCShellRequestRendersFlexiPageTabUsingTargetPageType(t *testing.T) {
	root, err := lightningTestRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	fixture := filepath.Join(root, "testdata", "local-tests", "lwc-shell")
	p, err := project.Load(fixture)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewWithSource(&storage.OrgState{}, SourceMetadata{Project: p})

	shell, _, diagnostics, err := handler.resolveLWCShellRequest(httptest.NewRequest(http.MethodGet, "/lwc/preview/tab/Lwc_Probe", nil), []string{"tab", "Lwc_Probe"})
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
	if shell.Context.Kind != lwcshell.RenderTargetTab || shell.Context.TabName != "Lwc_Probe" {
		t.Fatalf("context = %#v", shell.Context)
	}
	if shell.Page.Type != "AppPage" {
		t.Fatalf("page type = %q", shell.Page.Type)
	}
	ref := lwcShellPageReference(shell)
	if ref["type"] != "standard__navItemPage" {
		t.Fatalf("page reference = %#v", ref)
	}
	if mounts := lwcShellMounts(shell); len(mounts) != 2 {
		t.Fatalf("mounts = %#v", mounts)
	}
}

func TestResolveLWCShellRequestUsesHomePageReference(t *testing.T) {
	root := t.TempDir()
	writeLWCShellServerTestFile(t, root, "force-app/main/default/lwc/contextProbe/contextProbe.js-meta.xml", `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata">
	  <isExposed>true</isExposed>
	  <targets><target>lightning__HomePage</target></targets>
	</LightningComponentBundle>`)
	writeLWCShellServerTestFile(t, root, "force-app/main/default/flexipages/Custom_Home.flexipage-meta.xml", `<FlexiPage xmlns="http://soap.sforce.com/2006/04/metadata">
	  <type>HomePage</type>
	  <flexiPageRegions>
	    <name>main</name>
	    <componentInstances>
	      <componentName>contextProbe</componentName>
	    </componentInstances>
	  </flexiPageRegions>
	</FlexiPage>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewWithSource(&storage.OrgState{}, SourceMetadata{Project: p})

	shell, _, diagnostics, err := handler.resolveLWCShellRequest(httptest.NewRequest(http.MethodGet, "/lwc/preview/home/Custom_Home", nil), []string{"home", "Custom_Home"})
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
	ref := lwcShellPageReference(shell)
	if ref["type"] != "standard__namedPage" {
		t.Fatalf("page reference = %#v", ref)
	}
	attrs, ok := ref["attributes"].(map[string]any)
	if !ok || attrs["pageName"] != "home" {
		t.Fatalf("page reference attrs = %#v", ref["attributes"])
	}
}

func TestLWCShellUnsupportedCustomTabReturnsDiagnostic(t *testing.T) {
	root := t.TempDir()
	writeLWCShellServerTestFile(t, root, "force-app/main/default/tabs/External.tab-meta.xml", `<CustomTab xmlns="http://soap.sforce.com/2006/04/metadata">
	  <label>External</label>
	  <url>https://example.com</url>
	</CustomTab>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewWithSource(&storage.OrgState{}, SourceMetadata{Project: p})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/lwc/preview/tab/External", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "text/html") {
		t.Fatalf("content-type = %q body=%s", got, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "GLADELWC007") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestLWCShellUnsupportedComponentReturnsDiagnostics(t *testing.T) {
	root := t.TempDir()
	writeLWCShellServerTestFile(t, root, "force-app/main/default/flexipages/Account_Record_Page.flexipage-meta.xml", `<FlexiPage xmlns="http://soap.sforce.com/2006/04/metadata">
  <sobjectType>Account</sobjectType>
  <type>RecordPage</type>
  <flexiPageRegions>
    <name>main</name>
    <componentInstances>
      <componentName>missingProbe</componentName>
    </componentInstances>
  </flexiPageRegions>
</FlexiPage>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewWithSource(&storage.OrgState{}, SourceMetadata{Project: p})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/lwc/preview/record/Account/001000000000001AAA?page=Account_Record_Page", nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "GLADELWC005") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestLWCShellMixedPageDiagnosticsStillRendersValidComponents(t *testing.T) {
	root := t.TempDir()
	writeLWCShellServerTestFile(t, root, "sfdx-project.json", `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"61.0"}`)
	writeLWCShellServerTestFile(t, root, "force-app/main/default/lwc/contextProbe/contextProbe.js-meta.xml", `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata">
	  <isExposed>true</isExposed>
	  <targets><target>lightning__RecordPage</target></targets>
	</LightningComponentBundle>`)
	writeLWCShellServerTestFile(t, root, "force-app/main/default/lwc/contextProbe/contextProbe.js", `import { LightningElement } from 'lwc';
export default class ContextProbe extends LightningElement {}`)
	writeLWCShellServerTestFile(t, root, "force-app/main/default/lwc/contextProbe/contextProbe.html", `<template><p>context</p></template>`)
	writeLWCShellServerTestFile(t, root, "force-app/main/default/flexipages/Account_Record_Page.flexipage-meta.xml", `<FlexiPage xmlns="http://soap.sforce.com/2006/04/metadata">
	  <sobjectType>Account</sobjectType>
	  <type>RecordPage</type>
	  <flexiPageRegions>
	    <name>main</name>
	    <componentInstances>
	      <componentName>contextProbe</componentName>
	    </componentInstances>
	    <componentInstances>
	      <componentName>missingProbe</componentName>
	    </componentInstances>
	  </flexiPageRegions>
	</FlexiPage>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	source, err := NewSourceMetadataFromProject(p)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewWithSource(&storage.OrgState{}, source)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/lwc/preview/record/Account/001000000000001AAA?page=Account_Record_Page", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{"c:contextProbe", "GLADELWC005", "missingProbe"} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in:\n%s", want, body)
		}
	}
}

func TestLWCShellKeepsAuraAndPlatformComponentsAsPlaceholders(t *testing.T) {
	root := t.TempDir()
	writeLWCShellServerTestFile(t, root, "sfdx-project.json", `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"PKG","sourceApiVersion":"61.0"}`)
	writeLWCShellServerTestFile(t, root, "force-app/main/default/lwc/contextProbe/contextProbe.js-meta.xml", `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata">
	  <isExposed>true</isExposed>
	  <targets><target>lightning__AppPage</target></targets>
	</LightningComponentBundle>`)
	writeLWCShellServerTestFile(t, root, "force-app/main/default/lwc/contextProbe/contextProbe.js", `import { LightningElement } from 'lwc';
export default class ContextProbe extends LightningElement {}`)
	writeLWCShellServerTestFile(t, root, "force-app/main/default/lwc/contextProbe/contextProbe.html", `<template><p>context</p></template>`)
	writeLWCShellServerTestFile(t, root, "force-app/main/default/aura/BusinessEventManager/BusinessEventManager.cmp", `<aura:component/>`)
	writeLWCShellServerTestFile(t, root, "force-app/main/default/flexipages/BusinessEvents.flexipage-meta.xml", `<FlexiPage xmlns="http://soap.sforce.com/2006/04/metadata">
	  <type>AppPage</type>
	  <flexiPageRegions>
	    <name>main</name>
	    <itemInstances><componentInstance><componentName>BusinessEventManager</componentName></componentInstance></itemInstances>
	    <itemInstances><componentInstance><componentName>flexipage:richText</componentName></componentInstance></itemInstances>
	    <itemInstances><componentInstance><componentName>contextProbe</componentName></componentInstance></itemInstances>
	  </flexiPageRegions>
	</FlexiPage>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	source, err := NewSourceMetadataFromProject(p)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewWithSource(&storage.OrgState{}, source)

	shell, _, diagnostics, err := handler.resolveLWCShellRequest(httptest.NewRequest(http.MethodGet, "/lwc/preview/app/BusinessEvents", nil), []string{"app", "BusinessEvents"})
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
	if len(shell.Regions) != 1 || len(shell.Regions[0].Components) != 3 {
		t.Fatalf("regions = %#v", shell.Regions)
	}
	if got := shell.Regions[0].Components[0]; got.Kind != "aura" || !strings.Contains(got.UnsupportedReason, "Aura") {
		t.Fatalf("aura placeholder = %#v", got)
	}
	if got := shell.Regions[0].Components[1]; got.Kind != "platform" || !strings.Contains(got.UnsupportedReason, "Salesforce") {
		t.Fatalf("platform placeholder = %#v", got)
	}
	if got := shell.Regions[0].Components[2]; got.ComponentName != "PKG:contextProbe" || got.Kind != "" {
		t.Fatalf("lwc component = %#v", got)
	}
	if mounts := lwcShellMounts(shell); len(mounts) != 1 || mounts[0].Qualified != "PKG:contextProbe" {
		t.Fatalf("mounts = %#v", mounts)
	}
	body := renderLWCShellHTML(lwcbrowser.PageConfig{}, shell)
	for _, want := range []string{"glade-placeholder", "PKG:BusinessEventManager", "flexipage:richText", "PKG:contextProbe"} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in:\n%s", want, body)
		}
	}
	if strings.Contains(body, "GLADELWC005") {
		t.Fatalf("placeholder page leaked missing-component diagnostic:\n%s", body)
	}
}

func TestLWCShellPlaceholderOnlyPageRendersEmptyMountList(t *testing.T) {
	shell := lwcshell.ShellPage{
		Context: lwcshell.PageContext{Kind: lwcshell.RenderTargetAppPage, PageName: "BusinessEvents"},
		Regions: []lwcshell.PageRegion{{
			Name: "main",
			Components: []lwcshell.PageComponent{{
				ComponentName:     "PKG:BusinessEventManager",
				Kind:              "aura",
				UnsupportedReason: "Aura component is shown as a local placeholder in LWC preview.",
			}},
		}},
	}
	if mounts := lwcShellMounts(shell); len(mounts) != 0 {
		t.Fatalf("mounts = %#v", mounts)
	}
	body := renderLWCShellHTML(lwcbrowser.PageConfig{}, shell)
	if !strings.Contains(body, `var mounts=[]`) {
		t.Fatalf("mount script did not render an empty list:\n%s", body)
	}
}

func TestLWCShellComponentRouteServesHTML(t *testing.T) {
	if _, err := os.Stat(filepath.Join("..", "..", "third_party", "lwc", "node_modules")); err != nil {
		t.Skip("npm install required in third_party/lwc")
	}
	root, err := lightningTestRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	fixture := filepath.Join(root, "testdata", "local-tests", "lwc-shell")
	p, err := project.Load(fixture)
	if err != nil {
		t.Fatal(err)
	}
	source, err := NewSourceMetadataFromProject(p)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	handler := NewWithSource(&org, source)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/lwc/preview/component/c/contextProbe?recordId=001000000000001AAA&objectApiName=Account", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		"Glade LWC Shell",
		"c:contextProbe",
		"001000000000001AAA",
		"/lightning/glade.out.js",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in:\n%s", want, body)
		}
	}
}

func TestLWCShellVisualforceTabRedirectsToApexPage(t *testing.T) {
	root := t.TempDir()
	tabPath := writeLWCShellServerTestFile(t, root, "force-app/main/default/tabs/Legacy_VF.tab-meta.xml", `<CustomTab xmlns="http://soap.sforce.com/2006/04/metadata">
  <label>Legacy VF</label>
  <page>LegacyPage</page>
</CustomTab>`)
	p := project.Project{Root: root, TabFiles: []string{tabPath}}
	handler := NewWithSource(&storage.OrgState{}, SourceMetadata{Project: p})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/lwc/preview/tab/Legacy_VF", nil))

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Location"); got != "/apex/LegacyPage" {
		t.Fatalf("Location = %q", got)
	}
}

func writeLWCShellServerTestFile(t *testing.T, root string, rel string, body string) string {
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

func writeCommunityProbeBundle(t *testing.T, root, target string) {
	t.Helper()
	writeLWCShellServerTestFile(t, root, "force-app/main/default/lwc/communityProbe/communityProbe.js", `import { LightningElement } from 'lwc';
export default class CommunityProbe extends LightningElement {}`)
	writeLWCShellServerTestFile(t, root, "force-app/main/default/lwc/communityProbe/communityProbe.html", `<template><p>community</p></template>`)
	writeLWCShellServerTestFile(t, root, "force-app/main/default/lwc/communityProbe/communityProbe.js-meta.xml", `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata">
	  <isExposed>true</isExposed>
	  <targets><target>`+target+`</target></targets>
	</LightningComponentBundle>`)
}
