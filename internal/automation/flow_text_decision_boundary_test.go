package automation

import "testing"

// The interview, subflow and graph builders consume this same normalized XML.
// Native EB/EC rows express negation with NOT 1 over Contains; the raw
// DoesNotContain spelling is rejected (flow_text_email/api{62,67}/ORG-RUN.txt).
func TestFlowTextNegationInterviewSubflowAndGraphTargets(t *testing.T) {
	raw := flowXML{Decisions: []flowDecisionXML{{
		DefaultConnector: flowConnectorXML{TargetReference: "Unmatched"},
		Rules: []flowDecisionRuleXML{{
			ConditionLogic: "NOT 1", Connector: flowConnectorXML{TargetReference: "Matched"},
			Conditions: []flowConditionXML{{LeftValueReference: "InputText", Operator: "Contains", RightValue: flowValueXML{StringValue: "x"}}},
		}},
	}}}
	flowNormalizeTextDecisionNegation(&raw, map[string]flowVariableXML{"inputtext": {DataType: "String"}})
	decision := raw.Decisions[0]
	rule := decision.Rules[0]
	if rule.Conditions[0].Operator != "Contains" || rule.ConditionLogic != "and" || rule.Connector.TargetReference != "Unmatched" || decision.DefaultConnector.TargetReference != "Matched" {
		t.Fatalf("negation must preserve Contains and exchange targets: %+v", decision)
	}
}

// Every caller must request a frame only for the captured scalar NOT Contains
// shape. Other decisions retain their existing lowering and diagnostics.
func TestFlowTextNegationInterviewAndTriggerNormalizationBoundaries(t *testing.T) {
	for _, route := range []string{"interview", "trigger_plain"} {
		for _, tc := range []struct {
			name, logic, operator, dataType, rightReference, defaultTarget string
			collection, want                                               bool
		}{
			{name: "native_scalar", want: true},
			{name: "positive_contains", logic: "and"},
			{name: "raw_operator", operator: "DoesNotContain"},
			{name: "collection", collection: true},
			{name: "non_text", dataType: "Number"},
			{name: "dynamic_operand", rightReference: "OtherText"},
			{name: "missing_default", defaultTarget: "none"},
		} {
			t.Run(route+"/"+tc.name, func(t *testing.T) {
				logic, operator, dataType, defaultTarget := tc.logic, tc.operator, tc.dataType, tc.defaultTarget
				if logic == "" {
					logic = "NOT 1"
				}
				if operator == "" {
					operator = "Contains"
				}
				if dataType == "" {
					dataType = "String"
				}
				if defaultTarget == "" {
					defaultTarget = "Unmatched"
				}
				if defaultTarget == "none" {
					defaultTarget = ""
				}
				raw := flowXML{Decisions: []flowDecisionXML{{
					DefaultConnector: flowConnectorXML{TargetReference: defaultTarget},
					Rules: []flowDecisionRuleXML{{
						ConditionLogic: logic, Connector: flowConnectorXML{TargetReference: "Matched"},
						Conditions: []flowConditionXML{{LeftValueReference: "InputText", Operator: operator, RightValue: flowValueXML{StringValue: "x", ElementReference: tc.rightReference}}},
					}},
				}}}
				if route == "trigger_plain" {
					raw.Start.Object, raw.Start.TriggerType = "Account", "RecordAfterSave"
				}
				got := flowNormalizeTextDecisionNegation(&raw, map[string]flowVariableXML{"inputtext": {DataType: dataType, IsCollection: tc.collection}})
				if got != tc.want {
					t.Fatalf("requires scalar frame=%t; want %t", got, tc.want)
				}
				if !got && (raw.Decisions[0].Rules[0].ConditionLogic != logic || raw.Decisions[0].Rules[0].Connector.TargetReference != "Matched" || raw.Decisions[0].DefaultConnector.TargetReference != defaultTarget) {
					t.Fatalf("uncaptured decision changed: %+v", raw.Decisions[0])
				}
			})
		}
	}
}
