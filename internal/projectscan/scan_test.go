package projectscan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanFindsProjectGaps(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "src/pages/Edit.page"), `<apex:page controller="EditController" action="{!load}">
  <apex:stylesheet value="{!URLFOR($Resource.Resources, 'site.css')}" />
  <apex:composition template="{!$Site.Template}" />
  {!$Label.EditTitle}
</apex:page>`)
	writeFile(t, filepath.Join(root, "src/components/Picker.component"), `<apex:component><apex:attribute name="value" type="String"/></apex:component>`)
	writeFile(t, filepath.Join(root, "src/aura/Thing/Thing.cmp"), `<aura:component controller="ThingController"/>`)
	writeFile(t, filepath.Join(root, "src/lwc/currencyMenu/currencyMenu.js"), `import { LightningElement, wire } from 'lwc';
import getCurrencyInformation from '@salesforce/apex/CurrencyMenuController.getCurrencyInformation';
import Save from '@salesforce/label/c.Save';
import RESOURCES from '@salesforce/resourceUrl/Resources';
import ACCOUNT_NAME from '@salesforce/schema/Account.Name';
import MISSING_SCHEMA from '@salesforce/schema/Missing__c.Name';
import { NavigationMixin } from 'lightning/navigation';
import { getObjectInfo } from 'lightning/uiObjectInfoApi';
@wire(getCurrencyInformation, { currencyCode: '$guestCurrencyCode' }) value;`)
	writeFile(t, filepath.Join(root, "src/workflows/Account.workflow"), `<Workflow><rules><fullName>Rule</fullName></rules></Workflow>`)
	writeFile(t, filepath.Join(root, "src/flows/Update.flow"), `<Flow><processType>Workflow</processType></Flow>`)
	writeFile(t, filepath.Join(root, "src/labels/CustomLabels.labels"), `<CustomLabels/>`)
	writeFile(t, filepath.Join(root, "src/email/Local/Welcome.email"), `Hello {!Contact.Name}`)
	writeFile(t, filepath.Join(root, "src/objects/Thing__c.object"), `<CustomObject/>`)
	writeFile(t, filepath.Join(root, "src/customMetadata/Page2.Home.md"), `<CustomMetadata/>`)
	writeFile(t, filepath.Join(root, "README.md"), `# Not custom metadata`)
	writeFile(t, filepath.Join(root, "src/staticresources/Resources.resource"), `body`)
	writeFile(t, filepath.Join(root, "src/contentassets/Setup.asset"), `body`)
	writeFile(t, filepath.Join(root, "src/namedCredentials/Api.namedCredential"), `<NamedCredential/>`)
	writeFile(t, filepath.Join(root, "src/remoteSiteSettings/Api.remoteSite"), `<RemoteSiteSetting/>`)
	writeFile(t, filepath.Join(root, "src/layouts/Account.layout"), `<Layout/>`)
	writeFile(t, filepath.Join(root, "src/classes/UsesPlatform.cls"), `public class UsesPlatform {
  void run() {
    PageReference p = Page.Edit;
    System.debug(Label.Save);
    Metadata.DeployContainer c = new Metadata.DeployContainer();
    System.debug(Site.getAdminEmail());
    System.debug(ConnectApi.Organization.getSettings().orgId);
    Auth.SessionManagement.getCurrentSession();
    Callable cb;
    Test.createStub(UsesPlatform.class, null);
    req.setEndpoint('callout:Api/path');
    Attachment a = new Attachment();
    Community__mdt cfg;
  }
}`)
	writeFile(t, filepath.Join(root, ".claude/worktrees/noisy/src/classes/Generated.cls"), `public class Generated {
  void run() {
    System.debug(Label.Generated);
  }
}`)

	report, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}

	wantCaps := []string{
		"apex.callable-stub",
		"aura.controller-test",
		"email.templates",
		"files.binary-content",
		"flow.save-order",
		"labels.localization",
		"lwc.controller-test",
		"metadata.apex-deploy",
		"metadata.legacy-source",
		"platform.cache-connectapi",
		"platform.auth-context",
		"site.community-context",
		"ui.presentation-metadata",
		"visualforce.component-test",
		"visualforce.controller-test",
		"workflow.save-order",
	}
	for _, cap := range wantCaps {
		if findSurface(report, cap) == nil {
			t.Fatalf("missing capability %s in report: %#v", cap, report.Surfaces)
		}
	}
	if report.Summary.FilesScanned == 0 || report.Summary.Findings == 0 || report.Summary.TestBlockingFindings == 0 {
		t.Fatalf("summary was not populated: %#v", report.Summary)
	}
	if len(report.TopBlockers) == 0 {
		t.Fatalf("top blockers empty")
	}
	for _, finding := range report.Findings {
		if finding.File == "README.md" && finding.Capability == "custommetadata.legacy-records" {
			t.Fatalf("README.md was classified as custom metadata: %#v", finding)
		}
		if finding.File == "src/customMetadata/Page2.Home.md" && finding.Capability == "custommetadata.legacy-records" {
			t.Fatalf("loaded custom metadata record was classified as unsupported: %#v", finding)
		}
	}
	if !hasLineFinding(report, "lwc.controller-test", "src/lwc/currencyMenu/currencyMenu.js", "CurrencyMenuController.getCurrencyInformation") {
		t.Fatalf("missing LWC Apex import finding")
	}
	if !hasLineFindingContaining(report, "ui.presentation-metadata", "src/lwc/currencyMenu/currencyMenu.js", "Missing__c.Name") {
		t.Fatalf("missing unresolved LWC schema finding")
	}
	if hasLineFinding(report, "ui.presentation-metadata", "src/lwc/currencyMenu/currencyMenu.js", "navigation") ||
		hasLineFinding(report, "ui.presentation-metadata", "src/lwc/currencyMenu/currencyMenu.js", "uiObjectInfoApi") {
		t.Fatalf("recognized Lightning client modules should not be local Apex test blockers")
	}
	if hasLineFinding(report, "ui.presentation-metadata", "src/layouts/Account.layout", "Account") {
		t.Fatalf("discovered presentation metadata file should not be a load blocker")
	}
	for _, finding := range report.Findings {
		if strings.Contains(finding.File, ".claude/") {
			t.Fatalf("scanner included generated agent worktree file: %#v", finding)
		}
	}
}

func TestScanSuppressesResolvedStandardSchemaReferences(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/lwc/resolved/resolved.js"), `import ACCOUNT_NAME from '@salesforce/schema/Account.Name';
import LEAD_LAST_NAME from '@salesforce/schema/Lead.LastName';
import BATCH_OBJECT from '@salesforce/schema/Batch__c';
import MISSING_FIELD from '@salesforce/schema/Account.NotAField__c';
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Batch__c/Batch__c.object-meta.xml"), `<CustomObject><label>Batch</label><pluralLabel>Batches</pluralLabel></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Batch__c/fields/Amount__c.field-meta.xml"), `<CustomField><fullName>Amount__c</fullName><label>Amount</label><type>Currency</type></CustomField>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/Resolved.page"), `<apex:page>
{!$ObjectType.Opportunity.Fields.StageName.Label}
{!$ObjectType.Batch__c.Fields.Name.InlineHelpText}
{!$ObjectType.Batch__c.Fields.pkg__Amount__c.Label}
{!$ObjectType.Batch__c.Fields[fieldName].Label}
{!$Component.localPanel}
{!$ObjectType.Account.Fields.NotAField__c.Label}
</apex:page>`)

	report, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}

	if hasLineFindingContaining(report, "ui.presentation-metadata", "force-app/main/default/lwc/resolved/resolved.js", "Account.Name") {
		t.Fatalf("resolved Account.Name schema import should not be reported")
	}
	if hasLineFindingContaining(report, "ui.presentation-metadata", "force-app/main/default/lwc/resolved/resolved.js", "Lead.LastName") {
		t.Fatalf("resolved Lead.LastName schema import should not be reported")
	}
	if hasLineFindingContaining(report, "ui.presentation-metadata", "force-app/main/default/lwc/resolved/resolved.js", "Batch__c") {
		t.Fatalf("resolved Batch__c schema import should not be reported")
	}
	if hasLineFindingContaining(report, "ui.presentation-metadata", "force-app/main/default/pages/Resolved.page", "Opportunity.Fields.StageName") {
		t.Fatalf("resolved Opportunity.StageName object type reference should not be reported")
	}
	if hasLineFindingContaining(report, "ui.presentation-metadata", "force-app/main/default/pages/Resolved.page", "Batch__c.Fields.Name") {
		t.Fatalf("resolved custom object standard Name field reference should not be reported")
	}
	if hasLineFindingContaining(report, "ui.presentation-metadata", "force-app/main/default/pages/Resolved.page", "Batch__c.Fields.pkg__Amount__c") {
		t.Fatalf("resolved namespaced custom field reference should not be reported")
	}
	if hasLineFindingContaining(report, "ui.presentation-metadata", "force-app/main/default/pages/Resolved.page", "Batch__c.Fields") {
		t.Fatalf("resolved dynamic custom object field map reference should not be reported")
	}
	if hasLineFindingContaining(report, "ui.presentation-metadata", "force-app/main/default/pages/Resolved.page", "$Component.localPanel") {
		t.Fatalf("$Component client-side id reference should not be reported")
	}
	if !hasLineFindingContaining(report, "ui.presentation-metadata", "force-app/main/default/lwc/resolved/resolved.js", "Account.NotAField__c") {
		t.Fatalf("missing unresolved Account.NotAField__c finding")
	}
	if !hasLineFindingContaining(report, "ui.presentation-metadata", "force-app/main/default/pages/Resolved.page", "Account.Fields.NotAField__c") {
		t.Fatalf("missing unresolved ObjectType field finding")
	}
}

func TestScanDoesNotClassifyPassiveUIFilesAsControllerBlockers(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/ExistingController.cls"), `public class ExistingController {
  public void save() {}
}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/ExistingExtension.cls"), `public class ExistingExtension {
  public ExistingExtension(ApexPages.StandardController controller) {}
  public void cancel() {}
}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/lwc/passive/passive.js"), `import { LightningElement } from 'lwc';
export default class Passive extends LightningElement {}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/lwc/passive/passive.html"), `<template><span>Passive</span></template>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/Passive.page"), `<apex:page><apex:outputText value="Passive"/></apex:page>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/Existing.page"), `<apex:page controller="ExistingController" action="{!save}"><apex:form /></apex:page>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/ExistingStandard.page"), `<apex:page standardController="Account" extensions="ExistingExtension" action="{!cancel}"><apex:form /></apex:page>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/Controller.page"), `<apex:page controller="Controller"><apex:form /></apex:page>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/MissingAction.page"), `<apex:page controller="ExistingController" action="{!missing}"><apex:form /></apex:page>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/components/ActionInput.component"), `<apex:component>
  <apex:attribute name="actSupAction" type="ApexPages.Action" description="action" />
  <apex:actionSupport event="onchange" action="{!actSupAction}" />
</apex:component>`)

	report, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}

	if hasLineFindingContaining(report, "lwc.controller-test", "force-app/main/default/lwc/passive/passive.js", "passive") ||
		hasLineFindingContaining(report, "lwc.controller-test", "force-app/main/default/lwc/passive/passive.html", "passive") {
		t.Fatalf("passive LWC files should not be reported as controller-test blockers")
	}
	if hasLineFindingContaining(report, "visualforce.controller-test", "force-app/main/default/pages/Passive.page", "Passive") {
		t.Fatalf("Visualforce pages without controller-facing attributes should not be controller-test blockers")
	}
	if hasLineFinding(report, "visualforce.controller-test", "force-app/main/default/pages/Existing.page", "ExistingController") {
		t.Fatalf("resolved Visualforce controller class should not be reported")
	}
	if hasLineFinding(report, "visualforce.controller-test", "force-app/main/default/pages/Existing.page", "{!save}") {
		t.Fatalf("resolved Visualforce controller action should not be reported")
	}
	if hasLineFinding(report, "visualforce.controller-test", "force-app/main/default/pages/ExistingStandard.page", "ExistingExtension") ||
		hasLineFinding(report, "visualforce.controller-test", "force-app/main/default/pages/ExistingStandard.page", "{!cancel}") {
		t.Fatalf("resolved Visualforce extension contract should not be reported")
	}
	if hasLineFinding(report, "visualforce.controller-test", "force-app/main/default/components/ActionInput.component", "{!actSupAction}") {
		t.Fatalf("resolved Visualforce action attribute should not be reported")
	}
	if !hasLineFinding(report, "visualforce.controller-test", "force-app/main/default/pages/Controller.page", "Controller") {
		t.Fatalf("Visualforce controller attribute should still be reported")
	}
	if !hasLineFinding(report, "visualforce.controller-test", "force-app/main/default/pages/MissingAction.page", "{!missing}") {
		t.Fatalf("missing Visualforce controller action should still be reported")
	}
}

func TestScanSuppressesSupportedVisualforceRuntimeReferences(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/AccountView.page"), `<apex:page standardController="Account" />`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/UsesVisualforce.cls"), `public class UsesVisualforce {
  void run() {
    PageReference page = Page.AccountView;
    ApexPages.currentPage().getParameters().put('id', '001000000000001AAA');
    ApexPages.StandardController controller = new ApexPages.StandardController(new Account(Name = 'Acme'));
  }
  void missing() {
    PageReference page = Page.MissingPage;
  }
}`)

	report, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}

	if hasLineFinding(report, "visualforce.controller-test", "force-app/main/default/classes/UsesVisualforce.cls", "PageReference") {
		t.Fatalf("supported PageReference type usage should not be reported")
	}
	if hasLineFinding(report, "visualforce.controller-test", "force-app/main/default/classes/UsesVisualforce.cls", "ApexPages.") {
		t.Fatalf("supported ApexPages current-page usage should not be reported")
	}
	if hasLineFinding(report, "visualforce.controller-test", "force-app/main/default/classes/UsesVisualforce.cls", "StandardController") {
		t.Fatalf("supported StandardController usage should not be reported")
	}
	if hasLineFinding(report, "visualforce.controller-test", "force-app/main/default/classes/UsesVisualforce.cls", "Page.AccountView") {
		t.Fatalf("registered Page.AccountView reference should not be reported")
	}
	if !hasLineFinding(report, "visualforce.controller-test", "force-app/main/default/classes/UsesVisualforce.cls", "Page.MissingPage") {
		t.Fatalf("missing unresolved Page.MissingPage finding")
	}
	if hasLineFinding(report, "visualforce.controller-test", "force-app/main/default/pages/AccountView.page", "Account") {
		t.Fatalf("resolved Visualforce standard controller object should not be reported")
	}
}

func TestScanSuppressesResolvedLWCControllerImports(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/WidgetController.cls"), `public class WidgetController {
  @AuraEnabled(cacheable=true)
  public static String getWidget() {
    return 'widget';
  }
}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/lwc/widget/widget.js"), `import getWidget from '@salesforce/apex/WidgetController.getWidget';
import missing from '@salesforce/apex/WidgetController.missing';
`)

	report, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}

	if hasLineFinding(report, "lwc.controller-test", "force-app/main/default/lwc/widget/widget.js", "WidgetController.getWidget") {
		t.Fatalf("resolved LWC Apex import should not be reported")
	}
	if !hasLineFinding(report, "lwc.controller-test", "force-app/main/default/lwc/widget/widget.js", "WidgetController.missing") {
		t.Fatalf("missing unresolved LWC Apex import finding")
	}
}

func TestScanTreatsBlobAsSupportedButKeepsFileObjects(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/UsesBinary.cls"), `public class UsesBinary {
  void run() {
    Blob body = Blob.valueOf('hello');
    String encoded = EncodingUtil.base64Encode(body);
    Blob decoded = EncodingUtil.base64Decode(encoded);
    ContentVersion version = new ContentVersion(VersionData = decoded);
    Attachment attachment = new Attachment(Body = body);
  }
}`)

	report, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}

	if hasLineFinding(report, "files.binary-content", "force-app/main/default/classes/UsesBinary.cls", "Blob") ||
		hasLineFinding(report, "files.binary-content", "force-app/main/default/classes/UsesBinary.cls", "base64Encode") ||
		hasLineFinding(report, "files.binary-content", "force-app/main/default/classes/UsesBinary.cls", "base64Decode") {
		t.Fatalf("core Blob and base64 helpers should not be file side-effect blockers")
	}
	if !hasLineFinding(report, "files.binary-content", "force-app/main/default/classes/UsesBinary.cls", "ContentVersion") {
		t.Fatalf("missing ContentVersion file-object finding")
	}
	if !hasLineFinding(report, "files.binary-content", "force-app/main/default/classes/UsesBinary.cls", "Attachment") {
		t.Fatalf("missing Attachment file-object finding")
	}
}

func TestScanSuppressesResolvedCustomMetadataTypeReferences(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Feature__mdt/Feature__mdt.object-meta.xml"), `<CustomObject><label>Feature</label><pluralLabel>Features</pluralLabel></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/UsesMetadata.cls"), `public class UsesMetadata {
  Feature__mdt configured;
  pkg__Feature__mdt namespaced;
  Missing__mdt missing;
  /*
   * BlockCommentOnly__mdt should not count.
   */
  // CommentOnly__mdt should not count.
  String dynamicName = 'StringOnly__mdt';
}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/profiles/Admin.profile-meta.xml"), `<Profile>
  <fieldPermissions><field>Missing__mdt.Enabled__c</field><editable>true</editable></fieldPermissions>
</Profile>`)

	report, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}

	if hasLineFinding(report, "custommetadata.legacy-records", "force-app/main/default/classes/UsesMetadata.cls", "Feature__mdt") {
		t.Fatalf("resolved Feature__mdt type reference should not be reported")
	}
	if hasLineFinding(report, "custommetadata.legacy-records", "force-app/main/default/classes/UsesMetadata.cls", "pkg__Feature__mdt") {
		t.Fatalf("resolved namespaced pkg__Feature__mdt type reference should not be reported")
	}
	if !hasLineFinding(report, "custommetadata.legacy-records", "force-app/main/default/classes/UsesMetadata.cls", "Missing__mdt") {
		t.Fatalf("missing unresolved Missing__mdt finding")
	}
	if hasLineFinding(report, "custommetadata.legacy-records", "force-app/main/default/classes/UsesMetadata.cls", "CommentOnly__mdt") {
		t.Fatalf("comment-only custom metadata mention should not be reported")
	}
	if hasLineFinding(report, "custommetadata.legacy-records", "force-app/main/default/classes/UsesMetadata.cls", "BlockCommentOnly__mdt") {
		t.Fatalf("block-comment-only custom metadata mention should not be reported")
	}
	if hasLineFinding(report, "custommetadata.legacy-records", "force-app/main/default/classes/UsesMetadata.cls", "StringOnly__mdt") {
		t.Fatalf("string-only custom metadata mention should not be reported")
	}
	if hasLineFinding(report, "custommetadata.legacy-records", "force-app/main/default/profiles/Admin.profile-meta.xml", "Missing__mdt") {
		t.Fatalf("profile metadata field permission should not be reported as Apex custom metadata type use")
	}
}

func TestScanSuppressesResolvedLabelReferences(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/labels/CustomLabels.labels"), `<CustomLabels>
  <labels><fullName>Save</fullName><value>Save</value></labels>
  <labels><fullName>Greeting</fullName><value>Hello</value></labels>
</CustomLabels>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/translations/fr.translation-meta.xml"), `<Translations>
  <customLabels><name>Greeting</name><label>Bonjour</label></customLabels>
</Translations>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/UsesLabels.cls"), `public class UsesLabels {
  void run() {
    System.debug(System.Label.Save);
    System.debug(Label.Greeting);
    System.debug(Label.Missing);
  }
}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/lwc/labels/labels.js"), `import SAVE from '@salesforce/label/c.Save';
import MISSING from '@salesforce/label/c.Missing';
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/Labels.page"), `<apex:page>
{!$Label.Save}
{!$Label.Missing}
</apex:page>`)

	report, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}

	if find := findSurface(report, "labels.localization"); find == nil {
		t.Fatalf("expected unresolved label surface")
	}
	if hasLineFinding(report, "labels.localization", "force-app/main/default/classes/UsesLabels.cls", "Save") {
		t.Fatalf("resolved System.Label.Save should not be reported")
	}
	if hasLineFinding(report, "labels.localization", "force-app/main/default/classes/UsesLabels.cls", "Greeting") {
		t.Fatalf("resolved Label.Greeting should not be reported")
	}
	if hasLineFindingContaining(report, "labels.localization", "force-app/main/default/lwc/labels/labels.js", "c.Save") {
		t.Fatalf("resolved LWC c.Save label should not be reported")
	}
	if hasLineFindingContaining(report, "labels.localization", "force-app/main/default/pages/Labels.page", "$Label.Save") {
		t.Fatalf("resolved Visualforce $Label.Save should not be reported")
	}
	if !hasLineFinding(report, "labels.localization", "force-app/main/default/classes/UsesLabels.cls", "Missing") {
		t.Fatalf("missing unresolved Apex label finding")
	}
	if !hasLineFindingContaining(report, "labels.localization", "force-app/main/default/lwc/labels/labels.js", "c.Missing") {
		t.Fatalf("missing unresolved LWC label finding")
	}
	if !hasLineFindingContaining(report, "labels.localization", "force-app/main/default/pages/Labels.page", "$Label.Missing") {
		t.Fatalf("missing unresolved Visualforce label finding")
	}
	for _, finding := range report.Findings {
		if finding.MetadataType == "CustomLabels" {
			t.Fatalf("label metadata files should not be reported as unsupported: %#v", finding)
		}
	}
}

func TestScanSuppressesResolvedResourcesAndEndpoints(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/staticresources/Site.resource"), "body")
	writeFile(t, filepath.Join(root, "force-app/main/default/staticresources/Site.resource-meta.xml"), `<StaticResource><contentType>text/plain</contentType></StaticResource>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/contentassets/Logo.asset"), "body")
	writeFile(t, filepath.Join(root, "force-app/main/default/contentassets/Logo.asset-meta.xml"), `<ContentAsset><contentType>image/png</contentType></ContentAsset>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/namedCredentials/Billing.namedCredential"), `<NamedCredential><endpoint>https://billing.example.test</endpoint></NamedCredential>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/remoteSiteSettings/Maps.remoteSite"), `<RemoteSiteSetting><url>https://maps.example.test</url></RemoteSiteSetting>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/lwc/resources/resources.js"), `import SITE from '@salesforce/resourceUrl/Site';
import MISSING from '@salesforce/resourceUrl/MissingResource';
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/Resources.page"), `<apex:page>
{!URLFOR($Resource.Site, 'css/app.css')}
{!$Resource.Logo}
{!URLFOR($Resource.MissingResource, 'css/app.css')}
</apex:page>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/UsesEndpoint.cls"), `public class UsesEndpoint {
  void run() {
    HttpRequest req = new HttpRequest();
    req.setEndpoint('callout:Billing/v1/accounts');
    req.setEndpoint('callout:MissingEndpoint/v1/accounts');
  }
}`)

	report, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}

	if hasLineFinding(report, "staticresources.urlfor", "force-app/main/default/lwc/resources/resources.js", "Site") {
		t.Fatalf("resolved LWC resource import should not be reported")
	}
	if hasLineFinding(report, "staticresources.urlfor", "force-app/main/default/pages/Resources.page", "Site") {
		t.Fatalf("resolved Visualforce URLFOR resource should not be reported")
	}
	if hasLineFinding(report, "staticresources.urlfor", "force-app/main/default/pages/Resources.page", "Logo") {
		t.Fatalf("resolved Visualforce content asset should not be reported")
	}
	if !hasLineFinding(report, "staticresources.urlfor", "force-app/main/default/lwc/resources/resources.js", "MissingResource") {
		t.Fatalf("missing unresolved LWC resource finding")
	}
	if !hasLineFinding(report, "staticresources.urlfor", "force-app/main/default/pages/Resources.page", "MissingResource") {
		t.Fatalf("missing unresolved Visualforce resource finding")
	}
	if hasLineFinding(report, "endpoint.metadata", "force-app/main/default/classes/UsesEndpoint.cls", "Billing") {
		t.Fatalf("resolved named credential callout should not be reported")
	}
	if !hasLineFinding(report, "endpoint.metadata", "force-app/main/default/classes/UsesEndpoint.cls", "MissingEndpoint") {
		t.Fatalf("missing unresolved named credential callout finding")
	}
	for _, finding := range report.Findings {
		switch finding.MetadataType {
		case "StaticResource", "ContentAsset", "NamedCredential", "RemoteSiteSetting":
			t.Fatalf("loaded metadata files should not be reported as unsupported: %#v", finding)
		}
	}
}

func TestScanRejectsFileRoot(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Only.cls")
	writeFile(t, path, "public class Only {}")
	if _, err := Scan(path); err == nil {
		t.Fatal("expected file root error")
	}
}

func findSurface(report Report, capability string) *Surface {
	for i := range report.Surfaces {
		if report.Surfaces[i].Capability == capability {
			return &report.Surfaces[i]
		}
	}
	return nil
}

func hasLineFinding(report Report, capability, file, symbol string) bool {
	for _, finding := range report.Findings {
		if finding.Capability == capability && finding.File == file && finding.Line > 0 && finding.Symbol == symbol {
			return true
		}
	}
	return false
}

func hasLineFindingContaining(report Report, capability, file, symbol string) bool {
	for _, finding := range report.Findings {
		if finding.Capability == capability && finding.File == file && finding.Line > 0 && strings.Contains(finding.Symbol, symbol) {
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
