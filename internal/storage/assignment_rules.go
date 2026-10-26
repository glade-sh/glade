package storage

// AssignmentRule is an ordered Lead or Case metadata rule. Its ID is shared
// with the queryable AssignmentRule setup row.
type AssignmentRule struct {
	ID      ID                    `json:"id,omitempty"`
	Object  string                `json:"object,omitempty"`
	Name    string                `json:"name"`
	Active  bool                  `json:"active,omitempty"`
	Entries []AssignmentRuleEntry `json:"entries,omitempty"`
}

type AssignmentRuleEntry struct {
	AssignedTo     string                 `json:"assignedTo"`
	AssignedToType string                 `json:"assignedToType"`
	Formula        string                 `json:"formula,omitempty"`
	BooleanFilter  string                 `json:"booleanFilter,omitempty"`
	Criteria       []WorkflowCriteriaItem `json:"criteria,omitempty"`
}

type AssignmentRuleSelection struct {
	UseDefault bool
	ID         ID
}

func CloneAssignmentRules(rules []AssignmentRule) []AssignmentRule {
	out := append([]AssignmentRule(nil), rules...)
	for i := range out {
		out[i].Entries = append([]AssignmentRuleEntry(nil), rules[i].Entries...)
		for j := range out[i].Entries {
			out[i].Entries[j].Criteria = append([]WorkflowCriteriaItem(nil), rules[i].Entries[j].Criteria...)
		}
	}
	return out
}
