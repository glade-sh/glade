package gladecli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
)

func dbEmailServiceXML(name, apexClass, runAsUser, localPart string) string {
	class := ""
	if apexClass != "" {
		class = "<apexClass>" + apexClass + "</apexClass>"
	}
	return `<?xml version="1.0" encoding="UTF-8"?>
<EmailServicesFunction xmlns="http://soap.sforce.com/2006/04/metadata">` + class + `<attachmentOption>All</attachmentOption><authenticationFailureAction>Discard</authenticationFailureAction><authorizationFailureAction>Bounce</authorizationFailureAction><emailServicesAddresses><developerName>Addr</developerName><isActive>true</isActive><localPart>` + localPart + `</localPart><runAsUser>` + runAsUser + `</runAsUser></emailServicesAddresses><functionInactiveAction>Bounce</functionInactiveAction><functionName>` + name + `</functionName><isActive>true</isActive><isAuthenticationRequired>false</isAuthenticationRequired><isErrorRoutingEnabled>false</isErrorRoutingEnabled><isTextAttachmentsAsBinary>false</isTextAttachmentsAsBinary><isTlsRequired>false</isTlsRequired><overLimitAction>Requeue</overLimitAction></EmailServicesFunction>`
}

// The db route builds its org from the same project index. Invalid email
// services must be skipped with a per-file warning, not fail the command.
func TestDBOrgSkipsInvalidEmailServicesWithWarnings(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("sfdx-project.json", `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	dir := "force-app/main/default/"
	write(dir+"classes/SyntheticHandler.cls", `global class SyntheticHandler implements Messaging.InboundEmailHandler {
    global Messaging.InboundEmailResult handleInboundEmail(Messaging.InboundEmail email, Messaging.InboundEnvelope envelope) { return new Messaging.InboundEmailResult(); }
}`)
	write(dir+"classes/SyntheticHandler.cls-meta.xml", `<ApexClass><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>`)
	write(dir+"emailservices/NoClassService.xml", dbEmailServiceXML("NoClassService", "", "system@example.invalid", "noclass"))
	write(dir+"emailservices/PlaceholderService.xml", dbEmailServiceXML("PlaceholderService", "SyntheticHandler", "${SYNTH_ADMIN_USERNAME}", "placeholder"))
	write(dir+"emailservices/UnknownUserService.xml", dbEmailServiceXML("UnknownUserService", "SyntheticHandler", "missing@example.invalid", "unknownuser"))
	write(dir+"emailservices/LongLocalService.xml", dbEmailServiceXML("LongLocalService", "SyntheticHandler", "system@example.invalid", strings.Repeat("a", 65)))
	write(dir+"emailservices/GoodService.xml", dbEmailServiceXML("GoodService", "SyntheticHandler", "system@example.invalid", "good"))

	_, index, err := loadProjectIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	for name, native := range map[string]string{
		"NoClassService":     "email service NoClassService: an Apex Class must be specified",
		"PlaceholderService": "email service PlaceholderService: no User named ${SYNTH_ADMIN_USERNAME} found",
		"UnknownUserService": "email service UnknownUserService: no User named missing@example.invalid found",
		"LongLocalService":   "email service LongLocalService: LocalPart maximum length is:64",
	} {
		found := 0
		for _, d := range index.Diagnostics {
			if d.File == filepath.Join(root, dir, "emailservices", name+".xml") {
				found++
				if d.Severity != diagnostic.Warning || d.Code != "GLADEEMAILSERVICE001" || !strings.Contains(d.Message, native) {
					t.Errorf("%s warning = %+v", name, d)
				}
			}
		}
		if found != 1 {
			t.Errorf("%s: %d warnings, want 1", name, found)
		}
	}
	org, err := orgForProject(root)
	if err != nil {
		t.Fatalf("db org build failed: %v", err)
	}
	var names []string
	for _, r := range org.Objects["EmailServicesFunction"].Records {
		names = append(names, r.Fields["FunctionName"].String)
	}
	if len(names) != 1 || names[0] != "GoodService" {
		t.Errorf("seeded services = %v, want [GoodService]", names)
	}
}
