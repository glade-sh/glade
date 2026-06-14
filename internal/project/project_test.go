package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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

func TestLoadSupplementsConventionalUnpackagedRoot(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app/main/default","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/Main.cls"), "public class Main {}")
	writeFile(t, filepath.Join(root, "unpackaged/classes/Extra.cls"), "public class Extra {}")

	p, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.ApexFiles) != 2 {
		t.Fatalf("apex files = %d, want 2: %#v", len(p.ApexFiles), p.ApexFiles)
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
	writeFile(t, artifactPath, `{"namespace":"pkg","version":"1.0","apexTypes":[{"kind":"class","name":"Address","namespace":"pkg","dependency":true}]}`)
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
	writeFile(t, filepath.Join(root, "packages", "pkg.glade-package.json"), `{"namespace":"pkg","version":"1.0"}`)
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

func TestLoadReportsManagedPackageArtifactMissingVersionAsLoadError(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "packages", "pkg.glade-package.json"), `{"namespace":"pkg"}`)
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
  "namespace": "NU"
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
