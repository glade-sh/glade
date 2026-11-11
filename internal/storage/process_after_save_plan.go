package storage

import (
	"sort"
	"strings"
)

// CapturedFourAfterSavePlan admits only the graph and field-type arrangement in
// process_builder_review fourflows. Its return order is the observed order of
// the four flow interviews captured by the two opposite name orderings.
func CapturedFourAfterSavePlan(rules []FlowRule, definition ObjectDefinition, definitions map[string]ObjectDefinition) ([]FlowRule, bool) {
	if len(rules) != 4 {
		return nil, false
	}
	var entry WorkflowCriteriaItem
	var create, updates, last, actions *FlowRule
	sameCriterion := func(c []WorkflowCriteriaItem) bool {
		return len(c) == 1 && strings.EqualFold(c[0].Field, entry.Field) && strings.EqualFold(c[0].Operation, "equals") && c[0].Value == entry.Value && c[0].SourceField == ""
	}
	plainStep := func(s FlowStep) bool { return s.FaultTarget == "" && len(s.FaultBranch) == 0 }
	branch := func(b FlowBranch) bool { return !b.Default && b.Formula == "" && sameCriterion(b.Criteria) }
	literalUpdate := func(s FlowStep) bool {
		if s.Kind != "fieldUpdate" || !plainStep(s) || len(s.FieldUpdates) != 1 {
			return false
		}
		u := s.FieldUpdates[0]
		return u.Field != "" && u.LiteralValue != "" && u.Formula == "" && !u.FlowFrameFormula && u.SourceField == "" && u.ActionOutput == "" && !strings.EqualFold(u.Field, entry.Field)
	}
	action := func(s FlowStep) bool {
		if s.Kind != "action" || !plainStep(s) || !strings.EqualFold(s.Action.ActionType, "apex") || len(s.Action.Inputs) != 1 || len(s.Action.Outputs) != 0 {
			return false
		}
		i := s.Action.Inputs[0]
		return i.Field != "" && strings.EqualFold(i.SourceField, "$Record") && i.LiteralValue == "" && i.Formula == "" && i.ActionOutput == ""
	}
	for n := range rules {
		r := &rules[n]
		if !r.Active || !strings.EqualFold(r.TriggerType, "RecordAfterSave") || !strings.EqualFold(r.RecordTriggerType, "CreateAndUpdate") || r.TriggerOrder != 0 || r.RunInMode != "" || r.Formula != "" || r.DoesRequireRecordChangedToMeetCriteria || len(r.Criteria) != 1 || len(r.CriteriaGroups) != 0 || len(r.Paths) != 0 || len(r.StepTargets) != 0 || len(r.Variables) != 0 {
			return nil, false
		}
		if n == 0 {
			entry = r.Criteria[0]
			if entry.Field == "" || entry.Value == "" || !strings.EqualFold(entry.Operation, "equals") || entry.SourceField != "" {
				return nil, false
			}
		} else if !sameCriterion(r.Criteria) {
			return nil, false
		}
		if len(r.Branches) == 1 && len(r.Steps) == 0 && branch(r.Branches[0]) {
			steps := r.Branches[0].Steps
			if len(steps) == 1 && steps[0].Kind == "recordCreate" && plainStep(steps[0]) {
				c := steps[0].RecordCreate
				if create != nil || c.ObjectName == "" || c.InputReference != "" || c.AssignRecordIDToReference != "" || c.StoreOutputAutomatically || len(c.InputAssignments) != 1 {
					return nil, false
				}
				a := c.InputAssignments[0]
				if !strings.EqualFold(a.Field, "Name") || a.LiteralValue == "" || a.Formula != "" || a.SourceField != "" || a.ActionOutput != "" {
					return nil, false
				}
				var target ObjectDefinition
				for key, d := range definitions {
					if strings.EqualFold(key, c.ObjectName) {
						target = d
						break
					}
				}
				f, ok := ResolveFieldName(target, "", "Name")
				if !ok || target.Fields[f].Type != FieldString {
					return nil, false
				}
				create = r
				continue
			}
			if len(steps) == 5 {
				if updates != nil {
					return nil, false
				}
				for _, s := range steps {
					if !literalUpdate(s) {
						return nil, false
					}
				}
				updates = r
				continue
			}
		}
		if len(r.Branches) == 0 && len(r.Steps) == 1 {
			s := r.Steps[0]
			if literalUpdate(s) {
				if last != nil {
					return nil, false
				}
				last = r
				continue
			}
			if s.Kind == "decision" && plainStep(s) && len(s.Branches) == 1 && branch(s.Branches[0]) {
				first := s.Branches[0].Steps
				if len(first) != 2 || !action(first[0]) || first[1].Kind != "decision" || !plainStep(first[1]) || len(first[1].Branches) != 1 || !branch(first[1].Branches[0]) {
					return nil, false
				}
				second := first[1].Branches[0].Steps
				if len(second) != 1 || !action(second[0]) || first[0].Action.ActionName == "" || second[0].Action.ActionName == "" || strings.EqualFold(first[0].Action.ActionName, second[0].Action.ActionName) || actions != nil {
					return nil, false
				}
				actions = r
				continue
			}
		}
		return nil, false
	}
	if create == nil || updates == nil || last == nil || actions == nil {
		return nil, false
	}
	entryName, ok := ResolveFieldName(definition, "", entry.Field)
	if !ok || definition.Fields[entryName].Type != FieldString {
		return nil, false
	}

	steps := updates.Branches[0].Steps
	expected := []FieldType{FieldString, FieldString, FieldDecimal, FieldBoolean, FieldDate}
	seen := map[string]bool{}
	for i, s := range steps {
		name, ok := ResolveFieldName(definition, "", s.FieldUpdates[0].Field)
		if !ok || definition.Fields[name].Type != expected[i] || seen[strings.ToLower(name)] {
			return nil, false
		}
		seen[strings.ToLower(name)] = true
	}
	if !strings.EqualFold(last.Steps[0].FieldUpdates[0].Field, steps[0].FieldUpdates[0].Field) {
		return nil, false
	}
	// Only the two relative name orders captured natively are supported.
	forward := create.Name < actions.Name && actions.Name < updates.Name && updates.Name < last.Name
	reverse := last.Name < updates.Name && updates.Name < actions.Name && actions.Name < create.Name
	if create.Name == "" || !forward && !reverse {
		return nil, false
	}
	plan := []FlowRule{*create, *updates, *last}
	sort.SliceStable(plan, func(i, j int) bool { return plan[i].Name < plan[j].Name })
	return append(plan, *actions), true
}
