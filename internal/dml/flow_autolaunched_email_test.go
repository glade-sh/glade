package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// An interview-run Flow resolves Send Email inputs from its own variables and
// formulas, and records an EmailMessage only when logEmailOnSend is true.
// Native logging rows: flow_text_email/api{62,67}/org.tsv, direct_email_*
// and subflow_email_*. The BLANKVALUE expression is a local formula variation.
func TestAutolaunchedFlowEmailSimpleUsesInterviewVariables(t *testing.T) {
	for _, viaSubflow := range []bool{false, true} {
		for _, tc := range []struct {
			name    string
			logSend string
			want    int
		}{
			{"unlogged", "false", 0},
			{"logged", "true", 1},
		} {
			name := tc.name
			if viaSubflow {
				name += " subflow"
			}
			t.Run(name, func(t *testing.T) {
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
				run := rule
				if viaSubflow {
					child := rule
					child.Name = "ScalarEmailChild"
					org.Metadata.Flows = []storage.FlowRule{child}
					run = storage.FlowRule{
						Name: "ScalarEmailParent", Active: true, ProcessType: "AutoLaunchedFlow",
						Variables: rule.Variables,
						Steps: []storage.FlowStep{{Kind: "subflow", Subflow: storage.FlowSubflow{
							Name: "Call", FlowName: "ScalarEmailChild",
							InputAssignments: []storage.WorkflowFieldUpdate{
								{Name: "bodyText", SourceField: "bodyText"},
								{Name: "bodyHtml", SourceField: "bodyHtml"},
							},
						}}},
					}
				}
				engine := NewEngine(&org)
				_, err := engine.RunAutolaunchedFlowWithOutput(run, FlowInterviewInput{Scalars: map[string]storage.Value{
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
}
