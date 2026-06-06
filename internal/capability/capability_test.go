package capability

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestMVPReportIsNotReadyUntilRequiredFeaturesAreSupported(t *testing.T) {
	report := MVPReport()
	if report.Ready {
		t.Fatal("MVP report should not be ready while required features are partial or unsupported")
	}
	if report.Required == 0 || report.Incomplete == 0 {
		t.Fatalf("report = %#v", report)
	}
	for _, feature := range report.Features {
		if feature.Required && feature.Status != StatusSupported {
			return
		}
	}
	t.Fatal("expected at least one incomplete required feature")
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
	if !strings.Contains(text, "MVP readiness: not ready") || !strings.Contains(text, "Trigger invocation") {
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
		"Required incomplete:",
		"### `apex.sema.body`: Method-body semantic analysis",
		"### `release.packaging`: Installable release binaries, checksums, docs",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("known gaps output missing %q: %q", want, text)
		}
	}
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
		"Test.invokePage(PageReference)":                                                true,
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
		"UIRequest.getCurrent()":                                                  true,
		"UIRequest.getRequestHeader(String)":                                      true,
		"UserInfo.hasPackageLicense(Id)":                                          true,
		"UserInfo.isCurrentUserLicensedForPackage(Id)":                            true,
		"WebServiceCallout.invoke(Object,Object,Map,List)":                        true,
		"WebServiceCallout.invoke(Object,Object,Map<String,Object>,List<String>)": true,
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

func TestHTTPStdlibRowsAreLocallyPromotedOrFenced(t *testing.T) {
	watched := map[string]Status{
		"Http.send local mock callouts":    StatusSupported,
		"Http.send real network transport": StatusUnsupported,
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
