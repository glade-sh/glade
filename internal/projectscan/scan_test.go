package projectscan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/uicontroller"
)

func TestScanFindsProjectGaps(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "src/pages/Edit.page"), `<apex:page controller="EditController" action="{!load}">
  <apex:stylesheet value="{!URLFOR($Resource.Resources, 'site.css')}" />
  <apex:composition template="{!$Site.Template}" />
  {!$Label.EditTitle}
</apex:page>`)
	writeFile(t, filepath.Join(root, "src/components/Picker.component"), `plain text without visualforce tags`)
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
	writeFile(t, filepath.Join(root, "src/workflows/Account.workflow"), `<Workflow><rules><fullName>Rule</fullName><active>true</active><actions><name>FollowUp</name><type>Task</type></actions></rules></Workflow>`)
	writeFile(t, filepath.Join(root, "src/flows/Update.flow"), `<Flow><processType>Workflow</processType><status>Active</status><start><object>Account</object></start><screens><name>Input</name></screens></Flow>`)
	writeFile(t, filepath.Join(root, "src/labels/CustomLabels.labels"), `<CustomLabels/>`)
	writeFile(t, filepath.Join(root, "src/email/Local/Welcome.email"), `Hello {!Contact.Name}`)
	writeFile(t, filepath.Join(root, "other/email/Unsupported.email"), `Hello {!Contact.Name}`)
	writeFile(t, filepath.Join(root, "src/objects/Thing__c.object"), `<CustomObject/>`)
	writeFile(t, filepath.Join(root, "other/objects/Unsupported__c.object"), `<CustomObject/>`)
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
    Metadata.DeployResult deployResult = new Metadata.DeployResult();
    deployResult.status = Metadata.DeployStatus.SUCCEEDED;
    deployResult.details = new Metadata.DeployDetails();
    Metadata.DeployMessage deployMessage = new Metadata.DeployMessage();
    List<Metadata.Metadata> records = Metadata.Operations.retrieve(Metadata.MetadataType.CustomMetadata, new List<String>{'Thing__mdt.Default'});
    Metadata.Operations.updateMetadata(null);
    System.debug('Metadata.StringOnlyType');
    // Metadata.CommentOnlyType should not be counted.
    System.debug(Site.getAdminEmail());
    System.debug(ConnectApi.Organization.getSettings().orgId);
    ConnectApi.UserSettings settings = ConnectApi.Organization.getSettings().userSettings;
    System.debug(settings.timeZone.name);
    ConnectApi.NamedCredentialType namedCredentialType = ConnectApi.NamedCredentialType.SecuredEndpoint;
    System.debug(namedCredentialType);
    System.debug(ConnectApi.NamedCredentials.getExternalCredential('googleBooksAPIApex'));
    System.debug(ConnectApi.ChatterUsers.getFollowings(null, UserInfo.getUserId()));
    System.debug(ConnectApi.Communities.getCommunities().communities);
    Auth.JWT jwt = new Auth.JWT();
    jwt.setIss('issuer');
    Auth.SessionManagement.getCurrentSession();
    Auth.JWTUtil.validateJWTWithKeysEndpoint('token', 'https://example.invalid/keys');
    ConnectApi.ChatterFeeds.getFeedElementsFromFeed(null, null);
    Metadata.UnknownType unsupportedMetadata;
    System.debug(Site.UnknownContext());
    Callable cb;
    Test.createStub(UsesPlatform.class, null);
    req.setEndpoint('callout:Api/path');
    Attachment a = new Attachment();
    Community__mdt cfg;
  }
  global static Metadata.DeployProblemType valueOf(String str) { return null; }
  global static List<Metadata.DeployProblemType> values() { return null; }
}`)
	writeFile(t, filepath.Join(root, "src/classes/ConnectApiWrapper.cls"), `public class ConnectApiWrapper {
  public ConnectApi.ExternalCredential createExternalCredential(ConnectApi.ExternalCredentialInput input) {
    return ConnectApi.NamedCredentials.createExternalCredential(input);
  }
  public ConnectApi.NamedCredential createNamedCredential(ConnectApi.NamedCredentialInput input) {
    return ConnectApi.NamedCredentials.createNamedCredential(input);
  }
}`)
	writeFile(t, filepath.Join(root, "src/classes/DirectNamedCredentialMutation.cls"), `public class DirectNamedCredentialMutation {
  void run(ConnectApi.NamedCredentialInput input) {
    ConnectApi.NamedCredentials.createNamedCredential(input);
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
		"flow.save-order",
		"lwc.controller-test",
		"metadata.apex-deploy",
		"platform.cache-connectapi",
		"platform.auth-context",
		"site.community-context",
		"ui.presentation-metadata",
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
	if surface := findSurface(report, "apex.callable-stub"); surface == nil || surface.Status != "partial" || surface.TestBlocking {
		t.Fatalf("callable/stub surface = %#v, want partial non-blocking", surface)
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
	if hasLineFindingContaining(report, "site.community-context", "src/classes/UsesPlatform.cls", "Site.getAdminEmail") {
		t.Fatalf("supported Site.getAdminEmail was reported as a blocker")
	}
	if hasLineFindingContaining(report, "platform.auth-context", "src/classes/UsesPlatform.cls", "Auth.SessionManagement.getCurrentSession") {
		t.Fatalf("supported Auth.SessionManagement.getCurrentSession was reported as a blocker")
	}
	if hasLineFindingContaining(report, "platform.cache-connectapi", "src/classes/UsesPlatform.cls", "ConnectApi.Organization.getSettings") {
		t.Fatalf("supported ConnectApi.Organization.getSettings was reported as a blocker")
	}
	if hasLineFindingContaining(report, "platform.cache-connectapi", "src/classes/UsesPlatform.cls", "ConnectApi.UserSettings") {
		t.Fatalf("supported ConnectApi.UserSettings was reported as a blocker")
	}
	if hasLineFindingContaining(report, "platform.cache-connectapi", "src/classes/UsesPlatform.cls", "ConnectApi.NamedCredentialType") {
		t.Fatalf("supported ConnectApi enum/type reference was reported as a blocker")
	}
	if hasLineFindingContaining(report, "platform.cache-connectapi", "src/classes/UsesPlatform.cls", "ConnectApi.NamedCredentials.getExternalCredential") {
		t.Fatalf("supported ConnectApi.NamedCredentials.getExternalCredential was reported as a blocker")
	}
	if hasLineFindingContaining(report, "platform.cache-connectapi", "src/classes/UsesPlatform.cls", "ConnectApi.ChatterUsers.getFollowings") {
		t.Fatalf("supported ConnectApi.ChatterUsers.getFollowings was reported as a blocker")
	}
	if hasLineFindingContaining(report, "platform.cache-connectapi", "src/classes/UsesPlatform.cls", "ConnectApi.Communities.getCommunities") {
		t.Fatalf("supported ConnectApi.Communities.getCommunities was reported as a blocker")
	}
	if hasLineFindingContaining(report, "platform.auth-context", "src/classes/UsesPlatform.cls", "Auth.JWT jwt") {
		t.Fatalf("supported Auth.JWT model was reported as a blocker")
	}
	if hasLineFindingContaining(report, "files.binary-content", "src/classes/UsesPlatform.cls", "Attachment") {
		t.Fatalf("supported Attachment SObject usage was reported as a blocker")
	}
	if hasLineFindingContaining(report, "metadata.apex-deploy", "src/classes/UsesPlatform.cls", "Metadata.DeployContainer") {
		t.Fatalf("supported Metadata.DeployContainer model was reported as a blocker")
	}
	if !hasLineFindingContaining(report, "metadata.apex-deploy", "src/classes/UsesPlatform.cls", "Metadata.UnknownType") {
		t.Fatalf("missing unsupported Metadata.UnknownType finding")
	}
	if !hasLineFindingEvidenceContaining(report, "metadata.apex-deploy", "src/classes/UsesPlatform.cls", "Metadata.Operations.updateMetadata") {
		t.Fatalf("missing unsupported Metadata.Operations mutation finding")
	}
	if hasLineFindingContaining(report, "metadata.apex-deploy", "src/classes/UsesPlatform.cls", "Metadata.DeployProblemType") {
		t.Fatalf("generated Metadata enum valueOf/values boilerplate should not be reported as a blocker")
	}
	if hasLineFindingContaining(report, "metadata.apex-deploy", "src/classes/UsesPlatform.cls", "Metadata.StringOnlyType") ||
		hasLineFindingContaining(report, "metadata.apex-deploy", "src/classes/UsesPlatform.cls", "Metadata.CommentOnlyType") {
		t.Fatalf("Metadata references in comments or strings should not be reported as blockers")
	}
	if !hasLineFindingContaining(report, "platform.auth-context", "src/classes/UsesPlatform.cls", "Auth.JWTUtil") {
		t.Fatalf("missing unsupported Auth.JWTUtil finding")
	}
	if !hasLineFindingContaining(report, "platform.cache-connectapi", "src/classes/UsesPlatform.cls", "ConnectApi.ChatterFeeds") {
		t.Fatalf("missing unsupported ConnectApi.ChatterFeeds finding")
	}
	if hasLineFindingContaining(report, "platform.cache-connectapi", "src/classes/ConnectApiWrapper.cls", "ConnectApi.NamedCredentials") {
		t.Fatalf("mockable ConnectApi wrapper seam was reported as a blocker")
	}
	if !hasLineFindingContaining(report, "platform.cache-connectapi", "src/classes/DirectNamedCredentialMutation.cls", "ConnectApi.NamedCredentials") {
		t.Fatalf("direct ConnectApi named credential mutation should remain a blocker")
	}
	if !hasLineFindingEvidenceContaining(report, "site.community-context", "src/classes/UsesPlatform.cls", "Site.UnknownContext") {
		t.Fatalf("missing unsupported Site.UnknownContext finding")
	}
	for _, finding := range report.Findings {
		if strings.Contains(finding.File, ".claude/") {
			t.Fatalf("scanner included generated agent worktree file: %#v", finding)
		}
	}
}

func TestScanMetadataProjectCollectsAnalyticsInSamePass(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"src","default":true}]}`)
	layoutPath := filepath.Join(root, "src/layouts/Account.layout")
	reportPath := filepath.Join(root, "src/reports/Sales.pipeline.report-meta.xml")
	dashboardPath := filepath.Join(root, "src/dashboards/Exec.pipeline.dashboard-meta.xml")
	pagePath := filepath.Join(root, "outside/pages/Loose.page")
	writeFile(t, layoutPath, `<Layout/>`)
	writeFile(t, reportPath, `<Report/>`)
	writeFile(t, dashboardPath, `<Dashboard/>`)
	writeFile(t, pagePath, `<apex:page />`)

	proj, err := loadScanProjectWithAnalytics(root)
	if err != nil {
		t.Fatal(err)
	}

	if len(proj.project.LayoutFiles) != 1 || filepath.Clean(proj.project.LayoutFiles[0]) != filepath.Clean(layoutPath) {
		t.Fatalf("layout metadata was not collected from scan pass: %#v", proj.project.LayoutFiles)
	}
	if !proj.reports[filepath.Clean(reportPath)] {
		t.Fatalf("report metadata was not collected from scan pass: %#v", proj.reports)
	}
	if !proj.dashboards[filepath.Clean(dashboardPath)] {
		t.Fatalf("dashboard metadata was not collected from scan pass: %#v", proj.dashboards)
	}
	if len(proj.project.VisualforcePageFiles) != 1 || filepath.Clean(proj.project.VisualforcePageFiles[0]) != filepath.Clean(pagePath) {
		t.Fatalf("Visualforce page metadata was not collected from scan pass: %#v", proj.project.VisualforcePageFiles)
	}
}

func TestScanSuppressesSupportedCallableStubSurface(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"src","default":true}]}`)
	writeFile(t, filepath.Join(root, "src/classes/Greeter.cls"), `public interface Greeter {
  String greet(String name);
}`)
	writeFile(t, filepath.Join(root, "src/classes/GreeterProvider.cls"), `private class GreeterProvider implements System.StubProvider {
  public Object handleMethodCall(Object stubbedObject, String stubbedMethodName, Type returnType, List<Type> listOfParamTypes, List<String> listOfParamNames, List<Object> listOfArgs) {
    return 'stubbed';
  }
}`)
	writeFile(t, filepath.Join(root, "src/classes/LocalCallable.cls"), `public class LocalCallable implements System.Callable {
  public Object call(String action, Map<String, Object> args) {
    return action;
  }
}`)
	writeFile(t, filepath.Join(root, "src/classes/PlatformApisTest.cls"), `@isTest
private class PlatformApisTest {
  @isTest static void supportedCallableAndStub() {
    Callable cb = (Callable) new LocalCallable();
    System.assert(cb instanceof System.Callable);
    Greeter greeter = Test.createStub(Greeter.class, new GreeterProvider());
    System.assertEquals('stubbed', greeter.greet('Ada'));
  }
}`)

	report, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if surface := findSurface(report, "apex.callable-stub"); surface != nil {
		t.Fatalf("callable/stub surface = %#v, want suppressed supported surface", surface)
	}
}

func TestScanSuppressesSupportedReportsEnumReferences(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"src","default":true}]}`)
	writeFile(t, filepath.Join(root, "src/classes/ReportFormatUser.cls"), `public class ReportFormatUser {
  public String run() {
    return Reports.ReportFormat.SUMMARY.name();
  }
}`)

	report, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if hasLineFindingContaining(report, "analytics.report-execution", "src/classes/ReportFormatUser.cls", "Reports.ReportFormat.SUMMARY") {
		t.Fatalf("supported Reports.ReportFormat enum reference should not be reported")
	}
}

func TestScanSuppressesConnectApiCallsGuardedOutOfTests(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"src","default":true}]}`)
	writeFile(t, filepath.Join(root, "src/classes/ChatterPoster.cls"), `public class ChatterPoster {
  public void guarded(ConnectApi.FeedItemInput input) {
    if (!Test.isRunningTest()) { ConnectApi.ChatterFeeds.postFeedElement(Network.getNetworkId(), input); }
  }
  public void unguarded(ConnectApi.FeedItemInput input) {
    ConnectApi.ChatterFeeds.postFeedElement(Network.getNetworkId(), input);
  }
  public void guardedBlock(ConnectApi.FeedItemInput input) {
    if (!Test.isRunningTest()) {
      ConnectApi.ChatterFeeds.postFeedElement(Network.getNetworkId(), input);
    }
  }
  public void guardedAnd(ConnectApi.FeedItemInput input, Boolean enabled) {
    if (enabled && !Test.isRunningTest()) {
      ConnectApi.ChatterFeeds.postFeedElement(Network.getNetworkId(), input);
    }
  }
  public void guardedOr(ConnectApi.FeedItemInput input, Boolean force) {
    if (!Test.isRunningTest() || force) {
      ConnectApi.ChatterFeeds.postFeedElement(Network.getNetworkId(), guardedOrInput);
    }
  }
}`)

	report, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if hasLineFindingEvidenceContaining(report, "platform.cache-connectapi", "src/classes/ChatterPoster.cls", "if (!Test.isRunningTest())") {
		t.Fatalf("test-guarded ConnectApi call should not be reported")
	}
	if hasLineFindingEvidenceContaining(report, "platform.cache-connectapi", "src/classes/ChatterPoster.cls", "guardedBlock") ||
		hasLineFindingEvidenceContaining(report, "platform.cache-connectapi", "src/classes/ChatterPoster.cls", "guardedAnd") {
		t.Fatalf("multiline test-guarded ConnectApi call should not be reported")
	}
	if !hasLineFindingContaining(report, "platform.cache-connectapi", "src/classes/ChatterPoster.cls", "ConnectApi.ChatterFeeds") {
		t.Fatalf("unguarded ChatterFeeds call should remain a blocker")
	}
	if !hasLineFindingEvidenceContaining(report, "platform.cache-connectapi", "src/classes/ChatterPoster.cls", "guardedOr") {
		t.Fatalf("or-guarded ChatterFeeds call can still run in tests and should remain a blocker")
	}
}

func TestScanSuppressesCalloutMockKeysWithoutEndpointMetadata(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"src","default":true}]}`)
	writeFile(t, filepath.Join(root, "src/classes/EndpointMocks.cls"), `@IsTest
private class EndpointMocks {
  static void setup() {
    MultiStaticResourceCalloutMock multimock = new MultiStaticResourceCalloutMock();
    multimock.setStaticResource('callout:MissingCredential', 'Fixture');
  }
  static void assertEndpoint(HttpRequest req) {
    System.assertEquals('callout:AssertOnlyCredential', req.getEndpoint());
  }
  static void send(HttpRequest req) {
    req.setEndpoint('callout:MissingCredential');
  }
  static void guarded(HttpRequest req) {
    if (!Test.isRunningTest()) {
      req.setEndpoint('callout:GuardedCredential');
    }
  }
  static void guardedOr(HttpRequest req, Boolean force) {
    if (!Test.isRunningTest() || force) {
      req.setEndpoint('callout:GuardedOrCredential');
    }
  }
}`)

	report, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if hasLineFindingEvidenceContaining(report, "endpoint.metadata", "src/classes/EndpointMocks.cls", "setStaticResource") {
		t.Fatalf("static-resource callout mock key should not require endpoint metadata")
	}
	if hasLineFinding(report, "endpoint.metadata", "src/classes/EndpointMocks.cls", "AssertOnlyCredential") {
		t.Fatalf("assert-only endpoint literal should not require named credential metadata")
	}
	if !hasLineFindingEvidenceContaining(report, "endpoint.metadata", "src/classes/EndpointMocks.cls", "req.setEndpoint") {
		t.Fatalf("real callout endpoint should still require named credential metadata")
	}
	if hasLineFinding(report, "endpoint.metadata", "src/classes/EndpointMocks.cls", "GuardedCredential") {
		t.Fatalf("test-guarded endpoint should not require named credential metadata")
	}
	if !hasLineFinding(report, "endpoint.metadata", "src/classes/EndpointMocks.cls", "GuardedOrCredential") {
		t.Fatalf("or-guarded endpoint can still run in tests and should remain a blocker")
	}
}

func TestScanSuppressesSupportedAuthAndNetworkLocalContracts(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"src","default":true}]}`)
	writeFile(t, filepath.Join(root, "src/classes/AuthNetworkContracts.cls"), `global class SamlHandler implements Auth.SamlJitHandler {
  global User createUser(Id samlSsoProviderId, Id communityId, Id portalId, String federationIdentifier, Map<String,String> attributes, String assertion) { return new User(); }
  global void updateUser(Id userId, Id samlSsoProviderId, Id communityId, Id portalId, String federationIdentifier, Map<String,String> attributes, String assertion) {}
}
global class OAuthProvider extends Auth.AuthProviderPluginClass {
  global override PageReference initiate(Map<String,String> config, String stateToPropagate) { return null; }
  global override Auth.AuthProviderTokenResponse handleCallback(Map<String,String> config, Auth.AuthProviderCallbackState state) { return null; }
  global override Auth.UserData getUserInfo(Map<String,String> config, Auth.AuthProviderTokenResponse response) { return null; }
  global override String getCustomMetadataType() { return 'OAuth__mdt'; }
}
public class AuthNetworkContracts {
  public void run(Network n, User u, Contact c, Account a) {
    String logout = System.Network.getLogoutUrl(n.Id);
    String selfReg = System.Network.getSelfRegUrl(n.Id);
    PageReference auth = Network.forwardToAuthPage('/start', 'page');
    String domain = Site.getDomain();
    String siteName = Site.getName();
    String prefix = Site.getPrefix();
    Id networkID = Network.getNetworkID();
    List<Network> networks = [SELECT Id FROM Network WHERE Status = 'Live'];
    String jobId = System.Network.createExternalUserAsync(u, c, a);
    Auth.JWTUtil.validateJWTWithKeysEndpoint('token', 'https://example.invalid/keys');
  }
}`)

	report, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, symbol := range []string{"Auth.SamlJitHandler", "Auth.AuthProviderPluginClass", "Auth.AuthProviderCallbackState", "Auth.AuthProviderTokenResponse"} {
		if hasLineFindingContaining(report, "platform.auth-context", "src/classes/AuthNetworkContracts.cls", symbol) {
			t.Fatalf("supported Auth local contract %s should not be reported", symbol)
		}
	}
	for _, evidence := range []string{"System.Network.getLogoutUrl", "System.Network.getSelfRegUrl", "Network.forwardToAuthPage", "Site.getDomain", "Site.getName", "Site.getPrefix", "Network.getNetworkID", "FROM Network", "System.Network.createExternalUserAsync"} {
		if hasLineFindingEvidenceContaining(report, "site.community-context", "src/classes/AuthNetworkContracts.cls", evidence) {
			t.Fatalf("supported Network local contract %s should not be reported", evidence)
		}
	}
	if !hasLineFindingContaining(report, "platform.auth-context", "src/classes/AuthNetworkContracts.cls", "Auth.JWTUtil") {
		t.Fatalf("JWT key endpoint validation should remain a blocker")
	}
}

func TestScanKeepsUnsupportedCreateStubNullProvider(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"src","default":true}]}`)
	writeFile(t, filepath.Join(root, "src/classes/CreateStubNullTest.cls"), `@isTest
private class CreateStubNullTest {
  @isTest static void unsupportedNullProvider() {
    Test.createStub(CreateStubNullTest.class, null);
  }
}`)

	report, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	surface := findSurface(report, "apex.callable-stub")
	if surface == nil || surface.Count != 1 {
		t.Fatalf("callable/stub surface = %#v, want one unsupported null-provider finding", surface)
	}
	if got := report.Findings[0].Symbol; got != "Test.createStub" {
		t.Fatalf("symbol = %q, want Test.createStub", got)
	}
}

func TestScanSuppressesLoadedLegacyObjectSource(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"src","default":true}]}`)
	writeFile(t, filepath.Join(root, "src/objects/Legacy__c.object"), `<CustomObject>
  <label>Legacy</label>
  <pluralLabel>Legacy</pluralLabel>
  <fields><fullName>Code__c</fullName><label>Code</label><type>Text</type></fields>
</CustomObject>`)

	report, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if hasLineFinding(report, "metadata.legacy-source", "src/objects/Legacy__c.object", "Legacy__c") {
		t.Fatalf("loaded legacy object should not be reported as a source blocker")
	}
}

func TestScanSuppressesCustomMetadataStubSelfReferences(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"src","default":true}]}`)
	writeFile(t, filepath.Join(root, "stubs/apex-sobject-stubs/ProbeTestMdt__mdt.cls"), `global class ProbeTestMdt__mdt extends SObject {
  public ProbeTestMdt__mdt() {}
}`)
	writeFile(t, filepath.Join(root, "src/classes/UsesMetadata.cls"), `public class UsesMetadata {
  void run() {
    MissingConfig__mdt cfg;
  }
}`)

	report, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if hasLineFinding(report, "custommetadata.legacy-records", "stubs/apex-sobject-stubs/ProbeTestMdt__mdt.cls", "ProbeTestMdt__mdt") {
		t.Fatalf("stub self-reference should not be reported as a project custom metadata blocker")
	}
	if !hasLineFinding(report, "custommetadata.legacy-records", "src/classes/UsesMetadata.cls", "MissingConfig__mdt") {
		t.Fatalf("real unresolved app custom metadata reference was not reported")
	}
}

func TestScanSuppressesResolvedStandardSchemaReferences(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"pkg"}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/lwc/resolved/resolved.js"), `import ACCOUNT_NAME from '@salesforce/schema/Account.Name';
import LEAD_LAST_NAME from '@salesforce/schema/Lead.LastName';
import CONTACT_ACCOUNT_NAME from '@salesforce/schema/Contact.Account.Name';
import OCR_CONTACT_EMAIL from '@salesforce/schema/OpportunityContactRole.Contact.Email';
import OCR_OPPORTUNITY_STAGE from '@salesforce/schema/OpportunityContactRole.Opportunity.StageName';
import BATCH_OBJECT from '@salesforce/schema/Batch__c';
import BATCH_PARENT_NAME from '@salesforce/schema/Batch__c.Parent__r.Name';
import PAYMENT_AMOUNT from '@salesforce/schema/pkg__Payment__c.pkg__Amount__c';
import PAYMENT_BATCH_NAME from '@salesforce/schema/pkg__Payment__c.pkg__Batch__r.Name';
import MANAGED_PAYMENT_AMOUNT from '@salesforce/schema/pkg1__ManagedPayment__c.pkg1__Payment_Amount__c';
import MANAGED_RECURRING_INSTALLMENT from '@salesforce/schema/pkg2__Managed_Renewal__c.pkg2__Installment_Period__c';
import FORM_TEMPLATE_MODIFIED from '@salesforce/schema/Form_Template__c.LastModifiedDate';
import FORM_TEMPLATE_VIEWED from '@salesforce/schema/Form_Template__c.LastViewedDate';
import FORM_TEMPLATE_REFERENCED from '@salesforce/schema/Form_Template__c.LastReferencedDate';
import RECURRING_ORG_NAME from '@salesforce/schema/pkg2__Managed_Renewal__c.pkg2__Organization__r.Name';
import RECURRING_ORG_CONTACT_LAST from '@salesforce/schema/pkg2__Managed_Renewal__c.pkg2__Organization__r.pkg1__PrimaryContact__r.LastName';
import ACCOUNT_ONE_TO_ONE_LAST from '@salesforce/schema/Account.pkg1__PrimaryContact__r.LastName';
import FORM_TEMPLATE_PRESENTATION_PATH from '@salesforce/schema/Form_Template__c.Requester__r.LastModifiedDate';
import EXTERNAL_MANAGED_FIELD from '@salesforce/schema/ext__Managed__c.ext__Amount__c';
import EXTERNAL_MANAGED_RELATIONSHIP from '@salesforce/schema/ext__Managed__c.ext__Account__r.Name';
import EXTERNAL_MANAGED_NESTED_RELATIONSHIP from '@salesforce/schema/ext__Managed__c.ext__Account__r.ext__Primary_Contact__r.LastName';
import MISSING_FIELD from '@salesforce/schema/Account.NotAField__c';
import MISSING_RELATIONSHIP from '@salesforce/schema/Batch__c.Missing__r.Name';
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Batch__c/Batch__c.object-meta.xml"), `<CustomObject><label>Batch</label><pluralLabel>Batches</pluralLabel></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Batch__c/fields/Amount__c.field-meta.xml"), `<CustomField><fullName>Amount__c</fullName><label>Amount</label><type>Currency</type></CustomField>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Batch__c/fields/Parent__c.field-meta.xml"), `<CustomField><fullName>Parent__c</fullName><label>Parent</label><type>Lookup</type><referenceTo>Account</referenceTo><relationshipName>Parent__r</relationshipName></CustomField>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Batch__c/fieldSets/BatchDetailView.fieldSet-meta.xml"), `<FieldSet>
  <fullName>BatchDetailView</fullName>
  <displayedFields><field>Name</field></displayedFields>
</FieldSet>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/DuplicateRecordSet/fieldSets/ContactMergeDRS.fieldSet-meta.xml"), `<FieldSet>
  <fullName>ContactMergeDRS</fullName>
  <displayedFields><field>Name</field></displayedFields>
</FieldSet>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/ServiceResource/ServiceResource.object-meta.xml"), `<CustomObject>
  <fieldSets>
    <fullName>FSL__CrewManagment_Lightbox</fullName>
    <displayedFields><field>Name</field></displayedFields>
  </fieldSets>
</CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/pkg__Payment__c/pkg__Payment__c.object-meta.xml"), `<CustomObject><label>Payment</label><pluralLabel>Payments</pluralLabel></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/pkg__Payment__c/fields/pkg__Amount__c.field-meta.xml"), `<CustomField><fullName>pkg__Amount__c</fullName><label>Amount</label><type>Currency</type></CustomField>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/pkg__Payment__c/fields/pkg__Batch__c.field-meta.xml"), `<CustomField><fullName>pkg__Batch__c</fullName><label>Batch</label><type>Lookup</type><referenceTo>Batch__c</referenceTo><relationshipName>pkg__Batch__r</relationshipName></CustomField>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/Account.object-meta.xml"), `<CustomObject><label>Account</label><pluralLabel>Accounts</pluralLabel></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/fields/pkg1__PrimaryContact__c.field-meta.xml"), `<CustomField><fullName>pkg1__PrimaryContact__c</fullName><label>One-to-One Contact</label><type>Lookup</type><referenceTo>Contact</referenceTo></CustomField>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Form_Template__c/Form_Template__c.object-meta.xml"), `<CustomObject><label>Form Template</label><pluralLabel>Form Templates</pluralLabel></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Form_Template__c/fieldSets/Template_Fields.fieldSet-meta.xml"), `<FieldSet>
  <fullName>Template_Fields</fullName>
  <displayedFields><field>Requester__r.LastModifiedDate</field></displayedFields>
</FieldSet>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/pkg1__ManagedPayment__c/pkg1__ManagedPayment__c.object-meta.xml"), `<CustomObject><label>Payment</label><pluralLabel>Payments</pluralLabel></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/pkg1__ManagedPayment__c/fieldSets/Payment_WizardFS.fieldSet-meta.xml"), `<FieldSet>
  <fullName>Payment_WizardFS</fullName>
  <displayedFields><field>pkg1__Payment_Amount__c</field></displayedFields>
</FieldSet>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/pkg2__Managed_Renewal__c/pkg2__Managed_Renewal__c.object-meta.xml"), `<CustomObject><label>Managed Renewal</label><pluralLabel>Managed Renewals</pluralLabel></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/pkg2__Managed_Renewal__c/fields/pkg2__Organization__c.field-meta.xml"), `<CustomField><fullName>pkg2__Organization__c</fullName><label>Organization</label><type>Lookup</type><referenceTo>Account</referenceTo></CustomField>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/quickActions/New_Managed_Renewal.quickAction-meta.xml"), `<QuickAction>
  <quickActionLayout><layoutSection><layoutColumns><layoutItems><field>pkg2__Installment_Period__c</field></layoutItems></layoutColumns></layoutSection></quickActionLayout>
  <targetObject>pkg2__Managed_Renewal__c</targetObject>
</QuickAction>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/ServiceResource.object"), `<CustomObject>
  <fieldSets>
    <fullName>FSL__CrewManagment_Lightbox</fullName>
    <displayedFields><field>Name</field></displayedFields>
  </fieldSets>
</CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/Resolved.page"), `<apex:page>
{!$ObjectType.Opportunity.Fields.StageName.Label}
{!$ObjectType.Batch__c.Fields.Name.InlineHelpText}
{!$ObjectType.Batch__c.Fields.pkg__Amount__c.Label}
{!$ObjectType.Batch__c.Parent__r.Name}
{!$ObjectType.Batch__c.FieldSets.BatchDetailView}
{!$ObjectType.DuplicateRecordSet.FieldSets.ContactMergeDRS}
{!$ObjectType.ServiceResource.FieldSets.FSL__CrewManagment_Lightbox}
{!$ObjectType.Opportunity.Createable}
{!$ObjectType.Contact.fields.FirstName.Createable}
{!$ObjectType.OpportunityContactRole.Fields.ContactId.Label}
{!$ObjectType.OpportunityContactRole.Contact.Email}
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
	if hasLineFindingContaining(report, "ui.presentation-metadata", "force-app/main/default/lwc/resolved/resolved.js", "Contact.Account.Name") {
		t.Fatalf("resolved Contact.Account.Name relationship schema import should not be reported")
	}
	if hasLineFindingContaining(report, "ui.presentation-metadata", "force-app/main/default/lwc/resolved/resolved.js", "OpportunityContactRole.Contact.Email") {
		t.Fatalf("resolved OpportunityContactRole.Contact.Email schema import should not be reported")
	}
	if hasLineFindingContaining(report, "ui.presentation-metadata", "force-app/main/default/lwc/resolved/resolved.js", "OpportunityContactRole.Opportunity.StageName") {
		t.Fatalf("resolved OpportunityContactRole.Opportunity.StageName schema import should not be reported")
	}
	if hasLineFinding(report, "ui.presentation-metadata", "force-app/main/default/lwc/resolved/resolved.js", "Batch__c") {
		t.Fatalf("resolved Batch__c schema import should not be reported")
	}
	if hasLineFindingContaining(report, "ui.presentation-metadata", "force-app/main/default/lwc/resolved/resolved.js", "Batch__c.Parent__r.Name") {
		t.Fatalf("resolved custom relationship schema import should not be reported")
	}
	if hasLineFindingContaining(report, "ui.presentation-metadata", "force-app/main/default/lwc/resolved/resolved.js", "pkg__Payment__c.pkg__Amount__c") {
		t.Fatalf("resolved package object field schema import should not be reported")
	}
	if hasLineFindingContaining(report, "ui.presentation-metadata", "force-app/main/default/lwc/resolved/resolved.js", "pkg__Payment__c.pkg__Batch__r.Name") {
		t.Fatalf("resolved package relationship schema import should not be reported")
	}
	if hasLineFindingContaining(report, "ui.presentation-metadata", "force-app/main/default/lwc/resolved/resolved.js", "pkg1__ManagedPayment__c.pkg1__Payment_Amount__c") {
		t.Fatalf("loaded field-set field reference should not be reported")
	}
	if hasLineFindingContaining(report, "ui.presentation-metadata", "force-app/main/default/lwc/resolved/resolved.js", "pkg2__Managed_Renewal__c.pkg2__Installment_Period__c") {
		t.Fatalf("loaded quick-action field reference should not be reported")
	}
	if hasLineFindingContaining(report, "ui.presentation-metadata", "force-app/main/default/lwc/resolved/resolved.js", "Form_Template__c.LastModifiedDate") {
		t.Fatalf("custom object standard audit field reference should not be reported")
	}
	if hasLineFindingContaining(report, "ui.presentation-metadata", "force-app/main/default/lwc/resolved/resolved.js", "Form_Template__c.LastViewedDate") {
		t.Fatalf("custom object standard LastViewedDate field reference should not be reported")
	}
	if hasLineFindingContaining(report, "ui.presentation-metadata", "force-app/main/default/lwc/resolved/resolved.js", "Form_Template__c.LastReferencedDate") {
		t.Fatalf("custom object standard LastReferencedDate field reference should not be reported")
	}
	if hasLineFindingContaining(report, "ui.presentation-metadata", "force-app/main/default/lwc/resolved/resolved.js", "pkg2__Managed_Renewal__c.pkg2__Organization__r.Name") {
		t.Fatalf("namespaced custom relationship path should not be reported")
	}
	if hasLineFindingContaining(report, "ui.presentation-metadata", "force-app/main/default/lwc/resolved/resolved.js", "pkg2__Managed_Renewal__c.pkg2__Organization__r.pkg1__PrimaryContact__r.LastName") {
		t.Fatalf("nested custom relationship path should not be reported")
	}
	if hasLineFindingContaining(report, "ui.presentation-metadata", "force-app/main/default/lwc/resolved/resolved.js", "Account.pkg1__PrimaryContact__r.LastName") {
		t.Fatalf("standard object custom relationship path should not be reported")
	}
	if hasLineFindingContaining(report, "ui.presentation-metadata", "force-app/main/default/lwc/resolved/resolved.js", "Form_Template__c.Requester__r.LastModifiedDate") {
		t.Fatalf("presentation-declared dotted field path should not be reported")
	}
	if hasLineFindingContaining(report, "ui.presentation-metadata", "force-app/main/default/lwc/resolved/resolved.js", "ext__Managed__c.ext__Amount__c") {
		t.Fatalf("external managed-package field reference should not be reported")
	}
	if hasLineFindingContaining(report, "ui.presentation-metadata", "force-app/main/default/lwc/resolved/resolved.js", "ext__Managed__c.ext__Account__r.Name") {
		t.Fatalf("external managed-package relationship reference should not be reported")
	}
	if hasLineFindingContaining(report, "ui.presentation-metadata", "force-app/main/default/lwc/resolved/resolved.js", "ext__Managed__c.ext__Account__r.ext__Primary_Contact__r.LastName") {
		t.Fatalf("nested external managed-package relationship reference should not be reported")
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
	if hasLineFindingContaining(report, "ui.presentation-metadata", "force-app/main/default/pages/Resolved.page", "Batch__c.Parent__r.Name") {
		t.Fatalf("resolved Visualforce relationship object type reference should not be reported")
	}
	if hasLineFindingContaining(report, "ui.presentation-metadata", "force-app/main/default/pages/Resolved.page", "Batch__c.FieldSets.BatchDetailView") {
		t.Fatalf("resolved Visualforce field set reference should not be reported")
	}
	if hasLineFindingContaining(report, "ui.presentation-metadata", "force-app/main/default/pages/Resolved.page", "DuplicateRecordSet.FieldSets.ContactMergeDRS") {
		t.Fatalf("resolved standard-object Visualforce field set reference should not be reported")
	}
	if hasLineFindingContaining(report, "ui.presentation-metadata", "force-app/main/default/pages/Resolved.page", "ServiceResource.FieldSets.FSL__CrewManagment_Lightbox") {
		t.Fatalf("inline object field set reference should not be reported")
	}
	if hasLineFindingContaining(report, "ui.presentation-metadata", "force-app/main/default/pages/Resolved.page", "Opportunity.Createable") {
		t.Fatalf("resolved Visualforce object permission property should not be reported")
	}
	if hasLineFindingContaining(report, "ui.presentation-metadata", "force-app/main/default/pages/Resolved.page", "Contact.fields.FirstName.Createable") {
		t.Fatalf("resolved Visualforce field permission property should not be reported")
	}
	if hasLineFindingContaining(report, "ui.presentation-metadata", "force-app/main/default/pages/Resolved.page", "OpportunityContactRole.Fields.ContactId") {
		t.Fatalf("resolved standard OpportunityContactRole field reference should not be reported")
	}
	if hasLineFindingContaining(report, "ui.presentation-metadata", "force-app/main/default/pages/Resolved.page", "OpportunityContactRole.Contact.Email") {
		t.Fatalf("resolved standard OpportunityContactRole relationship reference should not be reported")
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
	if !hasLineFindingContaining(report, "ui.presentation-metadata", "force-app/main/default/lwc/resolved/resolved.js", "Batch__c.Missing__r.Name") {
		t.Fatalf("missing unresolved relationship schema finding")
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
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/BasePanel.cls"), `public virtual class BasePanel {
  public virtual PageReference editSettings() { return null; }
  public virtual PageReference saveSettings() { return null; }
}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/InheritedPanel.cls"), `public class InheritedPanel extends BasePanel {
}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/InheritedExtension.cls"), `public class InheritedExtension extends BasePanel {
  public InheritedExtension(ApexPages.StandardController controller) {}
}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/AuditExtension.cls"), `public class AuditExtension {
  public AuditExtension(ApexPages.StandardController controller) {}
}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/pkg__NamespacedController.cls"), `public class pkg__NamespacedController {
  public PageReference save() { return null; }
}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/pkg__QuoteExtController.cls"), `public class pkg__QuoteExtController {
  public PageReference onSubmit() { return null; }
}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/lwc/passive/passive.js"), `import { LightningElement } from 'lwc';
export default class Passive extends LightningElement {}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/lwc/passive/passive.html"), `<template><span>Passive</span></template>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/Passive.page"), `<apex:page><apex:outputText value="Passive"/></apex:page>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/Existing.page"), `<apex:page controller="ExistingController" action="{!save}"><apex:form /></apex:page>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/ExistingStandard.page"), `<apex:page standardController="Account" extensions="ExistingExtension" action="{!cancel}"><apex:form /></apex:page>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/ExistingStandardMulti.page"), `<apex:page standardController="Account" extensions="ExistingExtension, AuditExtension"><apex:form /></apex:page>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/FSL__Known__c/FSL__Known__c.object-meta.xml"), `<CustomObject><fullName>FSL__Known__c</fullName><label>Known</label><pluralLabel>Known</pluralLabel></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/pkg__Quote__c/pkg__Quote__c.object-meta.xml"), `<CustomObject><label>Quote</label><pluralLabel>Quotes</pluralLabel></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/ManagedPackage.page"), `<apex:page controller="FSL.ManagedController">
{!$ObjectType.ManagedSettings__c.fields.MissingFromLocalMetadata__c.Name}
{!$ObjectType.Account.fields.MissingFromLocalMetadata__c.Name}
<apex:commandButton action="{!refresh}" />
</apex:page>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/NamespacedController.page"), `<apex:page controller="pkg.NamespacedController" action="{!save}" />`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/NamespacedExtension.page"), `<apex:page standardController="pkg__Quote__c" extensions="QuoteExtController" action="{!onSubmit}" />`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/Inherited.page"), `<apex:page controller="InheritedPanel">
  <apex:commandButton action="{!editSettings}" />
</apex:page>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/InheritedStandard.page"), `<apex:page standardController="Account" extensions="InheritedExtension">
  <apex:commandButton action="{!saveSettings}" />
</apex:page>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/SetNavigation.page"), `<apex:page standardController="Account" recordSetVar="allocations" action="{!setCon.first}"><apex:form /></apex:page>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/FormulaAction.page"), `<apex:page controller="ExistingController" action="{!if(true, save, null)}"><apex:form /></apex:page>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/RowItemAction.page"), `<apex:page controller="ExistingController"><apex:repeat value="{!items}" var="item"><apex:commandLink action="{!item.editItem}" /></apex:repeat></apex:page>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/Controller.page"), `<apex:page controller="Controller"><apex:form /></apex:page>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/MissingAction.page"), `<apex:page controller="ExistingController" action="{!missing}"><apex:form /></apex:page>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/MissingCustomQuickSave.page"), `<apex:page controller="ExistingController" action="{!quickSave}"><apex:form /></apex:page>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/components/ActionInput.component"), `<apex:component>
  <apex:attribute name="actSupAction" type="ApexPages.Action" description="action" />
  <apex:actionSupport event="onchange" action="{!actSupAction}" />
</apex:component>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/components/SetNavigation.component"), `<apex:component>
  <apex:commandLink action="{!setCon.next}" />
</apex:component>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/components/Indexed.component"), `<apex:component controller="ExistingController">
  <apex:attribute name="value" type="String" assignTo="{!value}" />
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
	if hasLineFinding(report, "visualforce.controller-test", "force-app/main/default/pages/ExistingStandardMulti.page", "ExistingExtension, AuditExtension") {
		t.Fatalf("resolved Visualforce extension list should not be reported")
	}
	if hasLineFinding(report, "visualforce.controller-test", "force-app/main/default/pages/ManagedPackage.page", "FSL.ManagedController") {
		t.Fatalf("external managed-package Visualforce controller should not be reported when the namespace is known")
	}
	if hasLineFinding(report, "visualforce.controller-test", "force-app/main/default/pages/ManagedPackage.page", "{!refresh}") {
		t.Fatalf("external managed-package Visualforce page actions should not require local controller methods")
	}
	if hasLineFindingContaining(report, "ui.presentation-metadata", "force-app/main/default/pages/ManagedPackage.page", "ManagedSettings__c.fields.MissingFromLocalMetadata__c") {
		t.Fatalf("external managed custom object references should resolve as managed metadata dependencies")
	}
	if !hasLineFindingContaining(report, "ui.presentation-metadata", "force-app/main/default/pages/ManagedPackage.page", "Account.fields.MissingFromLocalMetadata__c") {
		t.Fatalf("external managed Visualforce controller should not suppress standard-object presentation metadata")
	}
	if hasLineFinding(report, "visualforce.controller-test", "force-app/main/default/pages/NamespacedController.page", "pkg.NamespacedController") ||
		hasLineFinding(report, "visualforce.controller-test", "force-app/main/default/pages/NamespacedController.page", "{!save}") {
		t.Fatalf("dot-namespace Visualforce controller should resolve to local namespaced Apex class")
	}
	if hasLineFinding(report, "visualforce.controller-test", "force-app/main/default/pages/NamespacedExtension.page", "QuoteExtController") ||
		hasLineFinding(report, "visualforce.controller-test", "force-app/main/default/pages/NamespacedExtension.page", "{!onSubmit}") {
		t.Fatalf("bare Visualforce extension on namespaced standard controller should resolve to namespaced Apex class")
	}
	if hasLineFinding(report, "visualforce.controller-test", "force-app/main/default/pages/Inherited.page", "{!editSettings}") {
		t.Fatalf("inherited Visualforce controller action should not be reported")
	}
	if hasLineFinding(report, "visualforce.controller-test", "force-app/main/default/pages/InheritedStandard.page", "{!saveSettings}") {
		t.Fatalf("inherited Visualforce extension action should not be reported")
	}
	if hasLineFinding(report, "visualforce.controller-test", "force-app/main/default/pages/SetNavigation.page", "allocations") ||
		hasLineFinding(report, "visualforce.controller-test", "force-app/main/default/pages/SetNavigation.page", "{!setCon.first}") {
		t.Fatalf("Visualforce recordSetVar and standard set controller navigation action should not be reported")
	}
	if hasLineFinding(report, "visualforce.controller-test", "force-app/main/default/pages/FormulaAction.page", "{!if(true, save, null)}") {
		t.Fatalf("Visualforce formula action attributes should not be reported as missing controller methods")
	}
	if hasLineFinding(report, "visualforce.controller-test", "force-app/main/default/pages/RowItemAction.page", "{!item.editItem}") {
		t.Fatalf("Visualforce row-item action expressions should not be reported as page controller methods")
	}
	if hasLineFinding(report, "visualforce.controller-test", "force-app/main/default/components/ActionInput.component", "{!actSupAction}") {
		t.Fatalf("resolved Visualforce action attribute should not be reported")
	}
	if hasLineFinding(report, "visualforce.controller-test", "force-app/main/default/components/SetNavigation.component", "{!setCon.next}") {
		t.Fatalf("Visualforce component standard set controller navigation action should not be reported")
	}
	if hasLineFindingContaining(report, "visualforce.component-test", "force-app/main/default/components/Indexed.component", "Indexed") {
		t.Fatalf("parseable Visualforce component metadata should be indexed, not reported as a component blocker")
	}
	if !hasLineFinding(report, "visualforce.controller-test", "force-app/main/default/pages/Controller.page", "Controller") {
		t.Fatalf("Visualforce controller attribute should still be reported")
	}
	if !hasLineFinding(report, "visualforce.controller-test", "force-app/main/default/pages/MissingAction.page", "{!missing}") {
		t.Fatalf("missing Visualforce controller action should still be reported")
	}
	if !hasLineFinding(report, "visualforce.controller-test", "force-app/main/default/pages/MissingCustomQuickSave.page", "{!quickSave}") {
		t.Fatalf("standard action names on custom-controller-only pages should still require a real controller method")
	}
}

func TestScanSuppressesSupportedVisualforceRuntimeReferences(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/AccountView.page"), `<apex:page standardController="Account" />`)
	writeFile(t, filepath.Join(root, "outside/pages/Loose.page"), `<apex:page />`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/pkg__Thing__c/pkg__Thing__c.object-meta.xml"), `<CustomObject><label>Thing</label><pluralLabel>Things</pluralLabel></CustomObject>`)
	writeFile(t, filepath.Join(root, "refs/znu/pages/znu__Order.page"), `<apex:page standardController="znu__Order__c" />`)
	writeFile(t, filepath.Join(root, "refs/pkg/pages/pkg__ManagedPage.page"), `<apex:page />`)
	writeFile(t, filepath.Join(root, "refs/ext/pages/ext__ManagedExtension.page"), `<apex:page standardController="Account" extensions="ext.ExternalController" />`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/UsesVisualforce.cls"), `public class UsesVisualforce {
  void run() {
    PageReference page = Page.AccountView;
    PageReference loose = Page.Loose;
    PageReference managedPage = Page.znu__Order;
    PageReference externalManagedPage = Page.pkg__ManagedPage;
    PageReference missingManagedPage = Page.pkg__MissingManagedPage;
    ApexPages.currentPage().getParameters().put('id', '001000000000001AAA');
    ApexPages.StandardController controller = new ApexPages.StandardController(new Account(Name = 'Acme'));
    PageReference stringPage = new PageReference('Page.StringOnly');
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
	if hasLineFinding(report, "visualforce.controller-test", "force-app/main/default/classes/UsesVisualforce.cls", "Page.Loose") {
		t.Fatalf("Visualforce pages discovered outside package roots should be registered")
	}
	if hasLineFinding(report, "visualforce.controller-test", "force-app/main/default/classes/UsesVisualforce.cls", "Page.znu__Order") {
		t.Fatalf("registered managed Page.znu__Order reference should not be reported")
	}
	if hasLineFinding(report, "visualforce.controller-test", "force-app/main/default/classes/UsesVisualforce.cls", "Page.pkg__ManagedPage") {
		t.Fatalf("managed-package Page.* references should not be reported when page metadata is known")
	}
	if hasLineFinding(report, "visualforce.controller-test", "force-app/main/default/classes/UsesVisualforce.cls", "Page.StringOnly") {
		t.Fatalf("Page.* inside Apex strings should not be reported as page namespace references")
	}
	if hasLineFinding(report, "visualforce.controller-test", "refs/ext/pages/ext__ManagedExtension.page", "ext.ExternalController") {
		t.Fatalf("managed extension controller should resolve from Visualforce namespace metadata")
	}
	if hasLineFinding(report, "visualforce.controller-test", "force-app/main/default/classes/UsesVisualforce.cls", "Page.MissingPage") {
		t.Fatalf("missing Page.MissingPage metadata should not be reported as controller-test debt")
	}
	if !hasLineFinding(report, "visualforce.page-metadata", "force-app/main/default/classes/UsesVisualforce.cls", "Page.MissingPage") {
		t.Fatalf("missing unresolved Page.MissingPage metadata finding")
	}
	if !hasLineFinding(report, "visualforce.page-metadata", "force-app/main/default/classes/UsesVisualforce.cls", "Page.pkg__MissingManagedPage") {
		t.Fatalf("missing managed Page.* reference should remain a metadata blocker")
	}
	if hasLineFinding(report, "visualforce.controller-test", "force-app/main/default/pages/AccountView.page", "Account") {
		t.Fatalf("resolved Visualforce standard controller object should not be reported")
	}
}

func TestScanIgnoresClientControllerAttributesAndVisualforceComments(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/staticresources/Active.resource-meta.xml"), `<StaticResource><contentType>text/plain</contentType><cacheControl>Public</cacheControl></StaticResource>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/ClientOnly.page"), `<apex:page>
  <div ng-controller="ClientCtrl"></div>
  <!-- <apex:includeScript value="{!$Resource.MissingCommented}" /> -->
  <!--
  <apex:includeScript value="{!$Resource.AlsoMissing}" />
  -->
  <apex:includeScript value="{!$Resource.Active}" />
</apex:page>`)

	report, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}

	if hasLineFinding(report, "visualforce.controller-test", "force-app/main/default/pages/ClientOnly.page", "ClientCtrl") {
		t.Fatalf("client-side controller attributes should not be reported as Visualforce controller contracts")
	}
	if hasLineFinding(report, "staticresources.urlfor", "force-app/main/default/pages/ClientOnly.page", "MissingCommented") ||
		hasLineFinding(report, "staticresources.urlfor", "force-app/main/default/pages/ClientOnly.page", "AlsoMissing") {
		t.Fatalf("Visualforce references inside HTML comments should not be reported")
	}
	if hasLineFinding(report, "staticresources.urlfor", "force-app/main/default/pages/ClientOnly.page", "Active") {
		t.Fatalf("resolved active static resource should not be reported")
	}
}

func TestScanSuppressesPassiveAuraOutsidePackageRoots(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "loose/aura/passive/passive.cmp"), `<aura:component implements="force:hasRecordId">Passive</aura:component>`)
	writeFile(t, filepath.Join(root, "loose/aura/passive/passive.cmp-meta.xml"), `<AuraDefinitionBundle/>`)

	report, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}

	if hasLineFinding(report, "aura.controller-test", "loose/aura/passive/passive.cmp", "passive") ||
		hasLineFinding(report, "aura.controller-test", "loose/aura/passive/passive.cmp-meta.xml", "passive") {
		t.Fatalf("passive Aura bundles discovered outside package roots should not be reported")
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

func TestScanSuppressesResolvedAuraControllerBundles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace":"pkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/WidgetController.cls"), `public class WidgetController {
  @AuraEnabled public static String getWidget() { return 'widget'; }
  @AuraEnabled public static String saveWidget() { return 'saved'; }
}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/aura/Widget/Widget.cmp"), `<aura:component controller="pkg.WidgetController">
  <aura:handler name="init" value="{!this}" action="{!c.getWidget}" />
</aura:component>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/aura/Widget/WidgetController.js"), `({
  save: function(component) {
    component.get("c.saveWidget");
  }
})`)
	writeFile(t, filepath.Join(root, "force-app/main/default/aura/Missing/Missing.cmp"), `<aura:component controller="MissingController">
  <aura:handler name="init" value="{!this}" action="{!c.missing}" />
</aura:component>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/aura/ClientOnly/ClientOnly.cmp"), `<aura:component controller="WidgetController">
  <aura:handler name="init" value="{!this}" action="{!c.doInit}" />
  <lightning:button onclick="{!c.handleClick}" />
</aura:component>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/aura/ClientOnly/ClientOnlyController.js"), `({
  doInit: function(component, event, helper) {},
  handleClick: function(component, event, helper) {}
})`)
	writeFile(t, filepath.Join(root, "force-app/main/default/aura/MissingServerAction/MissingServerAction.cmp"), `<aura:component controller="WidgetController" />`)
	writeFile(t, filepath.Join(root, "force-app/main/default/aura/MissingServerAction/MissingServerActionHelper.js"), `({
  load: function(localCmp) {
    localCmp.get("c.missingServer");
  }
})`)
	writeFile(t, filepath.Join(root, "force-app/main/default/lwc/widget/widget.js"), `import saveWidget from '@salesforce/apex/pkg.WidgetController.saveWidget';
import missing from '@salesforce/apex/pkg.WidgetController.missing';
`)

	report, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}

	if hasFindingContaining(report, "aura.controller-test", "force-app/main/default/aura/Widget/Widget.cmp", "Widget") ||
		hasFindingContaining(report, "aura.controller-test", "force-app/main/default/aura/Widget/WidgetController.js", "Widget") {
		t.Fatalf("resolved Aura bundle files should not be reported: %#v", report.Findings)
	}
	if !hasFindingContaining(report, "aura.controller-test", "force-app/main/default/aura/Missing/Missing.cmp", "Missing") {
		t.Fatalf("missing unresolved Aura bundle finding")
	}
	if hasFindingContaining(report, "aura.controller-test", "force-app/main/default/aura/ClientOnly/ClientOnly.cmp", "ClientOnly") ||
		hasFindingContaining(report, "aura.controller-test", "force-app/main/default/aura/ClientOnly/ClientOnlyController.js", "ClientOnly") {
		t.Fatalf("client-only Aura actions should not be reported as Apex blockers: %#v", report.Findings)
	}
	if hasFindingContaining(report, "aura.controller-test", "force-app/main/default/aura/MissingServerAction/MissingServerActionHelper.js", "MissingServerAction") {
		t.Fatalf("missing Aura server action should not be reported as controller-test debt")
	}
	if !hasFindingContaining(report, "aura.action-metadata", "force-app/main/default/aura/MissingServerAction/MissingServerActionHelper.js", "MissingServerAction") {
		t.Fatalf("missing unresolved Aura server action metadata finding")
	}
	if hasLineFinding(report, "lwc.controller-test", "force-app/main/default/lwc/widget/widget.js", "pkg.WidgetController.saveWidget") {
		t.Fatalf("resolved namespaced LWC Apex import should not be reported")
	}
	if !hasLineFinding(report, "lwc.controller-test", "force-app/main/default/lwc/widget/widget.js", "pkg.WidgetController.missing") {
		t.Fatalf("missing unresolved namespaced LWC Apex import finding")
	}
}

func TestScanSuppressesResolvedAuraControllerBundlesWithoutSFDXProject(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "refs/pkg/classes/LegacyAuraController.cls"), `public class LegacyAuraController {
  @AuraEnabled
  public static String load() { return 'ok'; }
}`)
	writeFile(t, filepath.Join(root, "refs/pkg/aura/Legacy/Legacy.cmp"), `<aura:component controller="LegacyAuraController">
  <aura:handler name="init" value="{!this}" action="{!c.load}" />
</aura:component>`)
	writeFile(t, filepath.Join(root, "refs/pkg/aura/Legacy/LegacyController.js"), `({
  load: function(component) {
    component.get("c.load");
  }
})`)
	writeFile(t, filepath.Join(root, "refs/pkg/aura/Legacy/Legacy.cmp-meta.xml"), `<AuraDefinitionBundle/>`)
	writeFile(t, filepath.Join(root, "refs/pkg/aura/Legacy/Legacy.css"), `.THIS {}`)
	writeFile(t, filepath.Join(root, "refs/pkg/aura/Legacy/Legacy.svg"), `<svg/>`)

	report, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if findSurface(report, "aura.controller-test") != nil {
		t.Fatalf("resolved Aura bundle under non-SFDX root should not be controller-test debt: %#v", report.Findings)
	}
}

func TestResolvedAuraFilesUsesApexMetadataMethodFallback(t *testing.T) {
	root := t.TempDir()
	classPath := filepath.Join(root, "classes/LegacyController.cls")
	cmpPath := filepath.Join(root, "aura/Legacy/Legacy.cmp")
	writeFile(t, classPath, `global class LegacyController {
  @AuraEnabled
  global static String load() { return 'ok'; }
}`)
	ctx := scanContext{
		apexMetadataFiles: map[string][]string{
			"legacycontroller": {classPath},
		},
	}
	ui := uicontroller.Index{AuraBundles: []uicontroller.AuraBundle{{
		Name: "Legacy",
		Dir:  filepath.Dir(cmpPath),
		Files: []uicontroller.UIFile{{
			Path: cmpPath,
			Kind: "component",
		}},
		ActionReferences: []uicontroller.AuraActionReference{{
			Name:      "load",
			ClassName: "LegacyController",
			Resolved:  false,
		}},
	}}}

	resolved, actionMetadata := resolvedAuraFiles(ui, &ctx)
	if !resolved[filepath.Clean(cmpPath)] || !resolved[filepath.Clean(filepath.Dir(cmpPath))] {
		t.Fatalf("Aura files should resolve from Apex metadata method fallback: %#v", resolved)
	}
	if actionMetadata[filepath.Clean(cmpPath)] || actionMetadata[filepath.Clean(filepath.Dir(cmpPath))] {
		t.Fatalf("resolved metadata fallback should not report missing action metadata: %#v", actionMetadata)
	}
}

func TestApexMetadataAuraMethodFallbackRequiresAuraEnabledStatic(t *testing.T) {
	root := t.TempDir()
	classPath := filepath.Join(root, "classes/LegacyController.cls")
	writeFile(t, classPath, `global class LegacyController {
  public static String staticOnly() { return 'no'; }
  @AuraEnabled public String instanceOnly() { return 'no'; }
  @AuraEnabled private static String privateOnly() { return 'no'; }
  @AuraEnabled
  global static String callable() { return 'ok'; }
}`)

	if !apexFileDeclaresAuraMethod(classPath, "callable") {
		t.Fatalf("expected @AuraEnabled global static method to resolve")
	}
	for _, method := range []string{"staticOnly", "instanceOnly", "privateOnly"} {
		if apexFileDeclaresAuraMethod(classPath, method) {
			t.Fatalf("%s should not resolve as an Aura server action", method)
		}
	}
}

func TestScanDoesNotClassifyPassiveAuraArtifactsAsControllerBlockers(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/aura/dataProviderInterface/dataProviderInterface.intf"), `<aura:interface />`)
	writeFile(t, filepath.Join(root, "force-app/main/default/aura/dataProviderInterface/dataProviderInterface.intf-meta.xml"), `<AuraDefinitionBundle/>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/aura/defaultTokens/defaultTokens.tokens"), `<aura:tokens/>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/aura/defaultTokens/defaultTokens.tokens-meta.xml"), `<AuraDefinitionBundle/>`)

	report, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if findSurface(report, "aura.controller-test") != nil {
		t.Fatalf("passive Aura artifacts should not be controller-test blockers: %#v", report.Findings)
	}
}

func TestScanSuppressesSupportedFileObjects(t *testing.T) {
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
		hasLineFinding(report, "files.binary-content", "force-app/main/default/classes/UsesBinary.cls", "base64Decode") ||
		hasLineFinding(report, "files.binary-content", "force-app/main/default/classes/UsesBinary.cls", "ContentVersion") ||
		hasLineFinding(report, "files.binary-content", "force-app/main/default/classes/UsesBinary.cls", "Attachment") {
		t.Fatalf("supported file and Blob runtime shapes should not be file side-effect blockers")
	}
}

func TestScanSuppressesResolvedCustomMetadataTypeReferences(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Feature__mdt/Feature__mdt.object-meta.xml"), `<CustomObject><label>Feature</label><pluralLabel>Features</pluralLabel></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/customMetadata/pkg__RecordBacked.Default.md-meta.xml"), `<CustomMetadata>
  <label>Default</label>
  <values><field>pkg__Enabled__c</field><value>true</value></values>
</CustomMetadata>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/UsesMetadata.cls"), `public class UsesMetadata {
  Feature__mdt configured;
  pkg__Feature__mdt namespaced;
  pkg__RecordBacked__mdt recordBacked;
  ext__ManagedOnly__mdt externalManaged;
  Missing__mdt missing;
  /*
   * BlockCommentOnly__mdt should not count.
   */
  /*
MissingInCodeFence__mdt should not count.
System.debug(System.Label.BlockCommentLabel);
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
	if hasLineFinding(report, "custommetadata.legacy-records", "force-app/main/default/classes/UsesMetadata.cls", "pkg__RecordBacked__mdt") {
		t.Fatalf("record-backed custom metadata type reference should not be reported")
	}
	if hasLineFinding(report, "custommetadata.legacy-records", "force-app/main/default/classes/UsesMetadata.cls", "ext__ManagedOnly__mdt") {
		t.Fatalf("external managed custom metadata type reference should not be reported")
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
	if hasLineFinding(report, "custommetadata.legacy-records", "force-app/main/default/classes/UsesMetadata.cls", "MissingInCodeFence__mdt") {
		t.Fatalf("unstarred block-comment custom metadata mention should not be reported")
	}
	if hasLineFinding(report, "labels.localization", "force-app/main/default/classes/UsesMetadata.cls", "BlockCommentLabel") {
		t.Fatalf("unstarred block-comment label mention should not be reported")
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
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace":"orgns","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/labels/CustomLabels.labels"), `<CustomLabels>
  <labels><fullName>Save</fullName><value>Save</value></labels>
  <labels><fullName>Greeting</fullName><value>Hello</value></labels>
  <labels><fullName>Remove</fullName><value>Remove</value></labels>
  <labels><fullName>pkg__Managed</fullName><value>Managed</value></labels>
  <labels><fullName>AddressCopyUnknownObject</fullName><value>Unknown address object</value></labels>
  <labels><fullName>Contact_Merge_Error_Too_Few_Contacts</fullName><value>Too few contacts</value></labels>
</CustomLabels>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/pkg1__ManagedPayment__c/pkg1__ManagedPayment__c.object-meta.xml"), `<CustomObject><label>Payment</label><pluralLabel>Payments</pluralLabel></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/npo02__Address__c/npo02__Address__c.object-meta.xml"), `<CustomObject><label>Address</label><pluralLabel>Addresses</pluralLabel></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/pkg__Managed__c/pkg__Managed__c.object-meta.xml"), `<CustomObject><label>Managed</label><pluralLabel>Managed</pluralLabel></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/ext__External__c/ext__External__c.object-meta.xml"), `<CustomObject><label>External</label><pluralLabel>Externals</pluralLabel></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/translations/fr.translation-meta.xml"), `<Translations>
  <customLabels><name>Greeting</name><label>Bonjour</label></customLabels>
</Translations>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/UsesLabels.cls"), `public class UsesLabels {
  void run() {
    System.debug(System.Label.Save);
    System.debug(Label.Greeting);
    System.debug(Label.Save.replace('{0}', 'Done'));
    System.debug(Label.pkg.Managed);
    System.debug(option.Label.compareTo(other.Label));
    System.debug(wrapper.Label.get('Name'));
    System.debug(System.Label.npo02.AddressCopyUnknownObject);
    System.debug(Label.pkg1.Contact_Merge_Error_Too_Few_Contacts);
    System.debug(Label.ext.Managed_Dependency_Label);
    System.debug(Label.orgns.Own_Namespace_Missing);
    System.debug(System.Label.get('ext', 'Managed_Dynamic_Label', 'en_US'));
    String formula = '$Label.c.MyLabelName';
    System.debug(Label.Site.invalid_email);
    System.debug(Label.pkg1.Missing_Aliased_Label);
    System.debug(Label.Missing);
    System.debug(Label.Missing.replace('{0}', 'Done'));
  }
}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/lwc/labels/labels.js"), `import SAVE from '@salesforce/label/c.Save';
import MISSING from '@salesforce/label/c.Missing';
import MANAGED from '@salesforce/label/pkg.Managed';
import REMOVE from '@salesforce/label/c.Remove';
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/Labels.page"), `<apex:page>
{!$Label.Save}
{!$Label.Missing}
{!$Label.ext__External_Visualforce_Label}
{!$Label.site.site_login}
{!$Label.orgns__Own_Visualforce_Missing}
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
	if hasLineFindingContaining(report, "labels.localization", "force-app/main/default/classes/UsesLabels.cls", "Save.replace") {
		t.Fatalf("resolved label String method chain should not be reported")
	}
	if hasLineFindingContaining(report, "labels.localization", "force-app/main/default/classes/UsesLabels.cls", "Label.compareTo") {
		t.Fatalf("ordinary .Label field method chain should not be reported")
	}
	if hasLineFindingContaining(report, "labels.localization", "force-app/main/default/classes/UsesLabels.cls", "Label.get") {
		t.Fatalf("ordinary .Label field map access should not be reported")
	}
	if hasLineFinding(report, "labels.localization", "force-app/main/default/classes/UsesLabels.cls", "pkg.Managed") {
		t.Fatalf("resolved managed-package label fallback should not be reported")
	}
	if hasLineFinding(report, "labels.localization", "force-app/main/default/classes/UsesLabels.cls", "npo02.AddressCopyUnknownObject") {
		t.Fatalf("resolved aliased System.Label namespace should not be reported")
	}
	if hasLineFinding(report, "labels.localization", "force-app/main/default/classes/UsesLabels.cls", "pkg1.Contact_Merge_Error_Too_Few_Contacts") {
		t.Fatalf("resolved aliased Label namespace should not be reported")
	}
	if hasLineFinding(report, "labels.localization", "force-app/main/default/classes/UsesLabels.cls", "ext.Managed_Dependency_Label") {
		t.Fatalf("external managed-package label fallback should not be reported")
	}
	if hasLineFinding(report, "labels.localization", "force-app/main/default/classes/UsesLabels.cls", "get") {
		t.Fatalf("System.Label.get should not be reported as a missing label")
	}
	if hasLineFindingContaining(report, "labels.localization", "force-app/main/default/classes/UsesLabels.cls", "c.MyLabelName") {
		t.Fatalf("label-like Apex string literals should not be reported")
	}
	if hasLineFinding(report, "labels.localization", "force-app/main/default/classes/UsesLabels.cls", "Site.invalid_email") {
		t.Fatalf("platform Site label fallback should not be reported")
	}
	if hasLineFindingContaining(report, "labels.localization", "force-app/main/default/lwc/labels/labels.js", "c.Save") {
		t.Fatalf("resolved LWC c.Save label should not be reported")
	}
	if hasLineFindingContaining(report, "labels.localization", "force-app/main/default/lwc/labels/labels.js", "pkg.Managed") {
		t.Fatalf("resolved LWC managed-package label fallback should not be reported")
	}
	if hasLineFindingContaining(report, "labels.localization", "force-app/main/default/lwc/labels/labels.js", "c.Remove") {
		t.Fatalf("resolved LWC label named like a String method should not be reported")
	}
	if hasLineFindingContaining(report, "labels.localization", "force-app/main/default/pages/Labels.page", "$Label.Save") {
		t.Fatalf("resolved Visualforce $Label.Save should not be reported")
	}
	if hasLineFindingContaining(report, "labels.localization", "force-app/main/default/pages/Labels.page", "$Label.ext__External_Visualforce_Label") {
		t.Fatalf("external Visualforce managed-package label fallback should not be reported")
	}
	if hasLineFindingContaining(report, "labels.localization", "force-app/main/default/pages/Labels.page", "$Label.site.site_login") {
		t.Fatalf("platform Visualforce Site label fallback should not be reported")
	}
	if !hasLineFinding(report, "labels.localization", "force-app/main/default/classes/UsesLabels.cls", "Missing") {
		t.Fatalf("missing unresolved Apex label finding")
	}
	if hasLineFinding(report, "labels.localization", "force-app/main/default/classes/UsesLabels.cls", "pkg1.Missing_Aliased_Label") {
		t.Fatalf("external managed-package missing label fallback should not be reported")
	}
	if !hasLineFinding(report, "labels.localization", "force-app/main/default/classes/UsesLabels.cls", "orgns.Own_Namespace_Missing") {
		t.Fatalf("missing own-namespace label finding")
	}
	if !hasLineFindingContaining(report, "labels.localization", "force-app/main/default/classes/UsesLabels.cls", "Missing.replace") {
		t.Fatalf("missing unresolved Apex label method-chain finding")
	}
	if !hasLineFindingContaining(report, "labels.localization", "force-app/main/default/lwc/labels/labels.js", "c.Missing") {
		t.Fatalf("missing unresolved LWC label finding")
	}
	if !hasLineFindingContaining(report, "labels.localization", "force-app/main/default/pages/Labels.page", "$Label.Missing") {
		t.Fatalf("missing unresolved Visualforce label finding")
	}
	if !hasLineFindingContaining(report, "labels.localization", "force-app/main/default/pages/Labels.page", "$Label.orgns__Own_Visualforce_Missing") {
		t.Fatalf("missing own-namespace Visualforce label finding")
	}
	for _, finding := range report.Findings {
		if finding.MetadataType == "CustomLabels" {
			t.Fatalf("label metadata files should not be reported as unsupported: %#v", finding)
		}
	}
}

func TestScanSuppressesModeledDeclarativeAutomation(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Widget__c/Widget__c.object-meta.xml"), `<CustomObject><label>Widget</label><pluralLabel>Widgets</pluralLabel><fields><fullName>Status__c</fullName><label>Status</label><type>Text</type><length>40</length></fields></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/workflows/Widget__c.workflow-meta.xml"), `<Workflow>
  <fieldUpdates><fullName>SetStatus</fullName><field>Status__c</field><literalValue>Workflow</literalValue></fieldUpdates>
  <rules><fullName>Mark</fullName><active>true</active><actions><name>SetStatus</name><type>FieldUpdate</type></actions></rules>
</Workflow>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/workflows/Legacy__c.workflow"), `<Workflow>
  <fieldUpdates><fullName>SetLegacyStatus</fullName><field>Status__c</field><literalValue>Workflow</literalValue></fieldUpdates>
  <rules><fullName>LegacyMark</fullName><active>true</active><booleanFilter>1</booleanFilter><criteriaItems><field>Legacy__c.Status__c</field><operation>equals</operation></criteriaItems><actions><name>SetLegacyStatus</name><type>FieldUpdate</type></actions></rules>
</Workflow>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/workflows/Contact.workflow"), `<Workflow>
  <rules><fullName>CopyEmail</fullName><active>true</active><actions><name>ContactEmailUpdate</name><type>FieldUpdate</type></actions></rules>
</Workflow>`)
	writeFile(t, filepath.Join(root, "unpackaged/config/trial_tso/workflows/Contact.workflow"), `<Workflow>
  <fieldUpdates><fullName>ContactEmailUpdate</fullName><field>OtherEmail__c</field><formula>Email</formula></fieldUpdates>
</Workflow>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/flows/Widget_Status.flow-meta.xml"), `<Flow>
  <processType>Workflow</processType>
  <status>Active</status>
  <start><object>Widget__c</object></start>
  <assignments><name>SetFlow</name><assignmentItems><assignToReference>$Record.Status__c</assignToReference><operator>Assign</operator><value><stringValue>Flow</stringValue></value></assignmentItems></assignments>
</Flow>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/flows/Widget_Propagate_Delete.flow-meta.xml"), `<Flow>
  <processType>AutoLaunchedFlow</processType>
  <status>Active</status>
  <decisions><name>ExistingRequest</name><defaultConnector><targetReference>CreateRequest</targetReference></defaultConnector><rules><name>Exists</name><conditionLogic>and</conditionLogic><conditions><leftValueReference>PendingRequest</leftValueReference><operator>IsNull</operator><rightValue><booleanValue>false</booleanValue></rightValue></conditions></rules></decisions>
  <recordLookups><name>PendingRequest</name><object>ActionRequest__c</object><filterLogic>and</filterLogic><filters><field>SourceRecordId__c</field><operator>EqualTo</operator><value><elementReference>$Record.Id</elementReference></value></filters><getFirstRecordOnly>true</getFirstRecordOnly><storeOutputAutomatically>true</storeOutputAutomatically></recordLookups>
  <recordCreates><name>CreateRequest</name><object>ActionRequest__c</object><inputAssignments><field>ActionName__c</field><value><stringValue>Delete</stringValue></value></inputAssignments><inputAssignments><field>SourceRecordId__c</field><value><elementReference>$Record.Id</elementReference></value></inputAssignments><storeOutputAutomatically>true</storeOutputAutomatically></recordCreates>
  <start><object>Widget__c</object><triggerType>RecordBeforeDelete</triggerType></start>
  <variables><name>PendingRequest</name><dataType>SObject</dataType><objectType>ActionRequest__c</objectType></variables>
</Flow>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/flows/SetupWizard.flow-meta.xml"), `<Flow>
  <processType>Flow</processType>
  <status>Active</status>
  <screens><name>Wizard</name></screens>
  <recordLookups><name>Pick_Default</name></recordLookups>
</Flow>`)

	report, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if findSurface(report, "workflow.save-order") != nil {
		t.Fatalf("modeled workflow field update should not be reported: %#v", report.Findings)
	}
	if findSurface(report, "flow.save-order") != nil {
		t.Fatalf("modeled flow assignment should not be reported: %#v", report.Findings)
	}
}

func TestScanSuppressesResolvedResourcesAndEndpoints(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/staticresources/Site.resource"), "body")
	writeFile(t, filepath.Join(root, "force-app/main/default/staticresources/Site.resource-meta.xml"), `<StaticResource><contentType>text/plain</contentType></StaticResource>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/contentassets/Logo.asset"), "body")
	writeFile(t, filepath.Join(root, "force-app/main/default/contentassets/Logo.asset-meta.xml"), `<ContentAsset><contentType>image/png</contentType></ContentAsset>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/email/Welcome.email"), "Hello")
	writeFile(t, filepath.Join(root, "force-app/main/default/email/Welcome.email-meta.xml"), `<EmailTemplate><fullName>unfiled$public/Welcome</fullName><subject>Hello</subject><available>true</available></EmailTemplate>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/namedCredentials/Billing.namedCredential"), `<NamedCredential><endpoint>https://billing.example.test</endpoint></NamedCredential>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/remoteSiteSettings/Maps.remoteSite"), `<RemoteSiteSetting><url>https://maps.example.test</url></RemoteSiteSetting>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/lwc/resources/resources.js"), `import SITE from '@salesforce/resourceUrl/Site';
import MISSING from '@salesforce/resourceUrl/MissingResource';
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/Resources.page"), `<apex:page>
{!URLFOR($Resource.Site, 'css/app.css')}
{!$Resource.Logo}
<!-- {!URLFOR($Resource.MissingCommentResource, 'css/app.css')} -->
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
	if hasLineFinding(report, "staticresources.urlfor", "force-app/main/default/pages/Resources.page", "MissingCommentResource") {
		t.Fatalf("Visualforce HTML comments should not produce resource findings")
	}
	if hasLineFinding(report, "endpoint.metadata", "force-app/main/default/classes/UsesEndpoint.cls", "Billing") {
		t.Fatalf("resolved named credential callout should not be reported")
	}
	if !hasLineFinding(report, "endpoint.metadata", "force-app/main/default/classes/UsesEndpoint.cls", "MissingEndpoint") {
		t.Fatalf("missing unresolved named credential callout finding")
	}
	for _, finding := range report.Findings {
		switch finding.MetadataType {
		case "StaticResource", "ContentAsset", "EmailTemplate", "NamedCredential", "RemoteSiteSetting":
			t.Fatalf("loaded metadata files should not be reported as unsupported: %#v", finding)
		}
	}
}

func TestScanCountsReportAndDashboardMetadata(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/reports/Sales/Pipeline.report-meta.xml"), `<Report>
  <name>Pipeline</name>
  <format>Summary</format>
</Report>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/reports/Legacy/Bookings.report"), `<Report>
  <name>Bookings</name>
</Report>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/dashboards/Executive/Sales.dashboard-meta.xml"), `<Dashboard>
  <title>Sales</title>
</Dashboard>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/dashboards/Legacy/Support.dashboard"), `<Dashboard>
  <title>Support</title>
</Dashboard>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/RunReports.cls"), `public class RunReports {
  void run() {
    Reports.ReportManager.runReport('00O000000000001');
    Analytics.ExternalDataSourceConnection conn;
    VerificationResult result = new VerificationResult();
    Boolean empty = result.Reports != null && !result.Reports.isEmpty();
  }
  private class VerificationResult {
    List<String> Reports;
  }
}`)

	report, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.Reports != 2 {
		t.Fatalf("reports = %d, want 2", report.Summary.Reports)
	}
	if report.Summary.Dashboards != 2 {
		t.Fatalf("dashboards = %d, want 2", report.Summary.Dashboards)
	}
	if !hasLineFindingContaining(report, "analytics.report-execution", "force-app/main/default/classes/RunReports.cls", "Reports.ReportManager") {
		t.Fatalf("missing unsupported Reports execution finding")
	}
	if !hasLineFindingContaining(report, "analytics.report-execution", "force-app/main/default/classes/RunReports.cls", "Analytics.ExternalDataSourceConnection") {
		t.Fatalf("missing unsupported Analytics execution finding")
	}
	if hasLineFindingContaining(report, "analytics.report-execution", "force-app/main/default/classes/RunReports.cls", "Reports.isEmpty") {
		t.Fatalf("instance property named Reports should not be reported as namespace execution")
	}
	for _, finding := range report.Findings {
		if strings.Contains(finding.File, "/reports/") || strings.Contains(finding.File, "/dashboards/") {
			t.Fatalf("report/dashboard metadata should be accounted for without load blockers: %#v", finding)
		}
	}
}

func TestScanUsesMetadataOutsideConfiguredPackageDirectories(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace":"namz","packageDirectories":[{"path":"core","default":true}]}`)
	writeFile(t, filepath.Join(root, "core/customMetadata/pkg__SOQLQuery.Default.md-meta.xml"), `<CustomMetadata>
  <label>Default Query</label>
</CustomMetadata>`)
	writeFile(t, filepath.Join(root, "core/classes/UsesSupplementalMetadata.cls"), `public class UsesSupplementalMetadata {
  pkg__SOQLQuery__mdt query;
  void run() {
    HttpRequest req = new HttpRequest();
    req.setEndpoint('callout:OPEX__OpenExchangeRates/latest.json');
  }
}`)
	writeFile(t, filepath.Join(root, "core/lwc/payment/payment.js"), `import paymentOptionsIcon from '@salesforce/resourceUrl/pkg__PaymentOptions';`)
	writeFile(t, filepath.Join(root, "extras/namedCredentials/OpenExchangeRates.namedCredential-meta.xml"), `<NamedCredential>
  <endpoint>https://openexchangerates.example.test</endpoint>
  <protocol>NoAuthentication</protocol>
</NamedCredential>`)

	report, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}

	if hasLineFinding(report, "custommetadata.legacy-records", "core/classes/UsesSupplementalMetadata.cls", "pkg__SOQLQuery__mdt") {
		t.Fatalf("record-backed namespaced custom metadata type should not be reported")
	}
	if hasLineFinding(report, "endpoint.metadata", "core/classes/UsesSupplementalMetadata.cls", "OPEX__OpenExchangeRates") {
		t.Fatalf("named credential outside configured package directories should be available")
	}
	if hasLineFinding(report, "staticresources.urlfor", "core/lwc/payment/payment.js", "pkg__PaymentOptions") {
		t.Fatalf("external managed-package static resource reference should not be reported")
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

func hasFindingContaining(report Report, capability, file, symbol string) bool {
	for _, finding := range report.Findings {
		if finding.Capability == capability && finding.File == file && strings.Contains(finding.Symbol, symbol) {
			return true
		}
	}
	return false
}

func hasLineFindingContaining(report Report, capability, file, symbol string) bool {
	for _, finding := range report.Findings {
		if finding.Capability == capability && finding.File == file && strings.Contains(finding.Symbol, symbol) {
			return true
		}
	}
	return false
}

func hasLineFindingEvidenceContaining(report Report, capability, file, evidence string) bool {
	for _, finding := range report.Findings {
		if finding.Capability == capability && finding.File == file && finding.Line > 0 && strings.Contains(finding.Evidence, evidence) {
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
