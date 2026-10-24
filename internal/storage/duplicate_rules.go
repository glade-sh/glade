package storage

// DuplicateRule retains deployed metadata independently of DML transaction options.
type DuplicateRule struct {
	ID                 ID                  `json:"id,omitempty"`
	Object             string              `json:"object,omitempty"`
	Subtype            string              `json:"subtype,omitempty"`
	Name               string              `json:"name"`
	Label              string              `json:"label,omitempty"`
	Active             bool                `json:"active"`
	ActionOnInsert     string              `json:"actionOnInsert,omitempty"`
	ActionOnUpdate     string              `json:"actionOnUpdate,omitempty"`
	OperationsOnInsert []string            `json:"operationsOnInsert,omitempty"`
	OperationsOnUpdate []string            `json:"operationsOnUpdate,omitempty"`
	AlertText          string              `json:"alertText,omitempty"`
	SecurityOption     string              `json:"securityOption,omitempty"`
	SortOrder          int                 `json:"sortOrder,omitempty"`
	Filter             DuplicateRuleFilter `json:"filter,omitzero"`
	MatchRules         []MatchingRule      `json:"matchRules,omitempty"`
}

type DuplicateRuleFilter struct {
	BooleanFilter string                    `json:"booleanFilter,omitempty" xml:"booleanFilter"`
	Items         []DuplicateRuleFilterItem `json:"items,omitempty" xml:"duplicateRuleFilterItems"`
}

type DuplicateRuleFilterItem struct {
	Field     string `json:"field" xml:"field"`
	Operation string `json:"operation" xml:"operation"`
	Value     string `json:"value,omitempty" xml:"value"`
	SortOrder int    `json:"sortOrder,omitempty" xml:"sortOrder"`
}

type MatchingRule struct {
	Object        string             `json:"object"`
	Name          string             `json:"name" xml:"fullName"`
	Label         string             `json:"label,omitempty" xml:"label"`
	Status        string             `json:"status,omitempty" xml:"ruleStatus"`
	BooleanFilter string             `json:"booleanFilter,omitempty" xml:"booleanFilter"`
	Items         []MatchingRuleItem `json:"items,omitempty" xml:"matchingRuleItems"`
}

type MatchingRuleItem struct {
	Field         string `json:"field" xml:"fieldName"`
	Method        string `json:"method" xml:"matchingMethod"`
	BlankBehavior string `json:"blankBehavior" xml:"blankValueBehavior"`
}

func CloneDuplicateRules(rules []DuplicateRule) []DuplicateRule {
	out := append([]DuplicateRule(nil), rules...)
	for i := range out {
		out[i].OperationsOnInsert = append([]string(nil), rules[i].OperationsOnInsert...)
		out[i].OperationsOnUpdate = append([]string(nil), rules[i].OperationsOnUpdate...)
		out[i].Filter.Items = append([]DuplicateRuleFilterItem(nil), rules[i].Filter.Items...)
		out[i].MatchRules = append([]MatchingRule(nil), rules[i].MatchRules...)
		for j := range out[i].MatchRules {
			out[i].MatchRules[j].Items = append([]MatchingRuleItem(nil), rules[i].MatchRules[j].Items...)
		}
	}
	return out
}
