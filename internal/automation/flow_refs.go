package automation

import "strings"

// A decision consuming Get Records must run after that lookup. Whole-record
// null checks use ordered execution for save flows; delete flows retain their
// existing branch projection.
func flowRefsDecisionNeedsLookup(raw flowXML) bool {
	for _, decision := range raw.Decisions {
		for _, rule := range decision.Rules {
			for _, condition := range rule.Conditions {
				reference := strings.TrimSpace(condition.LeftValueReference)
				for _, lookup := range raw.RecordLookups {
					if len(lookup.OutputAssignments) > 0 && !lookup.StoreOutputAutomatically && strings.TrimSpace(lookup.OutputReference) == "" &&
						strings.EqualFold(reference, firstNonBlank(lookup.Name, lookup.Label)) && strings.EqualFold(condition.Operator, "IsNull") {
						return true
					}
					if !lookup.StoreOutputAutomatically {
						continue
					}
					name := firstNonBlank(lookup.Name, lookup.Label)
					if name == "" {
						continue
					}
					if lookup.GetFirstRecordOnly && strings.EqualFold(condition.Operator, "EqualTo") && strings.HasPrefix(strings.ToLower(reference), strings.ToLower(name)+".") {
						return true
					}
					if flowHasSaveTrigger(raw) && lookup.GetFirstRecordOnly && strings.EqualFold(reference, name) && strings.EqualFold(condition.Operator, "IsNull") {
						value := flowLiteralValue(condition.RightValue)
						if strings.EqualFold(value, "true") || strings.EqualFold(value, "false") {
							return true
						}
					}
					if !lookup.GetFirstRecordOnly && strings.EqualFold(reference, name) && strings.EqualFold(condition.Operator, "IsEmpty") {
						return true
					}
				}
			}
		}
	}
	return false
}
