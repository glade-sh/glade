package apextest

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

func emailServiceXML(name, apexClass, runAsUser, localPart string) string {
	class := ""
	if apexClass != "" {
		class = "<apexClass>" + apexClass + "</apexClass>"
	}
	return `<?xml version="1.0" encoding="UTF-8"?>
<EmailServicesFunction xmlns="http://soap.sforce.com/2006/04/metadata">` + class + `<attachmentOption>All</attachmentOption><authenticationFailureAction>Discard</authenticationFailureAction><authorizationFailureAction>Bounce</authorizationFailureAction><emailServicesAddresses><developerName>Addr</developerName><isActive>true</isActive><localPart>` + localPart + `</localPart><runAsUser>` + runAsUser + `</runAsUser></emailServicesAddresses><functionInactiveAction>Bounce</functionInactiveAction><functionName>` + name + `</functionName><isActive>true</isActive><isAuthenticationRequired>false</isAuthenticationRequired><isErrorRoutingEnabled>false</isErrorRoutingEnabled><isTextAttachmentsAsBinary>false</isTextAttachmentsAsBinary><isTlsRequired>false</isTlsRequired><overLimitAction>Requeue</overLimitAction></EmailServicesFunction>`
}

// invalidEmailServiceProject writes four invalid services and one valid one,
// beside ordinary Apex tests. noClassSeeded is the row count SeedTest expects
// for NoClassService.
func invalidEmailServiceProject(t *testing.T, noClassSeeded int) (root string, files map[string]string) {
	t.Helper()
	root = t.TempDir()
	api := "67.0"
	writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
	dir := filepath.Join(root, "force-app/main/default")
	files = map[string]string{
		"NoClassService":      filepath.Join(dir, "emailservices/NoClassService.xml"),
		"PlaceholderService":  filepath.Join(dir, "emailservices/PlaceholderService.xml"),
		"UnknownUserService":  filepath.Join(dir, "emailservices/UnknownUserService.xml"),
		"LongLocalService":    filepath.Join(dir, "emailservices/LongLocalService.xml"),
		"GoodService":         filepath.Join(dir, "emailservices/GoodService.xml"),
		"SyntheticHandlerCls": filepath.Join(dir, "classes/SyntheticHandler.cls"),
	}
	writeFile(t, files["NoClassService"], emailServiceXML("NoClassService", "", "system@example.invalid", "noclass"))
	writeFile(t, files["PlaceholderService"], emailServiceXML("PlaceholderService", "SyntheticHandler", "${SYNTH_ADMIN_USERNAME}", "placeholder"))
	writeFile(t, files["UnknownUserService"], emailServiceXML("UnknownUserService", "SyntheticHandler", "missing@example.invalid", "unknownuser"))
	writeFile(t, files["LongLocalService"], emailServiceXML("LongLocalService", "SyntheticHandler", "system@example.invalid", strings.Repeat("a", 65)))
	writeFile(t, files["GoodService"], emailServiceXML("GoodService", "SyntheticHandler", "system@example.invalid", "good"))
	classes := map[string]string{
		"SyntheticHandler": `global class SyntheticHandler implements Messaging.InboundEmailHandler {
    global Messaging.InboundEmailResult handleInboundEmail(Messaging.InboundEmail email, Messaging.InboundEnvelope envelope) {
        return new Messaging.InboundEmailResult();
    }
}`,
		"PlainTest": `@IsTest
private class PlainTest {
    @IsTest static void adds() { System.assertEquals(2, 1 + 1); }
}`,
		"SeedTest": fmt.Sprintf(`@IsTest
private class SeedTest {
    @IsTest static void onlyValidSeeds() {
        System.assertEquals(1, [SELECT COUNT() FROM EmailServicesFunction WHERE FunctionName = 'GoodService']);
        System.assertEquals(%d, [SELECT COUNT() FROM EmailServicesFunction WHERE FunctionName = 'NoClassService']);
        System.assertEquals(0, [SELECT COUNT() FROM EmailServicesFunction WHERE FunctionName = 'PlaceholderService']);
        System.assertEquals(0, [SELECT COUNT() FROM EmailServicesFunction WHERE FunctionName = 'UnknownUserService']);
        System.assertEquals(0, [SELECT COUNT() FROM EmailServicesFunction WHERE FunctionName = 'LongLocalService']);
    }
}`, noClassSeeded),
	}
	for name, source := range classes {
		path := filepath.Join(dir, "classes", name+".cls")
		writeFile(t, path, source)
		writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
	}
	return root, files
}

// assertEmailServiceWarnings requires one warning per skipped service, naming
// the file and carrying the native deploy-time text.
func assertEmailServiceWarnings(t *testing.T, diags []diagnostic.Diagnostic, files map[string]string, skipped map[string]string) {
	t.Helper()
	for name, native := range skipped {
		found := 0
		for _, d := range diags {
			if d.File == files[name] {
				found++
				if d.Severity != diagnostic.Warning || d.Code != "GLADEEMAILSERVICE001" || !strings.Contains(d.Message, native) {
					t.Errorf("%s warning = %+v, want warning GLADEEMAILSERVICE001 containing %q", name, d, native)
				}
			}
		}
		if found != 1 {
			t.Errorf("%s: %d warnings, want 1 (all: %+v)", name, found, diags)
		}
	}
	for _, d := range diags {
		if d.Severity == diagnostic.Error {
			t.Errorf("unexpected error diagnostic %+v", d)
		}
		if d.File == files["GoodService"] {
			t.Errorf("valid service warned: %+v", d)
		}
	}
}

var allInvalidEmailServices = map[string]string{
	"NoClassService":     "email service NoClassService: an Apex Class must be specified",
	"PlaceholderService": "email service PlaceholderService: no User named ${SYNTH_ADMIN_USERNAME} found",
	"UnknownUserService": "email service UnknownUserService: no User named missing@example.invalid found",
	"LongLocalService":   "email service LongLocalService: LocalPart maximum length is:64",
}

// An invalid email-service setup file is rejected by a metadata deploy, but a
// local run must skip it, warn per file and keep every unrelated test runnable.
func TestInvalidEmailServiceMetadataDoesNotAbortProjectRun(t *testing.T) {
	for _, route := range []struct {
		name string
		opts Options
	}{
		{"default", Options{NoDiskCache: true}},
		{"perf", Options{NoDiskCache: true, PerfCounters: true}},
	} {
		t.Run(route.name, func(t *testing.T) {
			root, files := invalidEmailServiceProject(t, 0)
			index := loadTestIndex(t, root)
			if index.HasErrors() {
				t.Fatalf("index errors: %v", index.Diagnostics)
			}
			assertEmailServiceWarnings(t, index.Diagnostics, files, allInvalidEmailServices)
			summary := Run(index, route.opts).Summary()
			if summary.Total != 2 || summary.Passed != 2 {
				t.Fatalf("summary = %+v, want 2 of 2 passed", summary)
			}
		})
	}
}

// Fixing a skipped file must clear its warning and seed the service, with no
// stale index reuse.
func TestInvalidEmailServiceMetadataEditThenRerun(t *testing.T) {
	root, files := invalidEmailServiceProject(t, 1)
	before := loadTestIndex(t, root)
	assertEmailServiceWarnings(t, before.Diagnostics, files, allInvalidEmailServices)
	writeFile(t, files["NoClassService"], emailServiceXML("NoClassService", "SyntheticHandler", "system@example.invalid", "noclass"))
	after := loadTestIndex(t, root)
	rest := map[string]string{}
	for name, native := range allInvalidEmailServices {
		if name != "NoClassService" {
			rest[name] = native
		}
	}
	assertEmailServiceWarnings(t, after.Diagnostics, files, rest)
	for _, d := range after.Diagnostics {
		if d.File == files["NoClassService"] {
			t.Errorf("stale warning after edit: %+v", d)
		}
	}
	if typesys.SameProjectIdentity(before, after) {
		t.Error("email-service edit left the index identity unchanged")
	}
	summary := Run(after, Options{NoDiskCache: true}).Summary()
	if summary.Total != 2 || summary.Passed != 2 {
		t.Fatalf("summary = %+v, want 2 of 2 passed", summary)
	}
}
