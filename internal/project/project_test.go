package project

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/namespaceremap"
)

func TestLoadSFDXProject(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{
  "packageDirectories": [{"path":"force-app","default":true}],
  "namespace": "pkg",
  "sourceApiVersion": "61.0"
}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/Hello.cls"), "public class Hello {}")
	writeFile(t, filepath.Join(root, "force-app/main/triggers/Hello.trigger"), "trigger Hello on Account (before insert) {}")
	writeFile(t, filepath.Join(root, "force-app/main/objects/Thing__c/Thing__c.object-meta.xml"), "<CustomObject/>")
	writeFile(t, filepath.Join(root, "force-app/main/objects/Legacy__c.object"), "<CustomObject/>")
	writeFile(t, filepath.Join(root, "force-app/main/objects/Thing__c/fields/Name__c.field-meta.xml"), "<CustomField/>")
	writeFile(t, filepath.Join(root, "force-app/main/objects/Thing__c/fieldSets/Summary.fieldSet-meta.xml"), "<FieldSet/>")
	writeFile(t, filepath.Join(root, "force-app/main/objects/Thing__c/recordTypes/Business.recordType-meta.xml"), "<RecordType/>")
	writeFile(t, filepath.Join(root, "force-app/main/objects/Thing__c/validationRules/Block.validationRule-meta.xml"), "<ValidationRule/>")
	writeFile(t, filepath.Join(root, "force-app/main/labels/CustomLabels.labels"), "<CustomLabels/>")
	writeFile(t, filepath.Join(root, "force-app/main/translations/fr.translation-meta.xml"), "<Translations/>")
	writeFile(t, filepath.Join(root, "force-app/main/staticresources/Site.resource"), "body")
	writeFile(t, filepath.Join(root, "force-app/main/staticresources/Site.resource-meta.xml"), "<StaticResource/>")
	writeFile(t, filepath.Join(root, "force-app/main/staticresources/Reset.css"), "body")
	writeFile(t, filepath.Join(root, "force-app/main/staticresources/Reset.resource-meta.xml"), "<StaticResource/>")
	writeFile(t, filepath.Join(root, "force-app/main/contentassets/Logo.asset"), "asset body")
	writeFile(t, filepath.Join(root, "force-app/main/contentassets/Logo.asset-meta.xml"), "<ContentAsset/>")
	writeFile(t, filepath.Join(root, "force-app/main/email/welcome.email"), "Welcome body")
	writeFile(t, filepath.Join(root, "force-app/main/email/welcome.email-meta.xml"), "<EmailTemplate/>")
	writeFile(t, filepath.Join(root, "force-app/main/documents/GLExport.documentFolder-meta.xml"), "<DocumentFolder/>")
	writeFile(t, filepath.Join(root, "force-app/main/namedCredentials/Api.namedCredential"), "<NamedCredential/>")
	writeFile(t, filepath.Join(root, "force-app/main/remoteSiteSettings/Api.remoteSite"), "<RemoteSiteSetting/>")
	writeFile(t, filepath.Join(root, "force-app/main/customMetadata/Feature.Default.md"), "<CustomMetadata/>")
	writeFile(t, filepath.Join(root, "force-app/main/customMetadata/Feature.Modern.md-meta.xml"), "<CustomMetadata/>")
	writeFile(t, filepath.Join(root, "force-app/main/api/rest/Feature.Nested.md-meta.xml"), "<CustomMetadata/>")
	writeFile(t, filepath.Join(root, "force-app/main/workflows/Thing__c.workflow-meta.xml"), "<Workflow/>")
	writeFile(t, filepath.Join(root, "force-app/main/workflows/Legacy__c.workflow"), "<Workflow/>")
	writeFile(t, filepath.Join(root, "force-app/main/flows/Onboard.flow-meta.xml"), "<Flow/>")
	writeFile(t, filepath.Join(root, "force-app/main/profiles/Admin.profile-meta.xml"), "<Profile/>")
	writeFile(t, filepath.Join(root, "force-app/main/permissionsets/App.permissionset-meta.xml"), "<PermissionSet/>")
	writeFile(t, filepath.Join(root, "force-app/main/permissionsetgroups/App.permissionsetgroup-meta.xml"), "<PermissionSetGroup/>")
	writeFile(t, filepath.Join(root, "force-app/main/permissionSetAssignments/App.permissionsetassignment-meta.xml"), "<PermissionSetAssignment/>")
	writeFile(t, filepath.Join(root, "force-app/main/objects/Thing__c/listViews/All.listView-meta.xml"), "<ListView/>")
	writeFile(t, filepath.Join(root, "force-app/main/layouts/Thing__c-Thing Layout.layout-meta.xml"), "<Layout/>")
	writeFile(t, filepath.Join(root, "force-app/main/objects/Thing__c/compactLayouts/Card.compactLayout-meta.xml"), "<CompactLayout/>")
	writeFile(t, filepath.Join(root, "force-app/main/tabs/Thing__c.tab-meta.xml"), "<CustomTab/>")
	writeFile(t, filepath.Join(root, "force-app/main/objects/Thing__c/webLinks/Open.webLink-meta.xml"), "<WebLink/>")
	writeFile(t, filepath.Join(root, "force-app/main/quickActions/Thing__c.New.quickAction-meta.xml"), "<QuickAction/>")
	writeFile(t, filepath.Join(root, "force-app/main/globalValueSets/Status.globalValueSet-meta.xml"), "<GlobalValueSet/>")
	writeFile(t, filepath.Join(root, "force-app/main/standardValueSets/CaseStatus.standardValueSet-meta.xml"), "<StandardValueSet/>")
	writeFile(t, filepath.Join(root, "force-app/main/flexipages/Home.flexipage-meta.xml"), "<FlexiPage/>")
	writeFile(t, filepath.Join(root, "force-app/main/applications/Console.app-meta.xml"), "<CustomApplication/>")
	writeFile(t, filepath.Join(root, "force-app/main/pages/Edit.page"), `<apex:page controller="EditController"/>`)
	writeFile(t, filepath.Join(root, "force-app/main/components/Picker.component"), `<apex:component/>`)
	writeFile(t, filepath.Join(root, "force-app/main/aura/Widget/Widget.cmp"), `<aura:component controller="WidgetController"/>`)
	writeFile(t, filepath.Join(root, "force-app/main/aura/Widget/WidgetController.js"), `({ save: function(cmp) { cmp.get("c.save"); } })`)
	writeFile(t, filepath.Join(root, "force-app/main/lwc/widget/widget.js"), `import save from '@salesforce/apex/WidgetController.save';`)
	writeFile(t, filepath.Join(root, "force-app/main/lwc/widget/widget.html"), `<template></template>`)
	writeFile(t, filepath.Join(root, "force-app/main/docs/README.md"), "# not metadata")

	p, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if p.Namespace != "pkg" || p.SourceAPIVersion != "61.0" {
		t.Fatalf("project metadata = %#v", p)
	}
	if len(p.ApexFiles) != 2 || len(p.ObjectFiles) != 2 || len(p.FieldFiles) != 1 || len(p.FieldSetFiles) != 1 || len(p.RecordTypeFiles) != 1 || len(p.ValidationRuleFiles) != 1 {
		t.Fatalf("unexpected file counts: %#v", p)
	}
	if len(p.LabelFiles) != 1 || len(p.TranslationFiles) != 1 || len(p.StaticResourceFiles) != 2 || len(p.StaticResourceMetas) != 2 || len(p.ContentAssetFiles) != 1 || len(p.ContentAssetMetas) != 1 || len(p.EmailTemplateFiles) != 2 || len(p.FolderFiles) != 1 || len(p.NamedCredentialFiles) != 1 || len(p.RemoteSiteFiles) != 1 || len(p.CustomMetadataFiles) != 3 {
		t.Fatalf("unexpected legacy metadata file counts: %#v", p)
	}
	if len(p.WorkflowFiles) != 2 {
		t.Fatalf("unexpected workflow file counts: %#v", p)
	}
	if len(p.FlowFiles) != 1 || len(p.ProfileFiles) != 1 || len(p.PermissionSetFiles) != 1 || len(p.PermissionSetGroupFiles) != 1 || len(p.PermissionAssignmentFiles) != 1 {
		t.Fatalf("unexpected metadata stub file counts: %#v", p)
	}
	if len(p.ListViewFiles) != 1 || len(p.LayoutFiles) != 1 || len(p.CompactLayoutFiles) != 1 {
		t.Fatalf("unexpected layout/list view file counts: %#v", p)
	}
	if len(p.TabFiles) != 1 || len(p.WebLinkFiles) != 1 || len(p.QuickActionFiles) != 1 || len(p.GlobalValueSetFiles) != 1 || len(p.StandardValueSetFiles) != 1 || len(p.FlexiPageFiles) != 1 || len(p.ApplicationFiles) != 1 {
		t.Fatalf("unexpected presentation metadata file counts: %#v", p)
	}
	if len(p.VisualforcePageFiles) != 1 || len(p.VisualforceComponentFiles) != 1 {
		t.Fatalf("unexpected visualforce file counts: %#v", p)
	}
	if len(p.AuraFiles) != 2 || len(p.LWCFiles) != 1 {
		t.Fatalf("unexpected UI controller file counts: %#v", p)
	}
}

func TestNormalizeApexNamespaceTokensForUnnamespacedProject(t *testing.T) {
	source := `public class UsesTokens {
  public void run() {
    %%%NAMESPACE_DOT%%%UTIL_CustomSettings_API.getSettingsForTests(
      new %%%NAMESPACE%%%Hierarchy_Settings__c(%%%NAMESPACE%%%Disable_Preferred_Email_Enforcement__c = false)
    );
    for (%%%NAMESPACE_DOT%%%TDTM_Global_API.TdtmToken token : %%%NAMESPACE_DOT%%%TDTM_Global_API.getTdtmConfig()) {
    }
  }
}`
	got := NormalizeApexNamespaceTokens(source, "")
	if strings.Contains(got, "%%%") {
		t.Fatalf("namespace token remained in source:\n%s", got)
	}
	if !strings.Contains(got, "UTIL_CustomSettings_API.getSettingsForTests") {
		t.Fatalf("dot token was not removed:\n%s", got)
	}
	if !strings.Contains(got, "new Hierarchy_Settings__c(Disable_Preferred_Email_Enforcement__c = false)") {
		t.Fatalf("API token was not removed:\n%s", got)
	}
}

func TestNormalizeApexNamespaceTokensForNamespacedProject(t *testing.T) {
	source := `public class UsesTokens {
  public void run() {
    %%%NAMESPACE_DOT%%%UTIL_CustomSettings_API.getSettingsForTests(
      new %%%NAMESPACE%%%Hierarchy_Settings__c(%%%NAMESPACE%%%Disable_Preferred_Email_Enforcement__c = false)
    );
  }
}`
	got := NormalizeApexNamespaceTokens(source, " hed ")
	for _, want := range []string{
		"hed.UTIL_CustomSettings_API.getSettingsForTests",
		"new hed__Hierarchy_Settings__c(hed__Disable_Preferred_Email_Enforcement__c = false)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("normalized source missing %q:\n%s", want, got)
		}
	}
}

func TestOrgShapeFeaturesLoadsScratchDefinition(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "glade.yml"), "org:\n  features: [MultiCurrency]\n")
	writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), `{
  "features": ["PersonAccounts", "AddCustomApps:30"],
  "settings": {
    "communitiesSettings": {"enableNetworksEnabled": true},
    "chatterSettings": {"enableChatter": true}
  }
}`)

	got := OrgShapeFeatures(root)
	want := []string{"MultiCurrency", "PersonAccounts", "AddCustomApps:30", "Communities", "Chatter"}
	if len(got) != len(want) {
		t.Fatalf("features = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("features = %#v, want %#v", got, want)
		}
	}
}

func TestOrgShapeFeaturesLoadsCumulusCIOrgDefinitions(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "cumulusci.yml"), `
orgs:
  scratch:
    dev:
      config_file: orgs/dev.json
`)
	writeFile(t, filepath.Join(root, "orgs/dev.json"), `{
  "features": ["Communities", "PersonAccounts"],
  "settings": {
    "chatterSettings": {"enableChatter": true}
  }
}`)

	got := OrgShapeFeatures(root)
	want := []string{"Communities", "PersonAccounts", "Chatter"}
	if len(got) != len(want) {
		t.Fatalf("features = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("features = %#v, want %#v", got, want)
		}
	}
}

func TestLoadLegacySrcLayout(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "src/classes/Legacy.cls"), "public class Legacy {}")
	writeFile(t, filepath.Join(root, "src/triggers/Legacy.trigger"), "trigger Legacy on Account (before insert) {}")
	writeFile(t, filepath.Join(root, "src/objects/Thing__c/Thing__c.object-meta.xml"), "<CustomObject/>")
	writeFile(t, filepath.Join(root, "src/pages/Edit.page"), `<apex:page/>`)
	writeFile(t, filepath.Join(root, "src/components/Picker.component"), `<apex:component/>`)
	writeFile(t, filepath.Join(root, "src/aura/Widget/Widget.cmp"), `<aura:component/>`)
	writeFile(t, filepath.Join(root, "src/lwc/widget/widget.js"), `export default class Widget {}`)

	p, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.ApexFiles) != 2 || len(p.ObjectFiles) != 1 || len(p.VisualforcePageFiles) != 1 || len(p.VisualforceComponentFiles) != 1 || len(p.AuraFiles) != 1 || len(p.LWCFiles) != 1 {
		t.Fatalf("unexpected legacy layout counts: %#v", p)
	}
}

func TestLoadRespectsDeclaredPackageDirectoriesOverConventionalUnpackagedRoot(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app/main/default","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/Main.cls"), "public class Main {}")
	writeFile(t, filepath.Join(root, "unpackaged/classes/Extra.cls"), "public class Extra {}")

	p, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.ApexFiles) != 1 || filepath.Base(p.ApexFiles[0]) != "Main.cls" {
		t.Fatalf("apex files = %#v, want only declared package source", p.ApexFiles)
	}
}

func TestLoadDiscoversCustomMetadataRecordsOutsideCustomMetadataFolder(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	path := filepath.Join(root, "force-app/main/default/bindings/di_Binding.PaymentsApiPaymentFactory.md-meta.xml")
	writeFile(t, path, `<CustomMetadata><label>Payments API Payment Factory</label></CustomMetadata>`)

	p, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.CustomMetadataFiles) != 1 || p.CustomMetadataFiles[0] != path {
		t.Fatalf("custom metadata files = %#v", p.CustomMetadataFiles)
	}
}

func TestLoadDiscoversLegacyCustomMetadataRecordsUnderTypeFolder(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"src","default":true}]}`)
	legacyPath := filepath.Join(root, "src/objects/Feature__mdt/records/Default.md")
	modernPath := filepath.Join(root, "src/objects/Feature__mdt/records/Modern.md-meta.xml")
	writeFile(t, legacyPath, `<CustomMetadata><label>Default</label></CustomMetadata>`)
	writeFile(t, modernPath, `<CustomMetadata><label>Modern</label></CustomMetadata>`)

	p, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.CustomMetadataFiles) != 2 || p.CustomMetadataFiles[0] != legacyPath || p.CustomMetadataFiles[1] != modernPath {
		t.Fatalf("custom metadata files = %#v", p.CustomMetadataFiles)
	}
}

func TestLoadPrefersVisualforcePageMarkupOverMetadataCompanion(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"src","default":true}]}`)
	pagePath := filepath.Join(root, "src/pages/Setup.page")
	writeFile(t, pagePath, `<apex:page/>`)
	writeFile(t, filepath.Join(root, "src/pages/Setup.page-meta.xml"), `<ApexPage><label>Setup</label></ApexPage>`)
	metadataOnlyPath := filepath.Join(root, "src/pages/MetadataOnly.page-meta.xml")
	writeFile(t, metadataOnlyPath, `<ApexPage><label>Metadata Only</label></ApexPage>`)

	p, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.VisualforcePageFiles) != 2 || p.VisualforcePageFiles[0] != metadataOnlyPath || p.VisualforcePageFiles[1] != pagePath {
		t.Fatalf("visualforce page files = %#v", p.VisualforcePageFiles)
	}
}

func TestLoadDoesNotDuplicateCoveredNestedRoots(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"sfdx-source/app","default":true}]}`)
	writeFile(t, filepath.Join(root, "sfdx-source/app/classes/App.cls"), "public class App {}")

	p, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.ApexFiles) != 1 {
		t.Fatalf("apex files = %d, want 1: %#v", len(p.ApexFiles), p.ApexFiles)
	}
}

func TestLoadDeduplicatesOverlappingPackageDirectorySourceFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"packages"},{"path":"packages/app","default":true}]}`)
	sharedPath := filepath.Join(root, "packages/app/classes/Shared.cls")
	appDuplicatePath := filepath.Join(root, "packages/app/classes/Duplicate.cls")
	libDuplicatePath := filepath.Join(root, "packages/lib/classes/Duplicate.cls")
	writeFile(t, sharedPath, "public class Shared {}")
	writeFile(t, appDuplicatePath, "public class Duplicate {}")
	writeFile(t, libDuplicatePath, "public class Duplicate {}")

	p, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.ApexFiles) != 3 {
		t.Fatalf("apex files = %#v, want each physical file once", p.ApexFiles)
	}
	sharedCount := 0
	for _, file := range p.ApexFiles {
		if file == sharedPath {
			sharedCount++
		}
	}
	if sharedCount != 1 {
		t.Fatalf("shared source file count = %d, want 1 in %#v", sharedCount, p.ApexFiles)
	}
	if !slices.Contains(p.ApexFiles, appDuplicatePath) || !slices.Contains(p.ApexFiles, libDuplicatePath) {
		t.Fatalf("apex files = %#v, want both true duplicate class files", p.ApexFiles)
	}
}

func TestLoadSkipsSFDXDependencyCandidateThatIsSamePhysicalProject(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "rflib")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{
  "packageDirectories": [
    {"path":"rflib","default":true,"package":"RFLIB"},
    {"path":"rflib-fs","package":"RFLIB-FS","dependencies":[{"package":"RFLIB"}]}
  ]
}`)
	writeFile(t, filepath.Join(root, "rflib/main/default/classes/Core.cls"), "public class Core {}")
	writeFile(t, filepath.Join(root, "rflib-fs/main/default/classes/Extension.cls"), "public class Extension {}")
	alias := filepath.Join(parent, "RFLIB")
	if _, err := os.Stat(alias); err != nil {
		if err := os.Symlink(root, alias); err != nil {
			t.Skipf("could not create same-project alias path: %v", err)
		}
	}

	p, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.ManagedPackageDependencies) != 0 {
		t.Fatalf("dependencies = %#v, want no self dependency through %s", p.ManagedPackageDependencies, alias)
	}
	if len(p.ApexFiles) != 2 {
		t.Fatalf("apex files = %#v, want each project file once", p.ApexFiles)
	}
}

func TestPackagePathForFileChoosesConfiguredPackageRoot(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"packages"},{"path":"packages/app","default":true},{"path":"packages/lib"}]}`)
	writeFile(t, filepath.Join(root, "packages/app/classes/App.cls"), "public class App {}")
	writeFile(t, filepath.Join(root, "packages/lib/classes/Lib.cls"), "public class Lib {}")

	p, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	appPath := filepath.Join(root, "packages/app/classes/App.cls")
	libPath := filepath.Join(root, "packages/lib/classes/Lib.cls")
	if got := p.PackagePathForFile(appPath); got != "packages/app" {
		t.Fatalf("app package path = %q, want packages/app", got)
	}
	if got := p.PackagePathForFile(libPath); got != "packages/lib" {
		t.Fatalf("lib package path = %q, want packages/lib", got)
	}
	if got := p.PackagePathForFile(filepath.Join(root, "outside/Other.cls")); got != "" {
		t.Fatalf("outside package path = %q, want empty", got)
	}
}

func TestLoadSkipsStaticResourceVendorTrees(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/Good.cls"), "public class Good {}")
	writeFile(t, filepath.Join(root, "force-app/main/default/staticresources/Vendor.resource-meta.xml"), "<StaticResource/>")
	writeFile(t, filepath.Join(root, "force-app/main/default/staticresources/Vendor/scripts/Bad.cls"), "public class Bad {}")

	p, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.ApexFiles) != 1 || len(p.StaticResourceMetas) != 1 {
		t.Fatalf("unexpected static resource filtering: %#v", p)
	}
}

func TestDiscoverProjectIncludesDirectoryStaticResourceContent(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/fixture-app/main/staticresources/Bundle.resource-meta.xml"), "<StaticResource/>")
	writeFile(t, filepath.Join(root, "force-app/fixture-app/main/staticresources/Bundle/css/main.css"), "body{}")
	writeFile(t, filepath.Join(root, "force-app/fixture-app/main/staticresources/Bundle/scripts/NotAClass.cls"), "public class NotAClass {}")

	p, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(p.StaticResourceFiles, func(file string) bool {
		return strings.HasSuffix(filepath.ToSlash(file), "staticresources/Bundle/css/main.css")
	}) {
		t.Fatalf("StaticResourceFiles = %#v, want nested directory content", p.StaticResourceFiles)
	}
	if len(p.ApexFiles) != 0 {
		t.Fatalf("ApexFiles = %#v, want static resource content ignored as Apex", p.ApexFiles)
	}
}

func TestLoadSkipsDotDirectoriesAndFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/Good.cls"), "public class Good {}")
	writeFile(t, filepath.Join(root, "force-app/.cache/classes/Bad.cls"), "public class Bad {}")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/.Hidden.cls"), "public class Hidden {}")

	p, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.ApexFiles) != 1 {
		t.Fatalf("apex files = %#v, want only Good.cls", p.ApexFiles)
	}
	if filepath.Base(p.ApexFiles[0]) != "Good.cls" {
		t.Fatalf("apex files = %#v, want only Good.cls", p.ApexFiles)
	}
}

func TestLoadManagedPackageDependencies(t *testing.T) {
	root := t.TempDir()
	depRoot := filepath.Join(root, "deps", "pkg")
	consumerRoot := filepath.Join(root, "consumer")
	writeFile(t, filepath.Join(depRoot, "sfdx-project.json"), `{"namespace":"pkg","packageDirectories":[{"path":"force-app","default":true},{"path":"unpackaged"}]}`)
	writeFile(t, filepath.Join(depRoot, "force-app/main/default/classes/Visible.cls"), "global class Visible {}")
	writeFile(t, filepath.Join(depRoot, "unpackaged/main/default/classes/HiddenSetup.cls"), "public class HiddenSetup {}")
	writeFile(t, filepath.Join(consumerRoot, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(consumerRoot, "glade.yml"), `project:
  managedPackageDependencies: ["pkg:../deps/pkg:1.0"]
`)
	writeFile(t, filepath.Join(consumerRoot, "force-app/main/default/classes/Consumer.cls"), "public class Consumer {}")

	p, err := Load(consumerRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.ManagedPackageDependencies) != 1 {
		t.Fatalf("dependency count = %d", len(p.ManagedPackageDependencies))
	}
	dep := p.ManagedPackageDependencies[0]
	if dep.Status != "loaded" || dep.Namespace != "pkg" || dep.Version != "1.0" {
		t.Fatalf("dependency = %#v", dep)
	}
	if dep.Project.Namespace != "pkg" || len(dep.Project.ApexFiles) != 1 {
		t.Fatalf("loaded dependency project = %#v", dep.Project)
	}
	if filepath.Base(dep.Project.ApexFiles[0]) != "Visible.cls" {
		t.Fatalf("loaded dependency apex files = %#v", dep.Project.ApexFiles)
	}
}

func TestLoadResolvesLocalSFDXPackageDependencies(t *testing.T) {
	root := t.TempDir()
	coreRoot := filepath.Join(root, "packages", "core")
	consumerRoot := filepath.Join(root, "packages", "consumer")
	writeFile(t, filepath.Join(coreRoot, "sfdx-project.json"), `{
  "namespace": "pkgx",
  "packageDirectories": [{"path":"force-app","default":true,"package":"Core"}]
}`)
	writeFile(t, filepath.Join(coreRoot, "force-app/main/default/classes/CoreHelper.cls"), "global class CoreHelper {}")
	writeFile(t, filepath.Join(consumerRoot, "sfdx-project.json"), `{
  "namespace": "pkgx",
  "packageDirectories": [{
    "path":"force-app",
    "default":true,
    "package":"Consumer",
    "dependencies": [{"package":"Core"}]
  }]
}`)
	writeFile(t, filepath.Join(consumerRoot, "force-app/main/default/classes/Consumer.cls"), "public class Consumer {}")

	p, err := Load(consumerRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.ManagedPackageDependencies) != 1 {
		t.Fatalf("dependency count = %d", len(p.ManagedPackageDependencies))
	}
	dep := p.ManagedPackageDependencies[0]
	if dep.Status != "loaded" || dep.Namespace != "pkgx" || dep.Project == nil {
		t.Fatalf("dependency = %#v", dep)
	}
	if dep.Project.Root != coreRoot || len(dep.Project.ApexFiles) != 1 || filepath.Base(dep.Project.ApexFiles[0]) != "CoreHelper.cls" {
		t.Fatalf("loaded dependency project = %#v", dep.Project)
	}
}

func TestLoadDoesNotScanUnrelatedGrandchildrenForSFDXPackageDependencies(t *testing.T) {
	root := t.TempDir()
	consumerRoot := filepath.Join(root, "workspace", "consumer")
	unrelatedCoreRoot := filepath.Join(root, "workspace", "unrelated", "core")
	writeFile(t, filepath.Join(unrelatedCoreRoot, "sfdx-project.json"), `{
  "namespace": "pkgx",
  "packageDirectories": [{"path":"force-app","default":true,"package":"Core"}]
}`)
	writeFile(t, filepath.Join(unrelatedCoreRoot, "force-app/main/default/classes/CoreHelper.cls"), "global class CoreHelper {}")
	writeFile(t, filepath.Join(consumerRoot, "sfdx-project.json"), `{
  "namespace": "pkgx",
  "packageDirectories": [{
    "path":"force-app",
    "default":true,
    "package":"Consumer",
    "dependencies": [{"package":"Core"}]
  }]
}`)
	writeFile(t, filepath.Join(consumerRoot, "force-app/main/default/classes/Consumer.cls"), "public class Consumer {}")

	p, err := Load(consumerRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.ManagedPackageDependencies) != 0 {
		t.Fatalf("dependency count = %d, dependency = %#v", len(p.ManagedPackageDependencies), p.ManagedPackageDependencies)
	}
}

func TestLoadResolvesSiblingSFDXPackageDependencyWithAncestorGladeConfig(t *testing.T) {
	root := t.TempDir()
	workspaceRoot := filepath.Join(root, "workspace")
	coreRoot := filepath.Join(workspaceRoot, "modules", "src-core-package")
	consumerRoot := filepath.Join(workspaceRoot, "modules", "src-consumer-package")
	writeFile(t, filepath.Join(workspaceRoot, "glade.yml"), `project:
  managedPackageDependencies: ["vend:../vendor"]
`)
	writeFile(t, filepath.Join(root, "vendor", "sfdx-project.json"), `{"namespace":"vend","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "vendor", "force-app/main/default/classes/Vendor.cls"), "global class Vendor {}")
	writeFile(t, filepath.Join(coreRoot, "sfdx-project.json"), `{
  "namespace": "pkgx",
  "packageDirectories": [{"path":"force-app","default":true,"package":"Core"}]
}`)
	writeFile(t, filepath.Join(coreRoot, "force-app/main/default/classes/CoreHelper.cls"), "global class CoreHelper {}")
	writeFile(t, filepath.Join(consumerRoot, "sfdx-project.json"), `{
  "namespace": "pkgx",
  "packageDirectories": [{
    "path":"force-app",
    "default":true,
    "package":"Consumer",
    "dependencies": [{"package":"Core"}]
  }]
}`)
	writeFile(t, filepath.Join(consumerRoot, "force-app/main/default/classes/Consumer.cls"), "public class Consumer {}")

	p, err := Load(consumerRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.ManagedPackageDependencies) != 1 {
		t.Fatalf("dependency count = %d", len(p.ManagedPackageDependencies))
	}
	dep := p.ManagedPackageDependencies[0]
	if dep.SourceRoot != coreRoot || dep.Project == nil || dep.Project.Root != coreRoot {
		t.Fatalf("dependency = %#v", dep)
	}
}

func TestLoadResolvesSFDXPackageDependencyFromAncestorGladeConfig(t *testing.T) {
	root := t.TempDir()
	depRoot := filepath.Join(root, "upstream-package-develop")
	workspaceRoot := filepath.Join(root, "consumer-workspace")
	consumerRoot := filepath.Join(workspaceRoot, "sfdx-source", "apps", "consumer-core")
	writeFile(t, filepath.Join(workspaceRoot, "glade.yml"), `project:
  namespaceRemaps: ["UP:vend"]
  managedPackageDependencies: ["vend:../upstream-package-develop"]
`)
	writeFile(t, filepath.Join(depRoot, "sfdx-project.json"), `{"namespace":"UP","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(depRoot, "force-app/main/default/classes/UpstreamGateway.cls"), "global class UpstreamGateway {}")
	writeFile(t, filepath.Join(consumerRoot, "sfdx-project.json"), `{
  "namespace": "corepkg",
  "packageDirectories": [{
    "path":"force-app",
    "default":true,
    "package":"Core",
    "dependencies": [{"package":"vend"}]
  }]
}`)
	writeFile(t, filepath.Join(consumerRoot, "force-app/main/default/classes/Consumer.cls"), "public class Consumer {}")

	p, err := Load(consumerRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.ManagedPackageDependencies) != 1 {
		t.Fatalf("dependency count = %d", len(p.ManagedPackageDependencies))
	}
	dep := p.ManagedPackageDependencies[0]
	if dep.Status != "loaded" || dep.Namespace != "vend" || dep.Project == nil {
		t.Fatalf("dependency = %#v", dep)
	}
	if dep.Project.Namespace != "vend" || len(dep.Project.NamespaceRemaps) != 1 {
		t.Fatalf("dependency project = %#v", dep.Project)
	}
}

func TestLoadManagedPackageDependencyInheritsMatchingNamespaceRemap(t *testing.T) {
	root := t.TempDir()
	depRoot := filepath.Join(root, "deps", "base-source")
	consumerRoot := filepath.Join(root, "consumer")
	writeFile(t, filepath.Join(depRoot, "sfdx-project.json"), `{"namespace":"BasePkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(depRoot, "force-app/main/default/classes/Helper.cls"), "global class Helper {}")
	writeFile(t, filepath.Join(consumerRoot, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(consumerRoot, "glade.yml"), `project:
  namespaceRemaps: ["BasePkg:stagepkg"]
  managedPackageDependencies: ["stagepkg:../deps/base-source:1.0"]
`)
	writeFile(t, filepath.Join(consumerRoot, "force-app/main/default/classes/Consumer.cls"), "public class Consumer {}")

	p, err := Load(consumerRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.NamespaceRemaps) != 1 || p.NamespaceRemaps[0].From != "BasePkg" || p.NamespaceRemaps[0].To != "stagepkg" {
		t.Fatalf("project namespace remaps = %#v", p.NamespaceRemaps)
	}
	if len(p.ManagedPackageDependencies) != 1 {
		t.Fatalf("dependency count = %d", len(p.ManagedPackageDependencies))
	}
	dep := p.ManagedPackageDependencies[0]
	if dep.Status != "loaded" || dep.Namespace != "stagepkg" || dep.Project == nil {
		t.Fatalf("dependency = %#v", dep)
	}
	if dep.Project.Namespace != "stagepkg" {
		t.Fatalf("dependency runtime namespace = %q, want stagepkg", dep.Project.Namespace)
	}
	if len(dep.Project.NamespaceRemaps) != 1 || dep.Project.NamespaceRemaps[0].From != "BasePkg" || dep.Project.NamespaceRemaps[0].To != "stagepkg" {
		t.Fatalf("dependency namespace remaps = %#v", dep.Project.NamespaceRemaps)
	}
	if len(p.DependencyDiagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", p.DependencyDiagnostics)
	}
}

func TestLoadManagedPackageDependencyReportsMissingNamespaceRemap(t *testing.T) {
	root := t.TempDir()
	depRoot := filepath.Join(root, "deps", "base-source")
	consumerRoot := filepath.Join(root, "consumer")
	writeFile(t, filepath.Join(depRoot, "sfdx-project.json"), `{"namespace":"BasePkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(depRoot, "force-app/main/default/classes/Helper.cls"), "global class Helper {}")
	writeFile(t, filepath.Join(consumerRoot, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(consumerRoot, "glade.yml"), `project:
  managedPackageDependencies: ["stagepkg:../deps/base-source:1.0"]
`)

	p, err := Load(consumerRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.ManagedPackageDependencies) != 1 {
		t.Fatalf("dependency count = %d", len(p.ManagedPackageDependencies))
	}
	dep := p.ManagedPackageDependencies[0]
	if dep.Status != "namespace_mismatch" || dep.Project != nil {
		t.Fatalf("dependency = %#v", dep)
	}
	if len(p.DependencyDiagnostics) != 1 || p.DependencyDiagnostics[0].Code != "dependency_namespace_remap_missing" {
		t.Fatalf("diagnostics = %#v", p.DependencyDiagnostics)
	}
	if !strings.Contains(p.DependencyDiagnostics[0].Message, `namespaceRemaps: ["BasePkg:stagepkg"]`) {
		t.Fatalf("diagnostic message = %q", p.DependencyDiagnostics[0].Message)
	}
}

func TestMatchingNamespaceRemapsRequiresSourceAndRuntimeMatch(t *testing.T) {
	rules := []namespaceremap.Rule{{From: "BasePkg", To: "stagepkg"}}
	if got := matchingNamespaceRemaps(rules, "BasePkg", "other"); len(got) != 0 {
		t.Fatalf("source-only match should not inherit remap: %#v", got)
	}
	if got := matchingNamespaceRemaps(rules, "Other", "stagepkg"); len(got) != 0 {
		t.Fatalf("runtime-only match should not inherit remap: %#v", got)
	}
	if got := matchingNamespaceRemaps(rules, "BasePkg", "stagepkg"); len(got) != 1 {
		t.Fatalf("source and runtime match should inherit remap: %#v", got)
	}
}

func TestLoadReportsMissingManagedPackageDependency(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "glade.yml"), `project:
  managedPackageDependencies: ["pkg:../missing"]
`)

	p, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.ManagedPackageDependencies) != 1 || p.ManagedPackageDependencies[0].Status != "missing" {
		t.Fatalf("dependencies = %#v", p.ManagedPackageDependencies)
	}
	if len(p.DependencyDiagnostics) != 1 || p.DependencyDiagnostics[0].Code != "dependency_missing" {
		t.Fatalf("diagnostics = %#v", p.DependencyDiagnostics)
	}
}

func TestLoadManagedPackageArtifactDependency(t *testing.T) {
	root := t.TempDir()
	artifactPath := filepath.Join(root, "packages", "pkg.glade-package.json")
	writeFile(t, artifactPath, `{"namespace":"pkg","version":"1.0","sourceHash":"abc","apexTypes":[{"kind":"class","name":"Address","namespace":"pkg","dependency":true}]}`)
	writeFile(t, filepath.Join(root, "consumer", "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "consumer", "glade.yml"), `project:
  managedPackageDependencies: ["pkg:artifact:../packages/pkg.glade-package.json"]
`)

	p, err := Load(filepath.Join(root, "consumer"))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.ManagedPackageDependencies) != 1 {
		t.Fatalf("dependency count = %d", len(p.ManagedPackageDependencies))
	}
	dep := p.ManagedPackageDependencies[0]
	if dep.Status != "loaded" || dep.ArtifactPath != artifactPath || dep.SourceRoot != "" || dep.Version != "1.0" {
		t.Fatalf("dependency = %#v", dep)
	}
}

func TestLoadReportsManagedPackageArtifactVersionMismatch(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "packages", "pkg.glade-package.json"), `{"namespace":"pkg","version":"1.0","sourceHash":"abc"}`)
	writeFile(t, filepath.Join(root, "consumer", "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "consumer", "glade.yml"), `project:
  managedPackageDependencies: ["pkg:artifact:../packages/pkg.glade-package.json:2.0"]
`)

	p, err := Load(filepath.Join(root, "consumer"))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.ManagedPackageDependencies) != 1 || p.ManagedPackageDependencies[0].Status != "version_mismatch" {
		t.Fatalf("dependencies = %#v", p.ManagedPackageDependencies)
	}
	if len(p.DependencyDiagnostics) != 1 || p.DependencyDiagnostics[0].Code != "dependency_version_mismatch" {
		t.Fatalf("diagnostics = %#v", p.DependencyDiagnostics)
	}
}

func TestLoadReportsManagedPackageArtifactSchemaVersionAsLoadError(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "packages", "pkg.glade-package.json"), `{
  "schemaVersion": 99,
  "namespace": "pkg",
  "version": "1.2.3",
  "sourceHash": "abc",
  "builtAt": "2026-06-19T12:00:00Z"
}`)
	writeFile(t, filepath.Join(root, "consumer", "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "consumer", "glade.yml"), `project:
  managedPackageDependencies: ["pkg:artifact:../packages/pkg.glade-package.json:1.2.3"]
`)

	p, err := Load(filepath.Join(root, "consumer"))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.ManagedPackageDependencies) != 1 || p.ManagedPackageDependencies[0].Status != "load_error" {
		t.Fatalf("dependencies = %#v", p.ManagedPackageDependencies)
	}
	if len(p.DependencyDiagnostics) != 1 || p.DependencyDiagnostics[0].Code != "dependency_load_error" {
		t.Fatalf("diagnostics = %#v", p.DependencyDiagnostics)
	}
	if !strings.Contains(p.DependencyDiagnostics[0].Message, "unsupported artifact schemaVersion 99") {
		t.Fatalf("diagnostic message = %q", p.DependencyDiagnostics[0].Message)
	}
}

func TestLoadReportsManagedPackageArtifactMissingVersionAsLoadError(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "packages", "pkg.glade-package.json"), `{"namespace":"pkg","sourceHash":"abc"}`)
	writeFile(t, filepath.Join(root, "consumer", "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "consumer", "glade.yml"), `project:
  managedPackageDependencies: ["pkg:artifact:../packages/pkg.glade-package.json:2.0"]
`)

	p, err := Load(filepath.Join(root, "consumer"))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.ManagedPackageDependencies) != 1 || p.ManagedPackageDependencies[0].Status != "load_error" {
		t.Fatalf("dependencies = %#v", p.ManagedPackageDependencies)
	}
	if len(p.DependencyDiagnostics) != 1 || p.DependencyDiagnostics[0].Code != "dependency_load_error" {
		t.Fatalf("diagnostics = %#v", p.DependencyDiagnostics)
	}
}

func TestLoadReportsEmptyManagedPackageArtifactAsLoadError(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "packages", "pkg.glade-package.json"), `{}`)
	writeFile(t, filepath.Join(root, "consumer", "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "consumer", "glade.yml"), `project:
  managedPackageDependencies: ["pkg:artifact:../packages/pkg.glade-package.json"]
`)

	p, err := Load(filepath.Join(root, "consumer"))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.ManagedPackageDependencies) != 1 || p.ManagedPackageDependencies[0].Status != "load_error" {
		t.Fatalf("dependencies = %#v", p.ManagedPackageDependencies)
	}
	if len(p.DependencyDiagnostics) != 1 || p.DependencyDiagnostics[0].Code != "dependency_load_error" {
		t.Fatalf("diagnostics = %#v", p.DependencyDiagnostics)
	}
}

func TestLoadReportsMalformedManagedPackageArtifactAsLoadError(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "packages", "pkg.glade-package.json"), `{`)
	writeFile(t, filepath.Join(root, "consumer", "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "consumer", "glade.yml"), `project:
  managedPackageDependencies: ["pkg:artifact:../packages/pkg.glade-package.json"]
`)

	p, err := Load(filepath.Join(root, "consumer"))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.ManagedPackageDependencies) != 1 || p.ManagedPackageDependencies[0].Status != "load_error" {
		t.Fatalf("dependencies = %#v", p.ManagedPackageDependencies)
	}
	if len(p.DependencyDiagnostics) != 1 || p.DependencyDiagnostics[0].Code != "dependency_load_error" {
		t.Fatalf("diagnostics = %#v", p.DependencyDiagnostics)
	}
}

func TestDiscoverLWCBundleFiles(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "force-app/main/default/lwc/counter")
	writeFile(t, filepath.Join(base, "counter.js"), `export default class Counter {}`)
	writeFile(t, filepath.Join(base, "counter.html"), `<template><p>{count}</p></template>`)
	writeFile(t, filepath.Join(base, "counter.css"), `.title { color: red; }`)
	writeFile(t, filepath.Join(base, "counter.js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><isExposed>true</isExposed></LightningComponentBundle>`)

	p, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.LWCFiles) != 1 || !strings.HasSuffix(p.LWCFiles[0], "counter.js") {
		t.Fatalf("LWCFiles = %#v", p.LWCFiles)
	}
	if len(p.LWCHTMLFiles) != 1 {
		t.Fatalf("LWCHTMLFiles = %#v", p.LWCHTMLFiles)
	}
	if len(p.LWCCSSFiles) != 1 {
		t.Fatalf("LWCCSSFiles = %#v", p.LWCCSSFiles)
	}
	if len(p.LWCMetaFiles) != 1 {
		t.Fatalf("LWCMetaFiles = %#v", p.LWCMetaFiles)
	}
}

func TestDiscoverLWCUtilityFilesInNestedPackageDirectory(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{
  "packageDirectories": [
    {"path":"sfdx-source/nu","default":true},
    {"path":"sfdx-source/unpackaged"}
  ],
  "namespace": "PKG"
}`)
	utilityDir := filepath.Join(root, "sfdx-source/nu/main/component-library/lwc/bUtils")
	writeFile(t, filepath.Join(utilityDir, "bUtils.js"), `export { classSet } from './classSet';`)
	writeFile(t, filepath.Join(utilityDir, "classSet.js"), `export function classSet(value) { return value; }`)
	writeFile(t, filepath.Join(utilityDir, "bUtils.js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><isExposed>false</isExposed></LightningComponentBundle>`)
	componentDir := filepath.Join(root, "sfdx-source/unpackaged/lwc/recipePaymentForm")
	writeFile(t, filepath.Join(componentDir, "recipePaymentForm.js"), `import { LightningElement } from 'lwc'; export default class RecipePaymentForm extends LightningElement {}`)
	writeFile(t, filepath.Join(componentDir, "recipePaymentForm.html"), `<template></template>`)

	p, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if !projectTestHasSuffix(p.LWCFiles, "bUtils/bUtils.js") {
		t.Fatalf("LWCFiles missing bUtils utility entry: %#v", p.LWCFiles)
	}
	if !projectTestHasSuffix(p.LWCFiles, "bUtils/classSet.js") {
		t.Fatalf("LWCFiles missing bUtils sibling entry: %#v", p.LWCFiles)
	}
	if !projectTestHasSuffix(p.LWCMetaFiles, "bUtils/bUtils.js-meta.xml") {
		t.Fatalf("LWCMetaFiles missing bUtils metadata: %#v", p.LWCMetaFiles)
	}
	if !projectTestHasSuffix(p.LWCHTMLFiles, "recipePaymentForm/recipePaymentForm.html") {
		t.Fatalf("LWCHTMLFiles missing unpackaged component: %#v", p.LWCHTMLFiles)
	}
}

func projectTestHasSuffix(paths []string, suffix string) bool {
	suffix = filepath.ToSlash(suffix)
	for _, path := range paths {
		if strings.HasSuffix(filepath.ToSlash(path), suffix) {
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
