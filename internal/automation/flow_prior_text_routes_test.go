package automation

import "testing"

func TestFlowPriorTextCriteriaBeforeAndAfterSaveLowering(t *testing.T) {
	for _, trigger := range []string{"RecordBeforeSave", "RecordAfterSave"} {
		for _, operator := range []string{"EqualTo", "NotEqualTo", "IsNull"} {
			t.Run(trigger+"/"+operator, func(t *testing.T) {
				condition := flowConditionXML{LeftValueReference: "$Record__Prior.Title", Operator: operator, RightValue: flowValueXML{StringValue: "T1"}}
				if operator == "IsNull" {
					condition.RightValue = flowValueXML{BooleanValue: "false"}
				}
				raw := flowXML{
					Start:     flowStartXML{Object: "Contact", TriggerType: trigger, RecordTriggerType: "CreateAndUpdate"},
					Decisions: []flowDecisionXML{{Name: "Prior", Rules: []flowDecisionRuleXML{{Name: "Match", Conditions: []flowConditionXML{condition}}}}},
				}
				if !flowPriorDecisionConditionSupported(condition) || !flowPriorDecisionNeedsGraph(raw) {
					t.Fatal("captured direct prior-field criteria must use the interview graph")
				}
			})
		}
	}
}
