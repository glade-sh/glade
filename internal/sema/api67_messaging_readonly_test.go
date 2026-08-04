package sema

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

const cb133MessagingReads = `
Messaging.EmailFileAttachment attachment = new Messaging.EmailFileAttachment();
Messaging.SingleEmailMessage message = new Messaging.SingleEmailMessage();
Id attachmentId = attachment.id;
String templateName = message.templatename;
Boolean userMail = message.usermail;
attachment.getId();
message.getTemplateName();
message.isUserMail();
`

const cb133MessagingAssignments = `
Messaging.EmailFileAttachment attachment = new Messaging.EmailFileAttachment();
Messaging.SingleEmailMessage message = new Messaging.SingleEmailMessage();
attachment.id = null;
message.templatename = null;
message.usermail = null;
`

func TestAPI67MessagingReadOnlyPropertiesAnonymous(t *testing.T) {
	result := AnalyzeAnonymous(typesys.Index{}, cb133MessagingReads)
	if result.HasErrors() {
		t.Fatalf("Messaging read-only properties and getters should compile: %#v", result.Diagnostics)
	}
}

func TestAPI67MessagingReadOnlyPropertiesClass(t *testing.T) {
	root := t.TempDir()
	classPath := filepath.Join(root, "CB133MessagingReadOnly.cls")
	writeSemaFile(t, classPath, "public class CB133MessagingReadOnly {\n  public void run() {\n"+cb133MessagingReads+"  }\n}\n")
	result := Analyze(typesys.Build(project.Project{Root: root, ApexFiles: []string{classPath}}, schema.Schema{}))
	if result.HasErrors() {
		t.Fatalf("Messaging read-only properties and getters should compile in a class: %#v", result.Diagnostics)
	}
}

func TestAPI67MessagingAssignmentsRejectedAnonymous(t *testing.T) {
	assertCB133MessagingAssignmentsRejected(t, AnalyzeAnonymous(typesys.Index{}, cb133MessagingAssignments))
}

func TestAPI67MessagingAssignmentsRejectedClass(t *testing.T) {
	root := t.TempDir()
	classPath := filepath.Join(root, "CB133MessagingAssignments.cls")
	writeSemaFile(t, classPath, "public class CB133MessagingAssignments {\n  public void run() {\n"+cb133MessagingAssignments+"  }\n}\n")
	result := Analyze(typesys.Build(project.Project{Root: root, ApexFiles: []string{classPath}}, schema.Schema{}))
	assertCB133MessagingAssignmentsRejected(t, result)
}

func TestAPI67MessagingUndeclaredResultConstructorsRejectedAnonymous(t *testing.T) {
	for _, typeName := range []string{
		"Messaging.ActionResult",
		"Messaging.ActionableNotification",
		"Messaging.SendEmailResult",
		"Messaging.SendEmailError",
	} {
		result := AnalyzeAnonymous(typesys.Index{}, typeName+" value = new "+typeName+"();")
		if !result.HasErrors() {
			t.Fatalf("expected Salesforce API 67 to reject undeclared constructor: %s", typeName)
		}
	}
}

func assertCB133MessagingAssignmentsRejected(t *testing.T, result Result) {
	t.Helper()
	if len(result.Diagnostics) != 3 {
		t.Fatalf("Messaging read-only assignments produced %d diagnostics, want 3: %#v", len(result.Diagnostics), result.Diagnostics)
	}
	wantFields := []string{"attachment.id", "message.templatename", "message.usermail"}
	for _, field := range wantFields {
		matches := 0
		for _, diagnostic := range result.Diagnostics {
			if diagnostic.Code == "GLADESEMA028" && strings.Contains(diagnostic.Message, field) {
				matches++
			}
		}
		if matches != 1 {
			t.Fatalf("Messaging assignment %q matched %d diagnostics: %#v", field, matches, result.Diagnostics)
		}
	}
}
