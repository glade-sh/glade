package project

import (
	"os"
	"path/filepath"
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
	if len(p.LabelFiles) != 1 || len(p.TranslationFiles) != 1 || len(p.StaticResourceFiles) != 2 || len(p.StaticResourceMetas) != 2 || len(p.ContentAssetFiles) != 1 || len(p.ContentAssetMetas) != 1 || len(p.EmailTemplateFiles) != 2 || len(p.NamedCredentialFiles) != 1 || len(p.RemoteSiteFiles) != 1 || len(p.CustomMetadataFiles) != 3 {
		t.Fatalf("unexpected legacy metadata file counts: %#v", p)
	}
	if len(p.WorkflowFiles) != 2 {
		t.Fatalf("unexpected workflow file counts: %#v", p)
	}
	if len(p.FlowFiles) != 1 || len(p.ProfileFiles) != 1 || len(p.PermissionSetFiles) != 1 || len(p.PermissionAssignmentFiles) != 1 {
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

func TestLoadManagedPackageDependencies(t *testing.T) {
	root := t.TempDir()
	depRoot := filepath.Join(root, "deps", "znu")
	consumerRoot := filepath.Join(root, "consumer")
	writeFile(t, filepath.Join(depRoot, "sfdx-project.json"), `{"namespace":"znu","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(depRoot, "force-app/main/default/classes/Visible.cls"), "global class Visible {}")
	writeFile(t, filepath.Join(consumerRoot, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(consumerRoot, "oaer.yml"), `project:
  managedPackageDependencies: ["znu:../deps/znu:1.0"]
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
	if dep.Status != "loaded" || dep.Namespace != "znu" || dep.Version != "1.0" {
		t.Fatalf("dependency = %#v", dep)
	}
	if dep.Project.Namespace != "znu" || len(dep.Project.ApexFiles) != 1 {
		t.Fatalf("loaded dependency project = %#v", dep.Project)
	}
}

func TestLoadReportsMissingManagedPackageDependency(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "oaer.yml"), `project:
  managedPackageDependencies: ["znu:../missing"]
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

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
