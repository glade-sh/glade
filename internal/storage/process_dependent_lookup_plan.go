package storage

import "strings"

// CapturedProcessRelationshipIDBinding recognizes the relationship-ID input
// observed with the projected dependent-lookup subflow.
func (rule FlowRule) CapturedProcessRelationshipIDBinding(input WorkflowFieldUpdate) (string, bool) {
	steps, ok := rule.CapturedProcessDependentLookupSteps()
	if !ok || !rule.Variables[0].IsOutput || len(steps[1].RecordLookup.QueriedFields) != 2 {
		return "", false
	}
	criterion := steps[0].RecordLookup.Criteria[1]
	link := criterion.Field
	if len(link) <= 3 || !strings.EqualFold(link[len(link)-3:], "__c") || !strings.EqualFold(input.Field, criterion.SourceField) {
		return "", false
	}
	return link, strings.EqualFold(input.SourceField, link[:len(link)-3]+"__r.Id")
}

// CapturedProcessDependentLookupSteps recognizes the two-input, two-lookup
// process subflow captured in process_builder_review_lookup.
func (rule FlowRule) CapturedProcessDependentLookupSteps() ([]FlowStep, bool) {
	if len(rule.Variables) != 3 || len(rule.Steps) != 4 || len(rule.Branches) != 0 || len(rule.Paths) != 0 || len(rule.StepTargets) != 0 {
		return nil, false
	}
	variables := map[string]bool{}
	output := rule.Variables[0].IsOutput
	for _, v := range rule.Variables {
		if !v.IsInput || v.IsOutput != output || v.IsCollection || !strings.EqualFold(v.DataType, "String") || (v.DefaultValue != nil && v.DefaultValue.Kind != ValueNull) || v.Name == "" {
			return nil, false
		}
		key := strings.ToLower(v.Name)
		if variables[key] {
			return nil, false
		}
		variables[key] = true
	}
	for _, s := range rule.Steps {
		if s.FaultTarget != "" || len(s.FaultBranch) != 0 {
			return nil, false
		}
	}
	one, two := rule.Steps[0], rule.Steps[1]
	validLookup := func(s FlowStep, n int) bool {
		l := s.RecordLookup
		projected := n == 1 && output && len(l.QueriedFields) == 2
		return s.Kind == "recordLookup" && l.Name != "" && l.ObjectName != "" && l.GetFirstRecordOnly && l.StoreOutputAutomatically && !l.AssignNullValuesIfNoRecordsFound && l.OutputReference == "" && (len(l.QueriedFields) == 0 || projected) && len(l.OutputAssignments) == 0 && l.SortField == "" && l.SortOrder == "" && strings.EqualFold(l.CriteriaLogic, "and") && len(l.Criteria) == n
	}
	if !validLookup(one, 2) || !validLookup(two, 1) {
		return nil, false
	}
	c0, c1, c2 := one.RecordLookup.Criteria[0], one.RecordLookup.Criteria[1], two.RecordLookup.Criteria[0]
	validCriterion := func(c WorkflowCriteriaItem) bool {
		return strings.EqualFold(c.Operation, "equals") && c.Value == "" && c.SourceField != "" && variables[strings.ToLower(c.SourceField)]
	}
	if !strings.EqualFold(c0.Field, "Id") || !validCriterion(c0) || c1.Field == "" || strings.EqualFold(c1.Field, "Id") || !validCriterion(c1) || !strings.EqualFold(c2.Field, "Id") || !validCriterion(c2) || !strings.EqualFold(c1.SourceField, c2.SourceField) || strings.EqualFold(c0.SourceField, c1.SourceField) {
		return nil, false
	}
	a := rule.Steps[2].Assignment
	prefix, field, ok := strings.Cut(a.SourceField, ".")
	if rule.Steps[2].Kind != "assignment" || !strings.EqualFold(a.Operator, "Assign") || !variables[strings.ToLower(a.Target)] || strings.EqualFold(a.Target, c0.SourceField) || strings.EqualFold(a.Target, c1.SourceField) || !ok || !strings.EqualFold(prefix, two.RecordLookup.Name) || field == "" || a.LiteralValue != "" || a.Formula != "" {
		return nil, false
	}
	if fields := two.RecordLookup.QueriedFields; len(fields) != 0 && (!strings.EqualFold(fields[0], "Id") || !strings.EqualFold(fields[1], field)) {
		return nil, false
	}
	u := rule.Steps[3].RecordUpdate
	if rule.Steps[3].Kind != "recordUpdate" || !strings.EqualFold(u.ObjectName, one.RecordLookup.ObjectName) || u.InputReference != "" || u.CriteriaLogic != "" || len(u.Criteria) != 1 || len(u.InputAssignments) != 1 {
		return nil, false
	}
	criterion, assignment := u.Criteria[0], u.InputAssignments[0]
	if !strings.EqualFold(criterion.Field, "Id") || !strings.EqualFold(criterion.Operation, "equals") || !strings.EqualFold(criterion.SourceField, c0.SourceField) || criterion.Value != "" || assignment.Field == "" || !strings.EqualFold(assignment.SourceField, a.Target) || assignment.Formula != "" || assignment.LiteralValue != "" || assignment.ActionOutput != "" {
		return nil, false
	}
	return rule.Steps, true
}
