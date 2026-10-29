package automation

import "testing"

func TestFlowRefsDeleteLookupPreservesExistingRoutes(t *testing.T) {
	for _, shape := range []string{"singleton-field-equality", "explicit-output-null", "collection-empty"} {
		t.Run(shape, func(t *testing.T) {
			lookup := flowRecordLookupXML{Name: "Found", StoreOutputAutomatically: true, GetFirstRecordOnly: true}
			condition := flowConditionXML{LeftValueReference: "Found.Title", Operator: "EqualTo"}
			switch shape {
			case "explicit-output-null":
				lookup.StoreOutputAutomatically = false
				lookup.OutputAssignments = []flowLookupOutputXML{{}}
				condition.LeftValueReference, condition.Operator = "Found", "IsNull"
			case "collection-empty":
				lookup.GetFirstRecordOnly = false
				condition.LeftValueReference, condition.Operator = "Found", "IsEmpty"
			}
			raw := flowXML{ProcessType: "AutoLaunchedFlow", Start: flowStartXML{TriggerType: "RecordBeforeDelete"}, RecordLookups: []flowRecordLookupXML{lookup}, Decisions: []flowDecisionXML{{Rules: []flowDecisionRuleXML{{Conditions: []flowConditionXML{condition}}}}}}
			if !flowRefsDecisionNeedsLookup(raw) {
				t.Fatal("existing before-delete lookup route was suppressed")
			}
		})
	}
}

func TestFlowOrderedCollectionPreservesDeleteProjection(t *testing.T) {
	raw := flowXML{ProcessType: "AutoLaunchedFlow", Start: flowStartXML{Object: "Account", TriggerType: "RecordBeforeDelete"}, RecordLookups: []flowRecordLookupXML{{Name: "Found", StoreOutputAutomatically: true}}}
	if flowRequiresOrderedGraph(raw) {
		t.Fatal("uncaptured before-delete collection lookup must retain its existing projection")
	}
}
