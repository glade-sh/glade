package typesys

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
)

const incrementalEmailServiceXML = `<?xml version="1.0" encoding="UTF-8"?>
<EmailServicesFunction xmlns="http://soap.sforce.com/2006/04/metadata">
    <apexClass>InboundMailHandler</apexClass>
    <attachmentOption>None</attachmentOption>
    <authenticationFailureAction>Discard</authenticationFailureAction>
    <authorizationFailureAction>Discard</authorizationFailureAction>
    <functionInactiveAction>Discard</functionInactiveAction>
    <functionName>InboundMail</functionName>
    <isActive>true</isActive>
    <isAuthenticationRequired>false</isAuthenticationRequired>
    <isErrorRoutingEnabled>false</isErrorRoutingEnabled>
    <isTextAttachmentsAsBinary>false</isTextAttachmentsAsBinary>
    <isTlsRequired>false</isTlsRequired>
    <overLimitAction>Discard</overLimitAction>
</EmailServicesFunction>
`

func emailServiceWarnings(idx Index) []string {
	var out []string
	for _, d := range idx.Diagnostics {
		if d.Code == "GLADEEMAILSERVICE001" {
			out = append(out, d.File+": "+d.Message)
		}
	}
	return out
}

func buildEmailServiceIndex(t *testing.T, root string) (project.Project, Index) {
	t.Helper()
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	return p, Build(p, s)
}

func TestEmailServiceHandlerPresenceChangeInvalidatesIncrementalUpdate(t *testing.T) {
	root := t.TempDir()
	writeIncrementalSFDXProject(t, root, "")
	writeFile(t, filepath.Join(root, "force-app/main/default/emailservices/InboundMail.xml"), incrementalEmailServiceXML)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/Other.cls"), "public class Other {}")
	handler := filepath.Join(root, "force-app/main/default/classes/InboundMailHandler.cls")
	writeFile(t, handler, "public class InboundMailHandler implements Messaging.InboundEmailHandler { public Messaging.InboundEmailResult handleInboundEmail(Messaging.InboundEmail e, Messaging.InboundEnvelope v) { return null; } }")

	_, previous := buildEmailServiceIndex(t, root)
	if got := emailServiceWarnings(previous); len(got) != 0 {
		t.Fatalf("valid handler produced warnings: %v", got)
	}

	// Present to missing: the cached clean result must not be reused.
	if err := os.Remove(handler); err != nil {
		t.Fatal(err)
	}
	_, fresh := buildEmailServiceIndex(t, root)
	want := emailServiceWarnings(fresh)
	if len(want) == 0 {
		t.Fatal("fresh build did not warn about the missing handler class")
	}
	updated, fast := updateApexFilesIncremental(previous, nil, []string{handler})
	if fast {
		if got := emailServiceWarnings(updated); !equalStrings(got, want) {
			t.Fatalf("incremental warnings %v, fresh build %v", got, want)
		}
	}

	// Missing to present: the warning must clear.
	writeFile(t, handler, "public class InboundMailHandler implements Messaging.InboundEmailHandler { public Messaging.InboundEmailResult handleInboundEmail(Messaging.InboundEmail e, Messaging.InboundEnvelope v) { return null; } }")
	_, again := buildEmailServiceIndex(t, root)
	if got := emailServiceWarnings(again); len(got) != 0 {
		t.Fatalf("fresh build kept a stale warning: %v", got)
	}
	// The warned index never takes the fast path, so a restored handler rebuilds.
	if updated, fast := updateApexFilesIncremental(fresh, []string{handler}, nil); fast {
		if got := emailServiceWarnings(updated); len(got) != 0 {
			t.Fatalf("incremental update kept a stale warning: %v", got)
		}
	}

	// An unrelated class edit and delete still use the fast path when the handler is unchanged.
	other := filepath.Join(root, "force-app/main/default/classes/Other.cls")
	writeFile(t, other, "public class Other { Integer x; }")
	if _, fast := updateApexFilesIncremental(again, []string{other}, nil); !fast {
		t.Fatal("unrelated Apex edit lost the incremental path")
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
