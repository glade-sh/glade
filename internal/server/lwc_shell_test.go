package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/lwcshell"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

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
	if shell.Page.Type != "HomePage" {
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
