package visualforce

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/open-aer/oaer/internal/project"
	"github.com/open-aer/oaer/internal/storage"
)

func TestLoadProjectIndexesPagesAndComponents(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/Edit.page"), `<apex:page controller="EditController" action="{!load}">
  <apex:stylesheet value="{!URLFOR($Resource.SiteAssets, 'site.css')}" />
  <apex:outputText value="{!$Label.EditTitle}" />
  <apex:outputText value="{!$ObjectType.Account.fields.Name.label}" />
  <apex:composition template="{!$Site.Template}" />
</apex:page>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/AccountView.page"), `<apex:page standardController="Account" extensions="AccountExt, AuditExt">
  {!$Resource.Logo}
</apex:page>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/components/Picker.component"), `<apex:component controller="PickerController">
  <apex:attribute name="value" type="String" assignTo="{!selectedValue}" required="true" description="Selected value"/>
  {!$Label.PickerHelp}
</apex:component>`)

	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}

	if len(idx.Pages) != 2 || len(idx.Components) != 1 {
		t.Fatalf("unexpected visualforce inventory: %#v", idx)
	}
	edit, ok := idx.Page("Page.Edit")
	if !ok || edit.Controller != "EditController" || edit.Action != "{!load}" {
		t.Fatalf("controller page lookup = %#v, %v", edit, ok)
	}
	if ref, ok := idx.PageReference("Page.Edit"); !ok || ref.Name != "Edit" {
		t.Fatalf("page reference lookup = %#v, %v", ref, ok)
	}
	if ref, ok := idx.PageReference("Page.pkg__Edit"); !ok || ref.Name != "Edit" {
		t.Fatalf("namespaced page reference lookup = %#v, %v", ref, ok)
	}
	if ref, ok := idx.PageFile(filepath.Join(root, "force-app/main/default/pages/Edit.page")); !ok || ref.Name != "Edit" {
		t.Fatalf("page file lookup = %#v, %v", ref, ok)
	}
	if !idx.HasPageReference("Page.Edit") || idx.HasPageReference("Page.Missing") {
		t.Fatalf("page readiness lookup failed: %#v", idx.pagesByName)
	}
	if !hasMerge(edit.MergeReferences, "URLFOR", "$Resource", "SiteAssets") {
		t.Fatalf("missing URLFOR resource ref: %#v", edit.MergeReferences)
	}
	if !hasMerge(edit.MergeReferences, "Label", "$Label", "EditTitle") {
		t.Fatalf("missing label ref: %#v", edit.MergeReferences)
	}
	if !hasMerge(edit.MergeReferences, "ObjectType", "$ObjectType", "Account.fields.Name.label") {
		t.Fatalf("missing object type ref: %#v", edit.MergeReferences)
	}
	if !hasMerge(edit.MergeReferences, "Site", "$Site", "Template") {
		t.Fatalf("missing site ref: %#v", edit.MergeReferences)
	}

	account, ok := idx.Page("AccountView")
	if !ok || account.StandardController != "Account" || len(account.Extensions) != 2 || account.Extensions[0] != "AccountExt" || account.Extensions[1] != "AuditExt" {
		t.Fatalf("standard controller page = %#v, %v", account, ok)
	}
	if !hasMerge(account.MergeReferences, "StaticResource", "$Resource", "Logo") {
		t.Fatalf("missing direct resource ref: %#v", account.MergeReferences)
	}

	component, ok := idx.Component("picker")
	if !ok || component.Controller != "PickerController" || len(component.Attributes) != 1 {
		t.Fatalf("component lookup = %#v, %v", component, ok)
	}
	attribute := component.Attributes[0]
	if attribute.Name != "value" || attribute.Type != "String" || attribute.AssignTo != "{!selectedValue}" || attribute.Required != "true" {
		t.Fatalf("component attribute = %#v", attribute)
	}
	if !hasMerge(attribute.MergeReferences, "ControllerExpression", "", "selectedValue") {
		t.Fatalf("missing assignTo ref: %#v", attribute.MergeReferences)
	}
	if !hasMerge(component.MergeReferences, "Label", "$Label", "PickerHelp") {
		t.Fatalf("missing component label ref: %#v", component.MergeReferences)
	}
}

func TestExtractMergeReferences(t *testing.T) {
	refs := ExtractMergeReferences(`{!URLFOR($Resource.Bundle, 'x.css')} {!$Site.BaseUrl}`)
	if len(refs) != 2 {
		t.Fatalf("refs = %#v", refs)
	}
	if !hasMerge(refs, "URLFOR", "$Resource", "Bundle") || !hasMerge(refs, "Site", "$Site", "BaseUrl") {
		t.Fatalf("classified refs = %#v", refs)
	}
}

func TestLoadProjectBestEffortKeepsLenientMarkup(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/Good.page"), `<apex:page controller="GoodController" />`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/Broken.page"), `<apex:page><apex:outputText>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/components/Good.component"), `<apex:component>
  <apex:attribute name="actSupAction" type="ApexPages.Action" />
</apex:component>`)

	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := idx.Page("Good"); !ok {
		t.Fatalf("strict index missed lenient page: %#v", idx)
	}
	if _, ok := idx.Page("Broken"); !ok {
		t.Fatalf("strict index missed XML-hostile page: %#v", idx)
	}
	component, ok := idx.Component("Good")
	if !ok || len(component.Attributes) != 1 || component.Attributes[0].Name != "actSupAction" {
		t.Fatalf("strict index missed parseable component: %#v", idx)
	}
}

func TestParseComponentToleratesVisualforceMarkupThatIsNotStrictXML(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "BulkBilling.component")
	writeFile(t, path, `<apex:component controller="BulkBillingController">
  <apex:attribute name="cancelAction" type="ApexPages.Action" required="true"
    description="The action to execute." />
  <apex:commandButton action="{!cancelAction}" />
  <div>{!IF(ISBLANK(city), '', city & ',')}</div>
  <h1>{!c.Subheader}&nbsp;</h1>
  <script>
    if (event.status && event.result != null) {
      poll();
    }
  </script>
</apex:component>`)

	component, err := ParseComponentFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if component.Controller != "BulkBillingController" || len(component.Attributes) != 1 {
		t.Fatalf("component metadata = %#v", component)
	}
	if component.Attributes[0].Name != "cancelAction" || component.Attributes[0].Type != "ApexPages.Action" {
		t.Fatalf("component attribute = %#v", component.Attributes[0])
	}
	if !hasMerge(component.MergeReferences, "ControllerExpression", "", "cancelAction") {
		t.Fatalf("missing action merge reference: %#v", component.MergeReferences)
	}
}

func TestResolveResourceURL(t *testing.T) {
	registry := storage.MetadataRegistry{
		StaticResources: []storage.StaticResourceMetadata{{Name: "Bundle", URL: "/resource/Bundle"}},
		ContentAssets:   []storage.ContentAssetMetadata{{Name: "Logo", URL: "/sfc/servlet.shepherd/version/download/Logo"}},
	}
	if got, ok := ResolveResourceURL(registry, `URLFOR($Resource.Bundle, 'x.css')`); !ok || got != "/resource/Bundle/x.css" {
		t.Fatalf("URLFOR = %q, %v", got, ok)
	}
	if got, ok := ResolveResourceURL(registry, `$Resource.Logo`); !ok || got != "/sfc/servlet.shepherd/version/download/Logo" {
		t.Fatalf("$Resource = %q, %v", got, ok)
	}
}

func hasMerge(refs []MergeReference, kind, root, name string) bool {
	for _, ref := range refs {
		if ref.Kind == kind && ref.Root == root && ref.Name == name {
			return true
		}
	}
	return false
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
