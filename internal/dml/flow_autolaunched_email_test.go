package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// An interview-run Flow resolves Send Email inputs from its own variables and
// formulas, and records an EmailMessage only when logEmailOnSend is true.
// No native capture covers the unlogged case; it follows the documented
// logEmailOnSend input and a public project's passing org tests.
func TestAutolaunchedFlowEmailSimpleUsesInterviewVariables(t *testing.T) {
	for _, tc := range []struct {
		name    string
		logSend string
		want    int
	}{
		{"unlogged", "false", 0},
		{"logged", "true", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rule := storage.FlowRule{
				Name: "ScalarEmail", Active: true, ProcessType: "AutoLaunchedFlow",
				Variables: []storage.FlowVariable{
					{Name: "bodyText", DataType: "String", IsInput: true},
					{Name: "bodyHtml", DataType: "String", IsInput: true},
				},
				Steps: []storage.FlowStep{{Kind: "action", Action: storage.FlowAction{
					Name: "Send", ActionType: "emailSimple", ActionName: "emailSimple",
					Inputs: []storage.WorkflowFieldUpdate{
						{Name: "emailSubject", SourceField: "bodyText"},
						{Name: "emailBody", Formula: "BLANKVALUE(bodyHtml, bodyText)"},
						{Name: "logEmailOnSend", LiteralValue: tc.logSend},
					},
				}}},
			}
			org := storage.NewOrgState()
			engine := NewEngine(&org)
			_, err := engine.RunAutolaunchedFlowWithOutput(rule, FlowInterviewInput{Scalars: map[string]storage.Value{
				"bodyText": storage.StringValue("plain"), "bodyHtml": storage.StringValue(""),
			}})
			if err != nil {
				t.Fatal(err)
			}
			records := org.Objects["EmailMessage"].Records
			if len(records) != tc.want {
				t.Fatalf("EmailMessage count = %d, want %d", len(records), tc.want)
			}
			for _, record := range records {
				if got := record.Fields["TextBody"].String; got != "plain" {
					t.Fatalf("TextBody = %q, want plain", got)
				}
				if got := record.Fields["Subject"].String; got != "plain" {
					t.Fatalf("Subject = %q, want plain", got)
				}
			}
		})
	}
}
