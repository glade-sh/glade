package automation

import "strings"

// Route supported Get Records decision criteria through ordered execution so
// the lookup value is available. Delete-trigger flows retain their existing routes.
func flowRefsDecisionNeedsLookup(raw flowXML) bool {
	deleteTrigger := strings.EqualFold(strings.TrimSpace(raw.Start.TriggerType), "RecordBeforeDelete")
	for _, decision := range raw.Decisions {
		for _, rule := range decision.Rules {
			for _, condition := range rule.Conditions {
				reference := strings.TrimSpace(condition.LeftValueReference)
				for _, lookup := range raw.RecordLookups {
					name := firstNonBlank(lookup.Name, lookup.Label)
					// An element that only wrote scalar outputs still resolves in a
					// decision, so the ordered graph must run the Get Records first.
					if len(lookup.OutputAssignments) > 0 && !lookup.StoreOutputAutomatically && strings.TrimSpace(lookup.OutputReference) == "" &&
						strings.EqualFold(reference, name) && strings.EqualFold(condition.Operator, "IsNull") {
						return true
					}
					if !lookup.StoreOutputAutomatically {
						continue
					}
					if name == "" {
						continue
					}
					// Captured field operators and existing delete equality need
					// the lookup output before evaluating their criteria.
					if strings.HasPrefix(strings.ToLower(reference), strings.ToLower(name)+".") {
						if lookup.GetFirstRecordOnly && (strings.EqualFold(condition.Operator, "EqualTo") || !deleteTrigger && flowRefsDecisionRecordFieldOperatorSupported(condition.Operator)) {
							return true
						}
						continue
					}
					if !strings.EqualFold(reference, name) {
						continue
					}
					operation := strings.ToLower(strings.TrimSpace(condition.Operator))
					if !lookup.GetFirstRecordOnly && (operation == "isempty" || !deleteTrigger && operation == "isnull") {
						return true
					}
					if flowHasSaveTrigger(raw) && lookup.GetFirstRecordOnly && operation == "isnull" {
						value := flowLiteralValue(condition.RightValue)
						if strings.EqualFold(value, "true") || strings.EqualFold(value, "false") {
							return true
						}
					}
				}
			}
		}
	}
	return false
}

func flowRefsDecisionRecordFieldOperatorSupported(operator string) bool {
	switch strings.ToLower(strings.TrimSpace(operator)) {
	case "contains", "startswith", "endswith", "notequalto", "equalto", "isnull", "isblank":
		return true
	default:
		return false
	}
}
