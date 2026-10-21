package automation

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// The captured presence decision follows an automatic first-record lookup on
// the triggering object. Whole records, other fields and other operators keep
// their existing projection.
func flowCoreLookupIDReference(raw flowXML, decision flowDecisionXML) string {
	if !strings.EqualFold(raw.Start.TriggerType, "RecordAfterSave") || !strings.EqualFold(raw.Start.RecordTriggerType, "CreateAndUpdate") || raw.Start.DoesRequireRecordChangedToMeetCriteria {
		return ""
	}
	if len(decision.Rules) != 1 || len(decision.Rules[0].Conditions) != 1 || !strings.EqualFold(strings.TrimSpace(decision.Rules[0].ConditionLogic), "and") {
		return ""
	}
	if strings.TrimSpace(decision.Rules[0].Connector.TargetReference) == "" || strings.TrimSpace(decision.DefaultConnector.TargetReference) == "" {
		return ""
	}
	condition := decision.Rules[0].Conditions[0]
	if !strings.EqualFold(condition.Operator, "IsNull") || condition.RightValue != (flowValueXML{BooleanValue: "false"}) {
		return ""
	}
	reference := strings.TrimSpace(condition.LeftValueReference)
	resource, field, found := strings.Cut(reference, ".")
	if !found || !strings.EqualFold(field, "Id") {
		return ""
	}
	for _, lookup := range raw.RecordLookups {
		name := firstNonBlank(lookup.Name, lookup.Label)
		if lookup.GetFirstRecordOnly && lookup.StoreOutputAutomatically && strings.EqualFold(resource, name) && strings.EqualFold(lookup.Object, raw.Start.Object) && strings.EqualFold(raw.Start.Connector.TargetReference, name) && strings.EqualFold(lookup.Connector.TargetReference, decision.Name) {
			return reference
		}
	}
	return ""
}

func flowCoreLookupIDDecision(raw flowXML, decision flowDecisionXML) (flowDecisionXML, []storage.FlowStep) {
	reference := flowCoreLookupIDReference(raw, decision)
	if reference == "" {
		return decision, nil
	}
	// Resolve the output only after Get Records has run. The existing scalar
	// decision evaluator then handles both an ID and a valid null output.
	const temporary = "$GladeLookupId"
	rule := decision.Rules[0]
	condition := rule.Conditions[0]
	condition.LeftValueReference = temporary
	rule.Conditions = []flowConditionXML{condition}
	decision.Rules = []flowDecisionRuleXML{rule}
	return decision, []storage.FlowStep{{Kind: "assignment", Assignment: storage.FlowAssignment{
		Name: temporary, Target: temporary, Operator: "Assign", SourceField: reference,
	}}}
}
