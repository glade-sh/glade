package capability

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestMVPReportIsReadyWhenRequiredFeaturesAreSupported(t *testing.T) {
	report := MVPReport()
	if !report.Ready {
		t.Fatalf("MVP report should be ready once required features are supported: %#v", report)
	}
	if report.Required == 0 || report.Incomplete != 0 || report.Complete != report.Required {
		t.Fatalf("report = %#v", report)
	}
	for _, feature := range report.Features {
		if feature.Required && feature.Status != StatusSupported {
			t.Fatalf("%s status = %s, want %s", feature.ID, feature.Status, StatusSupported)
		}
	}
}

func TestMVPReportIncludesStatusCounts(t *testing.T) {
	report := MVPReport()
	total := 0
	for _, status := range []Status{StatusSupported, StatusPartial, StatusStub, StatusUnsupported, StatusUnknown} {
		total += report.StatusCounts[status]
	}
	if total != report.Total {
		t.Fatalf("status count total = %d, want %d", total, report.Total)
	}
	if report.StatusCounts[StatusSupported] != report.Complete {
		t.Fatalf("supported count = %d, want complete count %d", report.StatusCounts[StatusSupported], report.Complete)
	}
}

func TestLocalMVPDXRowsAreSupportedWithPostMVPTails(t *testing.T) {
	report := MVPReport()
	features := map[string]Feature{}
	for _, feature := range report.Features {
		features[feature.ID] = feature
	}

	postMVP := map[string]string{
		"dap.command":      "dap.live-ide-orchestration",
		"lsp.command":      "lsp.context-completion",
		"profile.native":   "profile.pprof-and-timing",
		"watch.command":    "watch.profile-trace-reports",
		"server.local-api": "server.rest-breadth",
	}
	for id, postID := range postMVP {
		feature, ok := features[id]
		if !ok {
			t.Fatalf("missing local MVP feature %s", id)
		}
		if !feature.Required || feature.Status != StatusSupported {
			t.Fatalf("%s = required %v status %s, want required supported", id, feature.Required, feature.Status)
		}
		if !strings.Contains(feature.Notes, "Local MVP") {
			t.Fatalf("%s notes do not describe the local MVP contract: %s", id, feature.Notes)
		}

		tail, ok := features[postID]
		if !ok {
			t.Fatalf("missing post-MVP feature %s for %s", postID, id)
		}
		if tail.Required || tail.Status != StatusPartial {
			t.Fatalf("%s = required %v status %s, want optional partial", postID, tail.Required, tail.Status)
		}
		if !strings.Contains(tail.Notes, "Post-MVP") {
			t.Fatalf("%s notes do not identify the post-MVP contract: %s", postID, tail.Notes)
		}
	}
}

func TestCoreRuntimeMVPFeaturesAreSupportedWithPostMVPTails(t *testing.T) {
	report := MVPReport()
	features := map[string]Feature{}
	for _, feature := range report.Features {
		features[feature.ID] = feature
	}

	postMVP := map[string]string{
		"limits.core": "limits.exact-accounting",
		"stdlib.core": "stdlib.platform-breadth",
	}
	for id, postID := range postMVP {
		feature, ok := features[id]
		if !ok {
			t.Fatalf("missing core runtime MVP feature %s", id)
		}
		if !feature.Required || feature.Status != StatusSupported {
			t.Fatalf("%s = required %v status %s, want required supported", id, feature.Required, feature.Status)
		}
		notes := strings.ToLower(feature.Notes)
		for _, blocked := range []string{"incomplete", "broader", "exact salesforce accounting", "unimplemented platform"} {
			if strings.Contains(notes, blocked) {
				t.Fatalf("%s supported notes still carry open gap language %q: %s", id, blocked, feature.Notes)
			}
		}

		tail, ok := features[postID]
		if !ok {
			t.Fatalf("missing post-MVP feature %s for %s", postID, id)
		}
		if tail.Required || tail.Status != StatusPartial {
			t.Fatalf("%s = required %v status %s, want optional partial", postID, tail.Required, tail.Status)
		}
		if !strings.Contains(tail.Notes, "Post-MVP") {
			t.Fatalf("%s notes do not identify the post-MVP contract: %s", postID, tail.Notes)
		}
	}
}

func TestWriteJSON(t *testing.T) {
	var out bytes.Buffer
	if err := WriteJSON(&out, MVPReport()); err != nil {
		t.Fatal(err)
	}
	var decoded Report
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Target != "full-featured glade-parity MVP" {
		t.Fatalf("target = %q", decoded.Target)
	}
	if decoded.StatusCounts[StatusSupported] != decoded.Complete {
		t.Fatalf("supported count = %d, want complete count %d", decoded.StatusCounts[StatusSupported], decoded.Complete)
	}
}

func TestWriteText(t *testing.T) {
	var out bytes.Buffer
	if err := WriteText(&out, MVPReport()); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "MVP readiness: ready") || !strings.Contains(text, "Required complete: 21/21") {
		t.Fatalf("text output = %q", text)
	}
}

func TestWriteMarkdown(t *testing.T) {
	var out bytes.Buffer
	if err := WriteMarkdown(&out, MVPReport()); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{
		"# Compatibility Dashboard",
		"Generated from `internal/capability`.",
		"Required complete:",
		"| Area | ID | Status | Capability | Notes |",
		"`triggers.runtime`",
		"## Tracked Post-MVP Capabilities",
		"`server.rest-breadth`",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("markdown output missing %q: %q", want, text)
		}
	}
}

func TestWriteKnownGapsMarkdown(t *testing.T) {
	var out bytes.Buffer
	if err := WriteKnownGapsMarkdown(&out, MVPReport()); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{
		"# Known Gaps",
		"Generated from `internal/capability`.",
		"All required capabilities are currently `supported`.",
		"No required MVP capability gaps are currently tracked.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("known gaps output missing %q: %q", want, text)
		}
	}
}

func TestLocalApexExecutionMVPFeaturesAreSupported(t *testing.T) {
	targets := map[string]bool{
		"apex.parser.project-scale": true,
		"apex.sema.body":            true,
		"dml.apex":                  true,
		"fixtures.persistence":      true,
		"sobject.apex":              true,
		"soql.apex":                 true,
		"triggers.runtime":          true,
	}
	for _, feature := range MVPFeatures() {
		if !targets[feature.ID] {
			continue
		}
		delete(targets, feature.ID)
		if feature.Status != StatusSupported {
			t.Fatalf("%s status = %s, want %s", feature.ID, feature.Status, StatusSupported)
		}
		notes := strings.ToLower(feature.Notes)
		for _, blocked := range []string{"remain incomplete", "remains incomplete", "full ", "broader "} {
			if strings.Contains(notes, blocked) {
				t.Fatalf("%s supported notes still carry open gap language %q: %s", feature.ID, blocked, feature.Notes)
			}
		}
	}
	for id := range targets {
		t.Fatalf("missing local Apex execution MVP feature %s", id)
	}
}

func TestCoreRuntimeControlFlowCapabilityIsSupported(t *testing.T) {
	for _, feature := range MVPFeatures() {
		if feature.ID != "vm.control-flow" {
			continue
		}
		if feature.Status != StatusSupported {
			t.Fatalf("%s status = %s, want %s", feature.ID, feature.Status, StatusSupported)
		}
		notes := strings.ToLower(feature.Notes)
		for _, blocked := range []string{"remaining gap", "remain incomplete", "remains incomplete", "outside control-flow"} {
			if strings.Contains(notes, blocked) {
				t.Fatalf("%s supported notes still carry open gap language %q: %s", feature.ID, blocked, feature.Notes)
			}
		}
		return
	}
	t.Fatal("missing vm.control-flow MVP feature")
}

func TestDatabaseStdlibRowsAreLocallyPromotedOrFenced(t *testing.T) {
	for _, entry := range StdlibMatrix() {
		if entry.Area != "Database" {
			continue
		}
		if entry.Status == StatusPartial {
			t.Fatalf("Database stdlib row %s remains partial: %s", entry.API, entry.Notes)
		}
		if entry.Status == StatusSupported && entry.Notes == "" {
			t.Fatalf("Database stdlib row %s needs local-model notes", entry.API)
		}
	}
}

func TestDatetimeArithmeticStdlibRowsAreSupported(t *testing.T) {
	watched := map[string]bool{
		"Datetime.addHours":   true,
		"Datetime.addMinutes": true,
		"Datetime.addSeconds": true,
	}
	for _, entry := range StdlibMatrix() {
		if !watched[entry.API] {
			continue
		}
		delete(watched, entry.API)
		if entry.Status != StatusSupported {
			t.Fatalf("%s status = %s, want %s", entry.API, entry.Status, StatusSupported)
		}
		if !strings.Contains(entry.Notes, "UTC-local") {
			t.Fatalf("%s notes do not describe local datetime arithmetic: %s", entry.API, entry.Notes)
		}
	}
	for api := range watched {
		t.Fatalf("missing stdlib row %s", api)
	}
}

func TestDeterministicURLStdlibRowsAreSupported(t *testing.T) {
	watched := map[string]bool{
		"URL.getOrgDomainUrl":      true,
		"URL.getSalesforceBaseUrl": true,
	}
	for _, entry := range StdlibMatrix() {
		if !watched[entry.API] {
			continue
		}
		delete(watched, entry.API)
		if entry.Status != StatusSupported {
			t.Fatalf("%s status = %s, want %s", entry.API, entry.Status, StatusSupported)
		}
		if !strings.Contains(entry.Notes, "Deterministic local") {
			t.Fatalf("%s notes do not describe deterministic local URL behavior: %s", entry.API, entry.Notes)
		}
	}
	for api := range watched {
		t.Fatalf("missing stdlib row %s", api)
	}
}

func TestUserInfoLocalIdentityStdlibRowsAreSupported(t *testing.T) {
	watched := map[string]bool{
		"UserInfo.getFirstName":                true,
		"UserInfo.getLanguage":                 true,
		"UserInfo.getLastName":                 true,
		"UserInfo.getLocale":                   true,
		"UserInfo.getName":                     true,
		"UserInfo.getOrganizationId":           true,
		"UserInfo.getProfileId":                true,
		"UserInfo.getSessionId":                true,
		"UserInfo.getUserEmail":                true,
		"UserInfo.getUserId":                   true,
		"UserInfo.getUserName":                 true,
		"UserInfo.getUserType":                 true,
		"UserInfo.isMultiCurrencyOrganization": true,
	}
	for _, entry := range StdlibMatrix() {
		if !watched[entry.API] {
			continue
		}
		delete(watched, entry.API)
		if entry.Status != StatusSupported {
			t.Fatalf("%s status = %s, want %s", entry.API, entry.Status, StatusSupported)
		}
		notes := strings.ToLower(entry.Notes)
		if !strings.Contains(notes, "local") && !strings.Contains(notes, "runas") {
			t.Fatalf("%s notes do not describe the local user identity model: %s", entry.API, entry.Notes)
		}
	}
	for api := range watched {
		t.Fatalf("missing stdlib row %s", api)
	}
}

func TestStdlibSupportedRowsDoNotClaimPlaceholderOrNoOpBehavior(t *testing.T) {
	for _, entry := range StdlibMatrix() {
		if entry.Status != StatusSupported {
			continue
		}
		notes := strings.ToLower(entry.Notes)
		if strings.Contains(notes, "placeholder") || strings.Contains(notes, "no-op") {
			t.Fatalf("stdlib row %s is supported with placeholder/no-op notes: %s", entry.API, entry.Notes)
		}
	}
}

func TestCoreServiceContextStdlibRowsAreExplicitUnsupported(t *testing.T) {
	watched := map[string]bool{
		"QuickAction.describeAvailableActions":                                          true,
		"QuickAction.describeAvailableQuickActions(String)":                             true,
		"QuickAction.describeQuickActions(List<String>)":                                true,
		"QuickAction.performQuickAction":                                                true,
		"QuickAction.performQuickAction(QuickAction.QuickActionRequest)":                true,
		"QuickAction.performQuickAction(QuickAction.QuickActionRequest,Boolean)":        true,
		"QuickAction.performQuickActions(List<QuickAction.QuickActionRequest>)":         true,
		"QuickAction.performQuickActions(List<QuickAction.QuickActionRequest>,Boolean)": true,
		"QuickAction.retrieveQuickActionTemplate(String,Id)":                            true,
		"QuickAction.retrieveQuickActionTemplates(List<String>,Id)":                     true,
		"Request.getCurrent()":                                                          true,
		"Request.getQuiddity()":                                                         true,
		"Request.getRequestId()":                                                        true,
		"RequestImpl.getCurrent()":                                                      true,
		"ResetPasswordResult.getPassword()":                                             true,
		"SandboxContext.organizationId()":                                               true,
		"SandboxContext.sandboxId()":                                                    true,
		"SandboxContext.sandboxName()":                                                  true,
		"SandboxPostCopy.runApexClass(SandboxContext)":                                  true,
		"Schedulable.execute(SchedulableContext)":                                       true,
		"SchedulableContext.getTriggerId()":                                             true,
		"Search.find(String,Object)":                                                    true,
		"Search.query(String,Object)":                                                   true,
		"Search.suggest(String,String,Object)":                                          true,
		"Search.suggest(String,String,Object,Object)":                                   true,
		"System.enqueueJob(Object,Object)":                                              true,
		"System.runAs(Object,Object)":                                                   true,
		"System.runAs(Package.Version)":                                                 true,
		"System.schedule(String,String,Object)":                                         true,
		"Test.enableChangeDataCapture()":                                                true,
		"Test.getEventBus()":                                                            true,
		"Test.getExternalService()":                                                     true,
		"Test.invokeContinuationMethod(Object,Continuation)":                            true,
		"Test.newSendEmailQuickActionDefaults(Id,Id)":                                   true,
		"Test.setContinuationResponse(String,HttpResponse)":                             true,
		"Test.setCurrentPageReference(Object)":                                          true,
		"Test.testInstall(InstallHandler,Version)":                                      true,
		"Test.testInstall(InstallHandler,Version,Boolean)":                              true,
		"Test.testNotificationActionHandler(Messaging.NotificationActionHandler,Messaging.ActionableNotification)": true,
		"Test.testSandboxPostCopyScript(SandboxPostCopy,Id,Id,String)":                                             true,
		"Test.testSandboxPostCopyScript(SandboxPostCopy,Id,Id,String,Boolean)":                                     true,
		"Test.testUninstall(UninstallHandler)":                                                                     true,
		"TrailblazerIdentity.generateUserEmailVerificationToken(String,String,String)":                             true,
		"TrailblazerIdentity.getUserOrgInfo(List<String>)":                                                         true,
		"TrailblazerIdentity.splunkLog(String,String)":                                                             true,
		"UIRequest.getCurrent()":                       true,
		"UIRequest.getRequestHeader(String)":           true,
		"UserInfo.hasPackageLicense(Id)":               true,
		"UserInfo.isCurrentUserLicensedForPackage(Id)": true,
	}
	for _, entry := range StdlibMatrix() {
		if !watched[entry.API] {
			continue
		}
		if entry.Status != StatusUnsupported {
			t.Fatalf("%s = %s, want unsupported", entry.API, entry.Status)
		}
		delete(watched, entry.API)
	}
	if len(watched) > 0 {
		t.Fatalf("missing explicit unsupported core service/context rows: %#v", watched)
	}
}

func TestWebServiceCalloutStdlibRowsAreLocallyPromotedOrFenced(t *testing.T) {
	watched := map[string]Status{
		"WebServiceCallout.invoke(Object,Object,Map,List)":                        StatusPartial,
		"WebServiceCallout.invoke(Object,Object,Map<String,Object>,List<String>)": StatusPartial,
	}
	for _, entry := range StdlibMatrix() {
		want, ok := watched[entry.API]
		if !ok {
			continue
		}
		delete(watched, entry.API)
		if entry.Status != want {
			t.Fatalf("%s = %s, want %s: %s", entry.API, entry.Status, want, entry.Notes)
		}
	}
	if len(watched) > 0 {
		t.Fatalf("missing WebServiceCallout stdlib rows: %#v", watched)
	}
}

func TestHTTPStdlibRowsAreLocallyPromotedOrFenced(t *testing.T) {
	watched := map[string]Status{
		"Http.send(HttpRequest)": StatusSupported,
	}
	for _, entry := range StdlibMatrix() {
		want, ok := watched[entry.API]
		if !ok {
			continue
		}
		delete(watched, entry.API)
		if entry.Status != want {
			t.Fatalf("%s = %s, want %s: %s", entry.API, entry.Status, want, entry.Notes)
		}
		if entry.Notes == "" {
			t.Fatalf("%s needs local-model notes", entry.API)
		}
	}
	for api := range watched {
		t.Fatalf("missing HTTP stdlib row %s", api)
	}
}

func TestDateDatetimeTimeZoneRowsAreLocallyPromotedOrFenced(t *testing.T) {
	watched := map[string]bool{
		"Date.addMonths":          true,
		"Date.addYears":           true,
		"Date.today":              true,
		"Datetime.addDays":        true,
		"Datetime.addMonths":      true,
		"Datetime.addYears":       true,
		"Datetime.format":         true,
		"Datetime.formatGmt":      true,
		"Datetime.now":            true,
		"TimeZone.getDisplayName": true,
		"TimeZone.getID":          true,
		"TimeZone.getOffset":      true,
		"TimeZone.getTimeZone":    true,
		"UserInfo.getTimeZone":    true,
	}
	for _, entry := range StdlibMatrix() {
		if !watched[entry.API] {
			continue
		}
		if entry.Status != StatusSupported {
			t.Fatalf("%s remains %s: %s", entry.API, entry.Status, entry.Notes)
		}
		if entry.Notes == "" {
			t.Fatalf("%s needs local-model notes", entry.API)
		}
	}
}
