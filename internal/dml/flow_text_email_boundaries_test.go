package dml

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Raw DoesNotContain is rejected in flow_text_email/api{62,67}/ORG-RUN.txt.
// Null prefix/suffix are false, pinned by flow_text_prefix_suffix/api{62,67}/org.tsv
// startswith_null and endswith_null. Email Alert retains its uncaptured boundary.
func TestFlowTextBoundaryValueCriteria(t *testing.T) {
	for _, operation := range []string{"DoesNotContain", "StartsWith", "EndsWith"} {
		values := []storage.Value{storage.NullValue()}
		if operation == "DoesNotContain" {
			values = append(values, storage.StringValue("Hello World"))
		}
		for _, value := range values {
			t.Run(operation+"/"+string(value.Kind), func(t *testing.T) {
				if matched, supported := evaluateFlowValueCriteria(value, storage.WorkflowCriteriaItem{Operation: operation, Value: "x"}); supported != (operation != "DoesNotContain") || matched {
					t.Fatalf("matched=%t supported=%t; native null prefix/suffix are supported and false", matched, supported)
				}
			})
		}
	}
}

func TestFlowTextEmailBoundaryEngineRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"interview", "interview_subflow", "invocable", "invocable_subflow", "trigger", "trigger_subflow", "trigger_legacy_actions"} {
			for _, boundary := range []string{"DoesNotContain", "StartsWith", "EndsWith", "emailAlert_false", "emailAlert_omitted", "emailAlert_true", "emailAlert_name_alias"} {
				if route == "trigger_legacy_actions" && !strings.HasPrefix(boundary, "emailAlert") {
					continue
				}
				t.Run(api+"/"+route+"/"+boundary, func(t *testing.T) {
					org := storage.NewOrgState()
					org.APIVersion = api
					storage.EnsureStandardObject(&org, "Account")
					trigger := strings.HasPrefix(route, "trigger")
					email := strings.HasPrefix(boundary, "emailAlert")
					rule := storage.FlowRule{Name: "Boundary", Active: true, ProcessType: "AutoLaunchedFlow"}
					if email {
						action := storage.FlowAction{Name: "Send", ActionType: "emailAlert", ActionName: "Account.Alert"}
						if boundary == "emailAlert_name_alias" {
							action.ActionType, action.ActionName = "", "emailAlert"
						}
						if boundary != "emailAlert_omitted" {
							action.Inputs = []storage.WorkflowFieldUpdate{{Name: "logEmailOnSend", LiteralValue: "false"}}
							if boundary == "emailAlert_true" {
								action.Inputs[0].LiteralValue = "true"
							}
						}
						rule.Steps = []storage.FlowStep{{Kind: "action", Action: action}}
						if route == "trigger_legacy_actions" {
							rule.Steps, rule.Actions = nil, []storage.FlowAction{action}
						}
					} else {
						value := storage.NullValue()
						if boundary == "DoesNotContain" {
							value = storage.StringValue("Hello World")
						}
						rule.Variables = []storage.FlowVariable{{Name: "InputText", DataType: "String", DefaultValue: &value}}
						rule.Steps = []storage.FlowStep{{Kind: "decision", Branches: []storage.FlowBranch{{Name: "Match", Criteria: []storage.WorkflowCriteriaItem{{Field: "InputText", Operation: boundary, Value: "x"}}}}}}
					}
					if strings.HasSuffix(route, "_subflow") {
						child := rule
						child.Name = "BoundaryChild"
						org.Metadata.Flows = []storage.FlowRule{child}
						rule.Steps = []storage.FlowStep{{Kind: "subflow", Subflow: storage.FlowSubflow{Name: "Call", FlowName: child.Name}}}
					}
					if trigger {
						rule.TriggerType, rule.RecordTriggerType = "RecordAfterSave", "Create"
						state := org.Objects["Account"]
						state.Definition.FlowRules = []storage.FlowRule{rule}
						org.Objects["Account"] = state
					}
					engine := NewEngine(&org)
					engine.InvocableFlow = strings.HasPrefix(route, "invocable")
					var runErr error
					if trigger {
						results := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Boundary seed")}}})
						if len(results) != 1 {
							t.Fatalf("insert results: %+v", results)
						}
						if !results[0].Success {
							if email || boundary != "DoesNotContain" {
								t.Fatalf("insert: %+v", results)
							}
							if !strings.Contains(results[0].Error, "unsupported criteria") {
								t.Fatalf("unexpected rejection: %+v", results)
							}
							return
						}
					} else {
						_, runErr = engine.RunAutolaunchedFlowWithOutput(rule, FlowInterviewInput{})
					}
					if !email && boundary == "DoesNotContain" {
						if runErr == nil || !strings.Contains(runErr.Error(), "unsupported criteria") {
							t.Fatalf("error=%v; want unsupported criteria", runErr)
						}
						return
					}
					if runErr != nil {
						t.Fatal(runErr)
					}
					want := 0
					if email && (trigger || boundary == "emailAlert_true") {
						want = 1
					}
					if got := len(org.Objects["EmailMessage"].Records); got != want {
						t.Fatalf("EmailMessage count=%d; pre-extension count=%d", got, want)
					}
				})
			}
		}
	}
}
