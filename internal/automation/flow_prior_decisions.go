package automation

import "strings"

func flowHasSaveTrigger(raw flowXML) bool {
	return strings.EqualFold(strings.TrimSpace(raw.Start.TriggerType), "RecordBeforeSave") || strings.EqualFold(strings.TrimSpace(raw.Start.TriggerType), "RecordAfterSave")
}

func flowPriorDecisionReference(reference string) bool {
	return strings.HasPrefix(strings.TrimSpace(reference), "$Record__Prior.")
}

func flowPriorDecisionConditionSupported(condition flowConditionXML) bool {
	reference := strings.TrimSpace(condition.LeftValueReference)
	if !flowPriorDecisionReference(reference) {
		return false
	}
	field := strings.TrimPrefix(reference, "$Record__Prior.")
	operand := strings.TrimSpace(condition.RightValue.BooleanValue)
	simple := field != "" && !strings.Contains(field, ".")
	switch strings.ToLower(strings.TrimSpace(condition.Operator)) {
	case "equalto", "notequalto":
		// Native rows at API 62.0 and 67.0 cover text, lookup and Boolean
		// fields under EqualTo and NotEqualTo against an existing prior.
		return simple
	case "isnull":
		// Keep the invalid traversal through identity visible to schema
		// validation; other prior relationship paths are not admitted.
		if field == "Id" || strings.HasPrefix(field, "Id.") {
			return strings.EqualFold(operand, "true")
		}
		return simple && (strings.EqualFold(operand, "true") || strings.EqualFold(operand, "false"))
	default:
		return false
	}
}

func flowPriorDecisionNeedsGraph(raw flowXML) bool {
	mode := strings.TrimSpace(raw.Start.RecordTriggerType)
	if !flowHasSaveTrigger(raw) || !strings.EqualFold(mode, "Update") && !strings.EqualFold(mode, "CreateAndUpdate") {
		return false
	}
	for _, decision := range raw.Decisions {
		for _, rule := range decision.Rules {
			for _, condition := range rule.Conditions {
				if flowPriorDecisionConditionSupported(condition) {
					return true
				}
				if strings.EqualFold(strings.TrimSpace(condition.Operator), "IsChanged") {
					field := flowRecordSourceFieldReference(condition.LeftValueReference)
					operand := strings.TrimSpace(condition.RightValue.BooleanValue)
					if field != "" && !strings.Contains(field, ".") && (strings.EqualFold(operand, "true") || strings.EqualFold(operand, "false")) {
						return true
					}
				}
			}
		}
	}
	return false
}
