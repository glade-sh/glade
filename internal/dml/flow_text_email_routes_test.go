package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Expectations are pinned by flow_text_email/api{62,67}/org.tsv: FS, FF,
// EB/EC Contains operands and the email true/false/omitted rows. Negation is
// pinned end to end by the project conformance tests. Invocable calls are twins.
func TestFlowTextDecisionInterviewAndInvocableRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"interview", "invocable"} {
			for _, row := range []struct {
				name, operation, comparison string
				value                       storage.Value
				want                        bool
			}{
				{"FS", "StartsWith", "hello", storage.StringValue("Hello World"), true},
				{"FF", "EndsWith", "world", storage.StringValue("Hello World"), true},
				{"EB_Contains_operand", "Contains", "world", storage.StringValue("Hello World"), true},
				{"EC_Contains_operand", "Contains", "x", storage.NullValue(), false},
			} {
				t.Run(api+"/"+route+"/"+row.name, func(t *testing.T) {
					org := storage.NewOrgState()
					org.APIVersion = api
					engine := NewEngine(&org)
					engine.InvocableFlow = route == "invocable"
					frame := newFlowFrame()
					frame.scalars["inputtext"] = row.value
					got, supported := engine.evaluateFlowFrameBranch(storage.FlowBranch{Criteria: []storage.WorkflowCriteriaItem{{
						Field: "InputText", Operation: row.operation, Value: row.comparison,
					}}}, storage.Record{}, storage.ObjectDefinition{}, frame)
					if !supported || got != row.want {
						t.Fatalf("matched=%t supported=%t; native matched=%t", got, supported, row.want)
					}
				})
			}
		}
	}
}

func TestFlowEmailLoggingInterviewInvocableAndTriggerRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"interview", "interview_subflow", "invocable", "invocable_subflow", "trigger", "trigger_subflow"} {
			for _, logSend := range []string{"true", "false", "omitted"} {
				t.Run(api+"/"+route+"/"+logSend, func(t *testing.T) {
					org := storage.NewOrgState()
					org.APIVersion = api
					storage.EnsureStandardObject(&org, "Account")
					inputs := []storage.WorkflowFieldUpdate{
						{Name: "emailSubject", LiteralValue: "Flow logging route"},
						{Name: "emailBody", LiteralValue: "variable body"},
					}
					if logSend != "omitted" {
						inputs = append(inputs, storage.WorkflowFieldUpdate{Name: "logEmailOnSend", LiteralValue: logSend})
					}
					rule := storage.FlowRule{Name: "LoggingRoute", Active: true, ProcessType: "AutoLaunchedFlow",
						Steps: []storage.FlowStep{{Kind: "action", Action: storage.FlowAction{Name: "Send", ActionType: "emailSimple", ActionName: "emailSimple", Inputs: inputs}}}}
					if route == "interview_subflow" || route == "invocable_subflow" || route == "trigger_subflow" {
						child := rule
						child.Name = "LoggingChild"
						org.Metadata.Flows = []storage.FlowRule{child}
						rule.Steps = []storage.FlowStep{{Kind: "subflow", Subflow: storage.FlowSubflow{Name: "Call", FlowName: child.Name}}}
					}
					trigger := route == "trigger" || route == "trigger_subflow"
					if trigger {
						rule.TriggerType, rule.RecordTriggerType = "RecordAfterSave", "Create"
						state := org.Objects["Account"]
						state.Definition.FlowRules = []storage.FlowRule{rule}
						org.Objects["Account"] = state
					}
					engine := NewEngine(&org)
					engine.InvocableFlow = route == "invocable" || route == "invocable_subflow"
					if trigger {
						result := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Logging seed")}}})
						if len(result) != 1 || !result[0].Success {
							t.Fatalf("insert: %+v", result)
						}
					} else if _, err := engine.RunAutolaunchedFlowWithOutput(rule, FlowInterviewInput{}); err != nil {
						t.Fatal(err)
					}
					want := 0
					if logSend == "true" {
						want = 1
					}
					if got := len(org.Objects["EmailMessage"].Records); got != want {
						t.Fatalf("EmailMessage count=%d; native count=%d", got, want)
					}
					for _, message := range org.Objects["EmailMessage"].Records {
						if got := message.Fields["TextBody"].String; got != "variable body" {
							t.Fatalf("TextBody=%q; native body=variable body", got)
						}
					}
				})
			}
		}
	}
}
