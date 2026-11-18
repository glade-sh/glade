package storage

import "strings"

// CapturedProcessRecordActionSteps recognizes the SObject-input, same-action
// graph in process_builder_review_record. Other action and assignment graphs
// stay unrun until they have native rows.
func (rule FlowRule) CapturedProcessRecordActionSteps() ([]FlowStep, bool) {
	if len(rule.Variables) != 1 || len(rule.Steps) != 4 || len(rule.Branches) != 0 || len(rule.Paths) != 0 || len(rule.StepTargets) != 0 || len(rule.ProcessStringConstants) != 1 {
		return nil, false
	}
	v := rule.Variables[0]
	if !v.IsInput || v.IsOutput || v.IsCollection || !strings.EqualFold(v.DataType, "SObject") || v.ObjectType == "" || v.DefaultValue != nil {
		return nil, false
	}
	for _, step := range rule.Steps {
		if step.FaultTarget != "" || len(step.FaultBranch) != 0 {
			return nil, false
		}
	}
	first, last := rule.Steps[0], rule.Steps[3]
	if first.Kind != "action" || last.Kind != "action" || !strings.EqualFold(first.Action.ActionType, "apex") || !strings.EqualFold(last.Action.ActionType, "apex") || first.Action.ActionName == "" || !strings.EqualFold(first.Action.ActionName, last.Action.ActionName) || len(first.Action.Inputs) != 4 || len(last.Action.Inputs) != 4 || len(first.Action.Outputs) != 0 || len(last.Action.Outputs) != 0 {
		return nil, false
	}
	literals, constants, records := 0, 0, 0
	seen := make(map[string]bool)
	for i, a := range first.Action.Inputs {
		key := strings.ToLower(a.Field)
		if seen[key] {
			return nil, false
		}
		seen[key] = true
		b := last.Action.Inputs[i]
		if a.Field == "" || !strings.EqualFold(a.Field, b.Field) || a.Formula != "" || b.Formula != "" || a.ActionOutput != "" || b.ActionOutput != "" {
			return nil, false
		}
		switch {
		case a.LiteralValue != "" && b.LiteralValue != "" && a.SourceField == "" && b.SourceField == "":
			literals++
		case strings.EqualFold(a.SourceField, v.Name) && strings.EqualFold(b.SourceField, v.Name) && a.LiteralValue == "" && b.LiteralValue == "":
			records++
		case a.SourceField != "" && strings.EqualFold(a.SourceField, b.SourceField) && a.LiteralValue == "" && b.LiteralValue == "":
			if _, ok := rule.ProcessStringConstants[strings.ToLower(a.SourceField)]; !ok {
				return nil, false
			}
			constants++
		default:
			return nil, false
		}
	}
	if literals != 2 || constants != 1 || records != 1 {
		return nil, false
	}
	a := rule.Steps[1].Assignment
	prefix, field, ok := strings.Cut(a.Target, ".")
	if rule.Steps[1].Kind != "assignment" || !ok || !strings.EqualFold(prefix, v.Name) || field == "" || !strings.EqualFold(a.Operator, "Assign") || a.LiteralValue == "" || a.SourceField != "" || a.Formula != "" {
		return nil, false
	}
	u := rule.Steps[2].RecordUpdate
	if rule.Steps[2].Kind != "recordUpdate" || !strings.EqualFold(u.ObjectName, v.ObjectType) || !strings.EqualFold(u.InputReference, v.Name) || len(u.Criteria) != 0 || len(u.InputAssignments) != 0 || u.CriteriaLogic != "" {
		return nil, false
	}
	return rule.Steps, true
}
