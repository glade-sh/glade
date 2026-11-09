package storage

// AutoResponseRule describes ordered project-owned Case or Lead response rules.
type AutoResponseRule struct {
	Object  string                  `json:"object,omitempty"`
	Name    string                  `json:"name"`
	Active  bool                    `json:"active,omitempty"`
	Entries []AutoResponseRuleEntry `json:"entries,omitempty"`
}
type AutoResponseRuleEntry struct {
	Formula       string                 `json:"formula,omitempty"`
	BooleanFilter string                 `json:"booleanFilter,omitempty"`
	Criteria      []WorkflowCriteriaItem `json:"criteria,omitempty"`
	Subject       string                 `json:"subject,omitempty"`
	Template      string                 `json:"template,omitempty"`
}

func CloneAutoResponseRules(rules []AutoResponseRule) []AutoResponseRule {
	out := append([]AutoResponseRule(nil), rules...)
	for i := range out {
		out[i].Entries = append([]AutoResponseRuleEntry(nil), rules[i].Entries...)
		for j := range out[i].Entries {
			out[i].Entries[j].Criteria = append([]WorkflowCriteriaItem(nil), rules[i].Entries[j].Criteria...)
		}
	}
	return out
}
