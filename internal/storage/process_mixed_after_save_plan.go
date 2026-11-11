package storage

import (
	"regexp"
	"strings"
)

// CapturedMixedRecipientInput and CapturedMixedTemplateInput are private loader
// bindings for the captured single-recipient workflow alert.
const CapturedMixedRecipientInput = "__glade_captured_recipient"
const CapturedMixedTemplateInput = "__glade_captured_template"

// CapturedMixedAfterSavePlan recognizes the exact four-flow arrangement in
// process_builder_review_afteraccept and process_builder_review_afteruser.
// Other graph, condition, formula, or name-order shapes remain unrun.
func CapturedMixedAfterSavePlan(rules []FlowRule, def ObjectDefinition, definitions map[string]ObjectDefinition) ([]FlowRule, bool) {
	if len(rules) != 4 {
		return nil, false
	}
	var create, actions, updates, last *FlowRule
	eq := strings.EqualFold
	for i := range rules {
		r := &rules[i]
		if !r.Active || !eq(r.TriggerType, "RecordAfterSave") || r.TriggerOrder != 0 || r.RunInMode != "" || r.Formula != "" || r.DoesRequireRecordChangedToMeetCriteria || len(r.Paths) != 0 || len(r.StepTargets) != 0 || len(r.Variables) != 0 {
			return nil, false
		}
		switch {
		case eq(r.RecordTriggerType, "Update") && len(r.Criteria) == 2 && len(r.Branches) == 1 && len(r.Steps) == 0:
			if create != nil {
				return nil, false
			}
			create = r
		case eq(r.RecordTriggerType, "CreateAndUpdate") && len(r.Criteria) == 1 && len(r.Steps) == 1 && len(r.Branches) == 0:
			if actions != nil {
				return nil, false
			}
			actions = r
		case eq(r.RecordTriggerType, "CreateAndUpdate") && len(r.Criteria) == 2 && len(r.Branches) == 1 && len(r.Steps) == 0:
			if updates != nil {
				return nil, false
			}
			updates = r
		case eq(r.RecordTriggerType, "CreateAndUpdate") && len(r.Criteria) == 3 && len(r.Steps) == 1 && len(r.Branches) == 0:
			if last != nil {
				return nil, false
			}
			last = r
		default:
			return nil, false
		}
	}
	if create == nil || actions == nil || updates == nil || last == nil || !(create.Name < actions.Name && actions.Name < updates.Name && updates.Name < last.Name) {
		return nil, false
	}
	condition := func(c WorkflowCriteriaItem, op, value string) bool {
		return eq(c.Operation, op) && eq(c.Value, value) && c.Field != "" && c.SourceField == ""
	}
	if len(create.CriteriaGroups) != 0 || !condition(create.Criteria[0], "isNull", "false") || !condition(create.Criteria[1], "isChanged", "true") || !eq(create.Criteria[0].Field, create.Criteria[1].Field) {
		return nil, false
	}
	if len(actions.CriteriaGroups) != 0 || !condition(actions.Criteria[0], "isNull", "false") {
		return nil, false
	}
	if !condition(updates.Criteria[0], "isChanged", "true") || !eq(updates.Criteria[1].Operation, "equals") || updates.Criteria[1].Value == "" || updates.Criteria[1].SourceField != "" || !eq(updates.Criteria[0].Field, updates.Criteria[1].Field) || len(updates.CriteriaGroups) != 2 || len(updates.CriteriaGroups[0]) != 1 || len(updates.CriteriaGroups[1]) != 1 || updates.CriteriaGroups[0][0] != 0 || updates.CriteriaGroups[1][0] != 1 {
		return nil, false
	}
	if len(last.CriteriaGroups) != 0 || !condition(last.Criteria[0], "isChanged", "false") || !eq(last.Criteria[0].Field, updates.Criteria[0].Field) || !eq(last.Criteria[1].Field, updates.Criteria[0].Field) || !eq(last.Criteria[1].Operation, "equals") || last.Criteria[1].Value == "" || last.Criteria[1].SourceField != "" || !condition(last.Criteria[2], "isChanged", "true") {
		return nil, false
	}
	fieldType := func(name string, typ FieldType) bool {
		n, ok := ResolveFieldName(def, "", name)
		return ok && def.Fields[n].Type == typ && eq(def.Fields[n].DisplayType, string(typ))
	}
	if !fieldType(create.Criteria[0].Field, FieldString) || !fieldType(actions.Criteria[0].Field, FieldString) || !fieldType(updates.Criteria[0].Field, FieldPicklist) || !fieldType(last.Criteria[2].Field, FieldBoolean) {
		return nil, false
	}
	plain := func(s FlowStep) bool { return s.FaultTarget == "" && len(s.FaultBranch) == 0 }
	branch := func(b FlowBranch) bool {
		return !b.Default && b.Formula == "" && len(b.Criteria) == 1 && eq(b.Criteria[0].Field, create.Criteria[0].Field) && eq(b.Criteria[0].Operation, "equals") && b.Criteria[0].Value != "" && b.Criteria[0].SourceField == ""
	}
	cb, ub := create.Branches[0], updates.Branches[0]
	if !branch(cb) || !branch(ub) || cb.Criteria[0].Value != ub.Criteria[0].Value || len(cb.Steps) != 1 || cb.Steps[0].Kind != "recordCreate" || !plain(cb.Steps[0]) {
		return nil, false
	}
	c := cb.Steps[0].RecordCreate
	if c.ObjectName == "" || c.InputReference != "" || c.AssignRecordIDToReference != "" || c.StoreOutputAutomatically || len(c.InputAssignments) != 1 {
		return nil, false
	}
	ca := c.InputAssignments[0]
	if !eq(ca.Field, "Name") || ca.LiteralValue == "" || ca.Formula != "" || ca.SourceField != "" || ca.ActionOutput != "" {
		return nil, false
	}
	targetOK := false
	for name, d := range definitions {
		if eq(name, c.ObjectName) {
			n, ok := ResolveFieldName(d, "", "Name")
			targetOK = ok && d.Fields[n].Type == FieldString
			break
		}
	}
	if !targetOK {
		return nil, false
	}
	if len(ub.Steps) != 5 {
		return nil, false
	}
	legacy := last.Criteria[1].Value == updates.Criteria[1].Value
	pattern := regexp.MustCompile(`(?i)^IF\(AND\(TEXT\(([A-Za-z_][A-Za-z0-9_]*)\)="([^"\\]+)",OR\(TEXT\(([A-Za-z_][A-Za-z0-9_]*)\)="([^"\\]+)",TEXT\(([A-Za-z_][A-Za-z0-9_]*)\)="([^"\\]+)"\)\),DATEVALUE\(CreatedDate\),NULL\)$`)
	priorPattern := regexp.MustCompile(`(?i)^IF\(AND\(TEXT\(([A-Za-z_][A-Za-z0-9_]*)\)="([A-Za-z0-9_ -]+)",(.+)\),DATEVALUE\(([A-Za-z_][A-Za-z0-9_]*)\),\$Record__Prior\.([A-Za-z_][A-Za-z0-9_]*)\)$`)
	priorCondition := regexp.MustCompile(`(?i)^TEXT\(\$Record__Prior\.([A-Za-z_][A-Za-z0-9_]*)\)="([A-Za-z0-9_ -]+)"$`)
	dateValue := regexp.MustCompile(`(?i)^DATEVALUE\(([A-Za-z_][A-Za-z0-9_]*)\)$`)
	var currentValues []string
	var priorValues [][]string
	timeField := ""
	seen := map[string]bool{}
	for i, step := range ub.Steps {
		if step.Kind != "fieldUpdate" || !plain(step) || len(step.FieldUpdates) != 1 {
			return nil, false
		}
		u := step.FieldUpdates[0]
		if u.LiteralValue != "" || u.SourceField != "" || u.ActionOutput != "" || u.Formula == "" || !fieldType(u.Field, FieldDate) || seen[strings.ToLower(u.Field)] {
			return nil, false
		}
		seen[strings.ToLower(u.Field)] = true
		formula := strings.ReplaceAll(u.Formula, " ", "")
		if legacy {
			if i == 4 {
				if !eq(formula, "DATEVALUE(CreatedDate)") {
					return nil, false
				}
				continue
			}
			m := pattern.FindStringSubmatch(formula)
			if len(m) != 7 || !eq(m[1], updates.Criteria[0].Field) || !eq(m[1], m[3]) || !eq(m[1], m[5]) || m[2] != updates.Criteria[1].Value || m[2] != m[6] || m[2] == m[4] {
				return nil, false
			}
			continue
		}
		if i == 4 {
			m := dateValue.FindStringSubmatch(formula)
			if len(m) != 2 || !fieldType(m[1], FieldDateTime) || eq(m[1], timeField) {
				return nil, false
			}
			continue
		}
		m := priorPattern.FindStringSubmatch(formula)
		if len(m) != 6 || !eq(m[1], updates.Criteria[0].Field) || !eq(m[5], u.Field) || !fieldType(m[4], FieldDateTime) {
			return nil, false
		}
		if timeField == "" {
			timeField = m[4]
		} else if !eq(timeField, m[4]) {
			return nil, false
		}
		condition := m[3]
		if strings.HasPrefix(strings.ToUpper(condition), "OR(") && strings.HasSuffix(condition, ")") {
			condition = condition[3 : len(condition)-1]
		}
		parts := strings.Split(condition, ",")
		if len(parts) != []int{3, 4, 3, 1}[i] {
			return nil, false
		}
		var values []string
		for _, part := range parts {
			c := priorCondition.FindStringSubmatch(part)
			if len(c) != 3 || !eq(c[1], updates.Criteria[0].Field) {
				return nil, false
			}
			values = append(values, c[2])
		}
		currentValues = append(currentValues, m[2])
		priorValues = append(priorValues, values)
	}
	if !legacy {
		base := priorValues[0]
		distinct := func(value string) bool {
			return value != base[0] && value != base[1] && value != base[2] && value != currentValues[0]
		}
		firstShape := currentValues[1] == base[1] && distinct(currentValues[3])
		secondShape := distinct(currentValues[1]) && currentValues[3] == base[1]
		if currentValues[0] != updates.Criteria[1].Value || base[0] != last.Criteria[1].Value || (!firstShape && !secondShape) || currentValues[2] != base[2] || len(priorValues[1]) != 4 || priorValues[1][0] != base[0] || priorValues[1][1] != base[1] || priorValues[1][2] != base[2] || priorValues[1][3] != currentValues[0] || priorValues[2][0] != base[0] || priorValues[2][1] != base[1] || priorValues[2][2] != base[2] || priorValues[3][0] != base[0] {
			return nil, false
		}
	}

	ls := last.Steps[0]
	if ls.Kind != "fieldUpdate" || !plain(ls) || len(ls.FieldUpdates) != 1 {
		return nil, false
	}
	lu := ls.FieldUpdates[0]
	if !eq(lu.Field, actions.Criteria[0].Field) || lu.LiteralValue == "" || lu.Formula != "" || lu.SourceField != "" || lu.ActionOutput != "" {
		return nil, false
	}
	ds := actions.Steps[0]
	if ds.Kind != "decision" || !plain(ds) || len(ds.Branches) != 1 {
		return nil, false
	}
	db := ds.Branches[0]
	if db.Default || strings.ReplaceAll(strings.ToUpper(db.Formula), " ", "") != "(ISNEW())=FALSE" || len(db.Criteria) != 0 || len(db.Steps) != 2 {
		return nil, false
	}
	alert, nested := db.Steps[0], db.Steps[1]
	if alert.Kind != "action" || !plain(alert) || !eq(alert.Action.ActionType, "emailAlert") || alert.Action.ActionName == "" || len(alert.Action.Outputs) != 0 || len(alert.Action.Inputs) < 1 || len(alert.Action.Inputs) > 3 {
		return nil, false
	}
	first := alert.Action.Inputs[0]
	if !eq(first.Name, "SObjectRowId") || !eq(first.SourceField, "Id") || first.LiteralValue != "" || first.Formula != "" {
		return nil, false
	}
	if len(alert.Action.Inputs) > 1 {
		if len(alert.Action.Inputs) != 3 || alert.Action.Inputs[1].Name != CapturedMixedRecipientInput || alert.Action.Inputs[2].Name != CapturedMixedTemplateInput {
			return nil, false
		}
		recipient, template := alert.Action.Inputs[1], alert.Action.Inputs[2]
		name, found := ResolveFieldName(def, "", recipient.SourceField)
		if !found || recipient.LiteralValue != "" || recipient.Formula != "" || recipient.ActionOutput != "" || template.LiteralValue == "" || template.Field != "" || template.SourceField != "" || template.Formula != "" || template.ActionOutput != "" {
			return nil, false
		}
		fd := def.Fields[name]
		if eq(recipient.Field, "email") {
			if fd.Type != FieldString || !eq(fd.DisplayType, "EMAIL") {
				return nil, false
			}
		} else if eq(recipient.Field, "userLookup") {
			if fd.Type != FieldReference || len(fd.ReferenceTo) != 1 || !eq(fd.ReferenceTo[0], "User") {
				return nil, false
			}
		} else {
			return nil, false
		}
	}
	if nested.Kind != "decision" || !plain(nested) || len(nested.Branches) != 1 || !branch(nested.Branches[0]) || nested.Branches[0].Criteria[0].Value != cb.Criteria[0].Value || len(nested.Branches[0].Steps) != 1 {
		return nil, false
	}
	a := nested.Branches[0].Steps[0]
	if a.Kind != "action" || !plain(a) || !eq(a.Action.ActionType, "apex") || a.Action.ActionName == "" || len(a.Action.Inputs) != 1 || len(a.Action.Outputs) != 0 || !eq(a.Action.Inputs[0].SourceField, "$Record") {
		return nil, false
	}
	return []FlowRule{*create, *updates, *last, *actions}, true
}
