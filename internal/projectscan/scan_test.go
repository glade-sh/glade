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
@wire(getCurrencyInformation, { currencyCode: '$guestCurrencyCode' }) value;`)
	writeFile(t, filepath.Join(root, "src/workflows/Account.workflow"), `<Workflow><rules><fullName>Rule</fullName></rules></Workflow>`)
	writeFile(t, filepath.Join(root, "src/flows/Update.flow"), `<Flow><processType>Workflow</processType></Flow>`)
	writeFile(t, filepath.Join(root, "src/labels/CustomLabels.labels"), `<CustomLabels/>`)
	writeFile(t, filepath.Join(root, "src/email/Nimble/Welcome.email"), `Hello {!Contact.Name}`)
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
    Attachment a = new Attachment();
    Community__mdt cfg;
  }
}`)

	report, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}

	wantCaps := []string{
		"apex.callable-stub",
		"aura.controller-test",
		"custommetadata.legacy-records",
		"email.templates",
		"endpoint.metadata",
		"files.binary-content",
		"flow.save-order",
		"labels.localization",
		"lwc.controller-test",
		"metadata.apex-deploy",
		"metadata.legacy-source",
		"platform.cache-connectapi",
		"platform.auth-context",
		"site.community-context",
		"staticresources.urlfor",
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
	}
	if !hasLineFinding(report, "lwc.controller-test", "src/lwc/currencyMenu/currencyMenu.js", "CurrencyMenuController.getCurrencyInformation") {
		t.Fatalf("missing LWC Apex import finding")
	}
	if !hasLineFindingContaining(report, "staticresources.urlfor", "src/pages/Edit.page", "$Resource.Resources") {
		t.Fatalf("missing Visualforce resource finding")
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
