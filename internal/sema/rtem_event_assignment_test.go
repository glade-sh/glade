package sema

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Native Salesforce rows (scratch org, API 62.0 and 67.0 deploys, identical):
// Real-Time Event Monitoring event and store objects accept Apex assignment to
// every field, including describe-read-only ones such as CreatedDate and
// EventIdentifier. Ordinary read-only standard fields stay rejected.
func TestRTEMEventFieldAssignmentMatchesNative(t *testing.T) {
	for _, tc := range []struct {
		object, field, value string
		rejected             bool
	}{
		{"ReportEvent", "UserId", "UserInfo.getUserId()", false},
		{"ReportEvent", "Operation", "'ReportExported'", false},
		{"ReportEvent", "EventDate", "Datetime.now()", false},
		{"ReportEvent", "CreatedDate", "Datetime.now()", false},
		{"ReportEvent", "EventIdentifier", "'abc'", false},
		{"ReportEvent", "Sequence", "1", false},
		{"ApiEvent", "Records", "'r'", false},
		{"ApiEvent", "Query", "'q'", false},
		{"LoginEvent", "CountryIso", "'US'", false},
		{"LoginEvent", "CipherSuite", "'c'", false},
		{"ListViewEvent", "ColumnHeaders", "'c'", false},
		{"LoginAsEvent", "DelegatedUsername", "'u'", false},
		{"FileEventStore", "FileAction", "'a'", false},
		{"BulkApiResultEventStore", "Query", "'q'", false},
		{"UriEvent", "CreatedDate", "Datetime.now()", false},
		{"LogoutEvent", "CreatedDate", "Datetime.now()", false},
		{"SessionHijackingEventStore", "CreatedDate", "Datetime.now()", false},
		{"PermissionSetEventStore", "CreatedDate", "Datetime.now()", false},
		{"IdentityVerificationEvent", "CreatedDate", "Datetime.now()", false},
		{"LightningUriEvent", "CreatedDate", "Datetime.now()", false},
		{"Contact", "CreatedDate", "Datetime.now()", true},
		{"Account", "CreatedDate", "Datetime.now()", true},
		{"Event", "CreatedDate", "Datetime.now()", true},
		{"ExternalEvent", "CreatedDate", "Datetime.now()", true},
		{"LiveChatTranscriptEvent", "CreatedDate", "Datetime.now()", true},
		{"TopicUserEvent", "CreatedDate", "Datetime.now()", true},
		{"LoginHistory", "UserId", "UserInfo.getUserId()", true},
		{"AsyncApexJob", "Status", "'x'", true},
	} {
		for _, api := range []string{"62.0", "67.0"} {
			t.Run(fmt.Sprintf("%s.%s@%s", tc.object, tc.field, api), func(t *testing.T) {
				root := t.TempDir()
				path := filepath.Join(root, "RtemProbe.cls")
				writeSemaFile(t, path, fmt.Sprintf("public class RtemProbe { public static void run() { %s e = new %s(); e.%s = %s; } }", tc.object, tc.object, tc.field, tc.value))
				result := Analyze(typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{}))
				rejected := false
				for _, d := range result.Diagnostics {
					if d.Code == "GLADESEMA027" {
						rejected = true
					}
				}
				if rejected != tc.rejected {
					t.Fatalf("rejected=%v want %v: %#v", rejected, tc.rejected, result.Diagnostics)
				}
			})
		}
	}
}
