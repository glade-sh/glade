package vm

import "strings"

// These standard change events have reflection identities even though they
// are absent from the ordinary standard-object describe catalog.
func standardChangeEventName(name string) (string, bool) {
	for _, candidate := range []string{
		"AccountChangeEvent", "ContactChangeEvent", "LeadChangeEvent",
		"OpportunityChangeEvent", "CaseChangeEvent",
	} {
		if strings.EqualFold(name, candidate) {
			return candidate, true
		}
	}
	return "", false
}
