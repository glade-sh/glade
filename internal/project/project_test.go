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
  "namespace": "NU",
  "sourceApiVersion": "61.0"
}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/Hello.cls"), "public class Hello {}")
	writeFile(t, filepath.Join(root, "force-app/main/triggers/Hello.trigger"), "trigger Hello on Account (before insert) {}")
	writeFile(t, filepath.Join(root, "force-app/main/objects/Thing__c/Thing__c.object-meta.xml"), "<CustomObject/>")
	writeFile(t, filepath.Join(root, "force-app/main/objects/Thing__c/fields/Name__c.field-meta.xml"), "<CustomField/>")
	writeFile(t, filepath.Join(root, "force-app/main/objects/Thing__c/fieldSets/Summary.fieldSet-meta.xml"), "<FieldSet/>")
	writeFile(t, filepath.Join(root, "force-app/main/objects/Thing__c/recordTypes/Business.recordType-meta.xml"), "<RecordType/>")
	writeFile(t, filepath.Join(root, "force-app/main/objects/Thing__c/validationRules/Block.validationRule-meta.xml"), "<ValidationRule/>")
	writeFile(t, filepath.Join(root, "force-app/main/labels/CustomLabels.labels"), "<CustomLabels/>")
	writeFile(t, filepath.Join(root, "force-app/main/staticresources/Site.resource"), "body")
	writeFile(t, filepath.Join(root, "force-app/main/staticresources/Site.resource-meta.xml"), "<StaticResource/>")
	writeFile(t, filepath.Join(root, "force-app/main/namedCredentials/Api.namedCredential"), "<NamedCredential/>")
	writeFile(t, filepath.Join(root, "force-app/main/remoteSiteSettings/Api.remoteSite"), "<RemoteSiteSetting/>")
	writeFile(t, filepath.Join(root, "force-app/main/customMetadata/Feature.Default.md"), "<CustomMetadata/>")
	writeFile(t, filepath.Join(root, "force-app/main/workflows/Thing__c.workflow-meta.xml"), "<Workflow/>")
	writeFile(t, filepath.Join(root, "force-app/main/flows/Onboard.flow-meta.xml"), "<Flow/>")
	writeFile(t, filepath.Join(root, "force-app/main/profiles/Admin.profile-meta.xml"), "<Profile/>")
	writeFile(t, filepath.Join(root, "force-app/main/permissionsets/App.permissionset-meta.xml"), "<PermissionSet/>")
	writeFile(t, filepath.Join(root, "force-app/main/permissionSetAssignments/App.permissionsetassignment-meta.xml"), "<PermissionSetAssignment/>")
	writeFile(t, filepath.Join(root, "force-app/main/pages/Edit.page"), `<apex:page controller="EditController"/>`)
	writeFile(t, filepath.Join(root, "force-app/main/components/Picker.component"), `<apex:component/>`)
	writeFile(t, filepath.Join(root, "force-app/main/aura/Cart/Cart.cmp"), `<aura:component controller="CartController"/>`)
	writeFile(t, filepath.Join(root, "force-app/main/aura/Cart/CartController.js"), `({ save: function(cmp) { cmp.get("c.save"); } })`)
	writeFile(t, filepath.Join(root, "force-app/main/lwc/cart/cart.js"), `import save from '@salesforce/apex/CartController.save';`)
	writeFile(t, filepath.Join(root, "force-app/main/lwc/cart/cart.html"), `<template></template>`)
	writeFile(t, filepath.Join(root, "force-app/main/docs/README.md"), "# not metadata")

	p, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if p.Namespace != "NU" || p.SourceAPIVersion != "61.0" {
		t.Fatalf("project metadata = %#v", p)
	}
	if len(p.ApexFiles) != 2 || len(p.ObjectFiles) != 1 || len(p.FieldFiles) != 1 || len(p.FieldSetFiles) != 1 || len(p.RecordTypeFiles) != 1 || len(p.ValidationRuleFiles) != 1 {
		t.Fatalf("unexpected file counts: %#v", p)
	}
	if len(p.LabelFiles) != 1 || len(p.StaticResourceFiles) != 1 || len(p.StaticResourceMetas) != 1 || len(p.NamedCredentialFiles) != 1 || len(p.RemoteSiteFiles) != 1 || len(p.CustomMetadataFiles) != 1 {
		t.Fatalf("unexpected legacy metadata file counts: %#v", p)
	}
	if len(p.WorkflowFiles) != 1 {
		t.Fatalf("unexpected workflow file counts: %#v", p)
	}
	if len(p.FlowFiles) != 1 || len(p.ProfileFiles) != 1 || len(p.PermissionSetFiles) != 1 || len(p.PermissionAssignmentFiles) != 1 {
		t.Fatalf("unexpected metadata stub file counts: %#v", p)
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
	writeFile(t, filepath.Join(root, "src/aura/Cart/Cart.cmp"), `<aura:component/>`)
	writeFile(t, filepath.Join(root, "src/lwc/cart/cart.js"), `export default class Cart {}`)

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

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
