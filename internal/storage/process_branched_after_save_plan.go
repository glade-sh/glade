package storage

import (
	"reflect"
	"regexp"
	"sort"
	"strings"
)

var capturedBranchedInputName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// CapturedBranchedAfterSavePlan recognizes the two captured arrangements with
// a share decision, User alert, five alternative date updates and one related
// update. Inputs must already retain the captured source filter bindings.
func CapturedBranchedAfterSavePlan(rules []FlowRule, def ObjectDefinition, definitions map[string]ObjectDefinition) ([]FlowRule, bool) {
	eq := strings.EqualFold
	if len(rules) != 4 || !strings.HasSuffix(strings.ToLower(def.APIName), "__c") {
		return nil, false
	}
	plan := append([]FlowRule(nil), rules...)
	sort.Slice(plan, func(i, j int) bool { return plan[i].Name < plan[j].Name })
	for _, r := range plan {
		if !r.Active || !eq(r.ProcessType, "AutoLaunchedFlow") || !eq(r.TriggerType, "RecordAfterSave") || r.TriggerOrder != 0 || r.RunInMode != "" || r.Formula != "" || r.DoesRequireRecordChangedToMeetCriteria || len(r.Paths) != 0 || len(r.StepTargets) != 0 || len(r.Variables) != 0 || len(r.TextTemplates) != 0 || len(r.ProcessStringConstants) != 0 {
			return nil, false
		}
	}
	a, b := plan[0], plan[1]
	if len(a.RecordCreates) != 1 || len(a.RecordDeletes) != 1 || len(a.RecordLookups) != 0 || len(a.CustomErrors) != 0 || len(b.RecordCreates) != 0 || len(b.RecordDeletes) != 0 || len(b.RecordLookups) != 0 || len(b.CustomErrors) != 0 {
		return nil, false
	}
	var dates, related FlowRule
	person := false
	if len(plan[2].Branches) == 5 && len(plan[3].Steps) == 1 {
		dates, related = plan[2], plan[3]
	} else if len(plan[3].Branches) == 5 && len(plan[2].Steps) == 1 {
		dates, related = plan[3], plan[2]
		person = true
	} else {
		return nil, false
	}
	if !(plan[0].Name < plan[1].Name && plan[1].Name < plan[2].Name && plan[2].Name < plan[3].Name) || !eq(a.RecordTriggerType, "Update") || !eq(b.RecordTriggerType, "CreateAndUpdate") || !eq(dates.RecordTriggerType, "CreateAndUpdate") || !eq(related.RecordTriggerType, "CreateAndUpdate") || len(a.Criteria) != 2 || len(b.Criteria) != 1 || len(dates.Criteria) != 2 || len(related.Criteria) != 3 || len(a.CriteriaGroups) != 0 || len(b.CriteriaGroups) != 0 || len(related.CriteriaGroups) != 0 || len(dates.CriteriaGroups) != 2 || len(dates.CriteriaGroups[0]) != 1 || dates.CriteriaGroups[0][0] != 0 || len(dates.CriteriaGroups[1]) != 1 || dates.CriteriaGroups[1][0] != 1 || len(a.Steps) != 1 || len(b.Steps) != 1 || len(related.Steps) != 1 || len(dates.Steps) != 0 || len(a.Branches) != 0 || len(b.Branches) != 0 || len(related.Branches) != 0 {
		return nil, false
	}
	plain := func(s FlowStep) bool {
		return s.FaultTarget == "" && len(s.FaultBranch) == 0 && s.Target == "" && s.VisitKey == ""
	}
	crit := func(c WorkflowCriteriaItem, op, value string) bool {
		return c.Field != "" && eq(c.Operation, op) && c.Value == value && c.SourceField == ""
	}
	field := func(name string, typ FieldType) bool {
		name, ok := ResolveFieldName(def, "", name)
		return ok && def.Fields[name].Type == typ && eq(def.Fields[name].DisplayType, string(typ))
	}
	ref := func(name, target string) bool {
		name, ok := ResolveFieldName(def, "", name)
		if !ok {
			return false
		}
		f := def.Fields[name]
		return f.Type == FieldReference && len(f.ReferenceTo) == 1 && eq(f.ReferenceTo[0], target)
	}
	if !crit(a.Criteria[0], "isNull", "false") || !crit(a.Criteria[1], "isChanged", "true") || !eq(a.Criteria[0].Field, a.Criteria[1].Field) || !crit(b.Criteria[0], "isNull", "false") || !crit(dates.Criteria[0], "isChanged", "true") || !crit(dates.Criteria[1], "equals", dates.Criteria[1].Value) || dates.Criteria[1].Value == "" || !eq(dates.Criteria[0].Field, dates.Criteria[1].Field) || !field(dates.Criteria[0].Field, FieldPicklist) || !crit(related.Criteria[0], "isChanged", "false") || !eq(related.Criteria[0].Field, dates.Criteria[0].Field) || !crit(related.Criteria[1], "equals", related.Criteria[1].Value) || !eq(related.Criteria[1].Field, dates.Criteria[0].Field) || related.Criteria[1].Value == "" || related.Criteria[1].Value == dates.Criteria[1].Value || !crit(related.Criteria[2], "isChanged", "true") {
		return nil, false
	}
	da := a.Steps[0]
	if da.Kind != "decision" || !plain(da) || len(da.Branches) != 2 || da.Branches[0].Default || !da.Branches[1].Default || da.Branches[0].Formula != "" || da.Branches[1].Formula != "" || len(da.Branches[0].Criteria) != 1 || (len(da.Branches[0].Steps) != 1 && len(da.Branches[0].Steps) != 2) || len(da.Branches[1].Criteria) != 0 || len(da.Branches[1].Steps) != 1 {
		return nil, false
	}
	del, create := da.Branches[0].Steps[0], da.Branches[1].Steps[0]
	replacement := len(da.Branches[0].Steps) == 2
	if replacement && (!person || !reflect.DeepEqual(da.Branches[0].Steps[1], create)) {
		return nil, false
	}
	if del.Kind != "recordDelete" || create.Kind != "recordCreate" || !plain(del) || !plain(create) {
		return nil, false
	}
	target := def.APIName[:len(def.APIName)-3] + "__Share"
	dc, cc := del.RecordDelete, create.RecordCreate
	if !eq(dc.ObjectName, target) || !eq(cc.ObjectName, target) || dc.InputReference != "" || cc.InputReference != "" || cc.AssignRecordIDToReference != "" || cc.StoreOutputAutomatically != replacement || len(dc.Criteria) != 3 || len(cc.InputAssignments) != 4 {
		return nil, false
	}
	checks := map[string]WorkflowCriteriaItem{}
	for _, x := range dc.Criteria {
		k := strings.ToLower(x.Field)
		if _, ok := checks[k]; ok {
			return nil, false
		}
		checks[k] = x
	}
	assigns := map[string]WorkflowFieldUpdate{}
	for _, x := range cc.InputAssignments {
		k := strings.ToLower(x.Field)
		if _, ok := assigns[k]; ok {
			return nil, false
		}
		assigns[k] = x
	}
	parent, recipient, cause := checks["parentid"], checks["userorgroupid"], checks["rowcause"]
	pa, ua, access, ca := assigns["parentid"], assigns["userorgroupid"], assigns["accesslevel"], assigns["rowcause"]
	if !eq(parent.Operation, "equals") || !eq(parent.SourceField, "Id") || parent.Value != "" || !eq(recipient.Operation, "equals") || recipient.Value != "" || !crit(cause, "equals", "Manual") || !eq(pa.SourceField, "Id") || pa.LiteralValue != "" || !ref(ua.SourceField, "User") || ua.LiteralValue != "" || !eq(ca.LiteralValue, "Manual") || ca.SourceField != "" || access.SourceField != "" || access.Formula != "" || access.ActionOutput != "" {
		return nil, false
	}
	for _, x := range cc.InputAssignments {
		if x.Formula != "" || x.ActionOutput != "" {
			return nil, false
		}
	}
	first := da.Branches[0].Criteria[0]
	if person {
		if !ref(a.Criteria[0].Field, "User") || !eq(a.Criteria[0].Field, ua.SourceField) || !eq(first.Field, "$Record__Prior."+ua.SourceField) || !crit(first, "isNull", "false") || !eq(recipient.SourceField, "$Record__Prior."+ua.SourceField) || access.LiteralValue != "Edit" || !eq(b.Criteria[0].Field, "OwnerId") {
			return nil, false
		}
	} else {
		if !field(a.Criteria[0].Field, FieldString) || !eq(first.Field, b.Criteria[0].Field) || !field(first.Field, FieldString) || !crit(first, "isNull", "false") || !eq(recipient.SourceField, ua.SourceField) || access.LiteralValue != "Read" {
			return nil, false
		}
	}
	firstFiltered := len(dates.Branches) == 5 && len(dates.Branches[0].Steps) == 1 && len(dates.Branches[0].Steps[0].FieldUpdates) == 1 && strings.HasPrefix(strings.ToUpper(strings.TrimSpace(dates.Branches[0].Steps[0].FieldUpdates[0].Formula)), "DATEVALUE(")
	if firstFiltered && (!person || !replacement) {
		return nil, false
	}
	if !capturedBranchedDates(dates, related, def) {
		return nil, false
	}
	db := b.Steps[0]
	if db.Kind != "decision" || !plain(db) || len(db.Branches) != 2 || db.Branches[0].Default || !db.Branches[1].Default || strings.ReplaceAll(strings.ToUpper(db.Branches[0].Formula), " ", "") != "(ISNEW())=TRUE" || len(db.Branches[0].Criteria) != 0 || len(db.Branches[0].Steps) != 1 || len(db.Branches[1].Criteria) != 0 || len(db.Branches[1].Steps) != 1 {
		return nil, false
	}
	apex := db.Branches[0].Steps[0]
	if apex.Kind != "action" || !plain(apex) || !eq(apex.Action.ActionType, "apex") || apex.Action.ActionName == "" || len(apex.Action.Inputs) != 1 || len(apex.Action.Outputs) != 0 || !capturedBranchedInputName.MatchString(apex.Action.Inputs[0].Name) || !eq(apex.Action.Inputs[0].SourceField, "$Record") || !eq(apex.Action.Inputs[0].Field, apex.Action.Inputs[0].Name) || apex.Action.Inputs[0].LiteralValue != "" || apex.Action.Inputs[0].Formula != "" || apex.Action.Inputs[0].ActionOutput != "" {
		return nil, false
	}
	nested := db.Branches[1].Steps[0]
	if nested.Kind != "decision" || !plain(nested) || len(nested.Branches) != 1 || nested.Branches[0].Default || nested.Branches[0].Formula != "" || len(nested.Branches[0].Criteria) != 3 || len(nested.Branches[0].Steps) != 1 {
		return nil, false
	}
	nc := nested.Branches[0].Criteria
	changed := a.Criteria[0].Field
	if person {
		changed = ua.SourceField
	}
	relation := ua.SourceField
	if !strings.HasSuffix(strings.ToLower(relation), "__c") {
		return nil, false
	}
	relation = relation[:len(relation)-3] + "__r.Username"
	if !eq(nc[0].Field, changed) || !crit(nc[0], "isNull", "false") || !eq(nc[1].Field, changed) || !crit(nc[1], "isChanged", "true") || !eq(nc[2].Field, relation) || !crit(nc[2], "notEqual", "{!$User.Username}") {
		return nil, false
	}
	alert := nested.Branches[0].Steps[0]
	if alert.Kind != "action" || !plain(alert) || !eq(alert.Action.ActionType, "emailAlert") || alert.Action.ActionName == "" || len(alert.Action.Outputs) != 0 || len(alert.Action.Inputs) != 3 {
		return nil, false
	}
	inputs := alert.Action.Inputs
	if !eq(inputs[0].Name, "SObjectRowId") || !eq(inputs[0].SourceField, "Id") || inputs[1].Name != CapturedMixedRecipientInput || !eq(inputs[1].Field, "userLookup") || !eq(inputs[1].SourceField, ua.SourceField) || inputs[2].Name != CapturedMixedTemplateInput || inputs[2].LiteralValue == "" || inputs[0].LiteralValue != "" || inputs[0].Formula != "" || inputs[0].ActionOutput != "" || inputs[1].LiteralValue != "" || inputs[1].Formula != "" || inputs[1].ActionOutput != "" || inputs[2].Field != "" || inputs[2].SourceField != "" || inputs[2].Formula != "" || inputs[2].ActionOutput != "" {
		return nil, false
	}
	u := related.Steps[0]
	if u.Kind != "recordUpdate" || !plain(u) || len(u.RecordUpdate.Criteria) != 1 || len(u.RecordUpdate.InputAssignments) != 1 {
		return nil, false
	}
	ur := u.RecordUpdate
	filter, assign := ur.Criteria[0], ur.InputAssignments[0]
	if !eq(filter.Field, "Id") || !eq(filter.Operation, "equals") || filter.Value != "" || !strings.HasPrefix(ur.InputReference, "$Record.") || !eq(filter.SourceField, strings.TrimPrefix(ur.InputReference, "$Record.")+".Id") || assign.LiteralValue != "" || assign.Formula != "" || assign.ActionOutput != "" {
		return nil, false
	}
	if person {
		if !eq(ur.ObjectName, "Account") || !eq(assign.Field, "PersonBirthdate") || !eq(assign.SourceField, related.Criteria[2].Field) || !field(assign.SourceField, FieldDate) || !ref(strings.TrimSuffix(strings.TrimPrefix(ur.InputReference, "$Record."), "__r")+"__c", "Account") {
			return nil, false
		}
		targetDate := false
		for name, d := range definitions {
			if eq(name, "Account") {
				n, found := ResolveFieldName(d, "", "PersonBirthdate")
				targetDate = found && d.Fields[n].Type == FieldDate
			}
		}
		if !targetDate {
			return nil, false
		}
	} else {
		rel := strings.TrimPrefix(ur.InputReference, "$Record.")
		if !strings.HasSuffix(strings.ToLower(rel), "__r") || !ref(rel[:len(rel)-3]+"__c", ur.ObjectName) || !strings.HasSuffix(strings.ToLower(ur.ObjectName), "__c") {
			return nil, false
		}
		if !field(related.Criteria[2].Field, FieldBoolean) || !eq(assign.Field, "Name") || !eq(assign.SourceField, a.Criteria[0].Field) {
			return nil, false
		}
		ok := false
		for name, d := range definitions {
			if eq(name, ur.ObjectName) {
				n, found := ResolveFieldName(d, "", "Name")
				ok = found && d.Fields[n].Type == FieldString
			}
		}
		if !ok {
			return nil, false
		}
	}
	return []FlowRule{plan[0], plan[2], plan[3], plan[1]}, true
}

func capturedBranchedDates(dates, related FlowRule, def ObjectDefinition) bool {
	eq := strings.EqualFold
	// The second native arrangement places the filtered branch first and
	// reverses its two entry literals. Its outcomes are disjoint; validate the
	// same dependencies in canonical order without changing execution order.
	if len(dates.Branches) == 5 && len(dates.Branches[0].Steps) == 1 && len(dates.Branches[0].Steps[0].FieldUpdates) == 1 && strings.HasPrefix(strings.ToUpper(strings.TrimSpace(dates.Branches[0].Steps[0].FieldUpdates[0].Formula)), "DATEVALUE(") {
		if len(dates.Criteria) != 2 || len(related.Criteria) != 3 || len(dates.Branches[0].Criteria) != 2 || len(dates.Branches[3].Criteria) != 1 || dates.Criteria[1].Value != dates.Branches[0].Criteria[0].Value || related.Criteria[1].Value != dates.Branches[3].Criteria[0].Value {
			return false
		}
		original := dates.Branches
		dates.Branches = []FlowBranch{original[3], original[4], original[2], original[1], original[0]}
		dates.Criteria = append([]WorkflowCriteriaItem(nil), dates.Criteria...)
		related.Criteria = append([]WorkflowCriteriaItem(nil), related.Criteria...)
		dates.Criteria[1].Value, related.Criteria[1].Value = related.Criteria[1].Value, dates.Criteria[1].Value
	}
	field := func(name string, typ FieldType) bool {
		name, ok := ResolveFieldName(def, "", name)
		return ok && def.Fields[name].Type == typ && eq(def.Fields[name].DisplayType, string(typ))
	}
	p := regexp.MustCompile(`(?i)^IF\(AND\(TEXT\(([A-Za-z_][A-Za-z0-9_]*)\)="([A-Za-z0-9_ -]+)",(.+)\),DATEVALUE\(([A-Za-z_][A-Za-z0-9_]*)\),\$Record__Prior\.([A-Za-z_][A-Za-z0-9_]*)\)$`)
	prior := regexp.MustCompile(`(?i)^TEXT\(\$Record__Prior\.([A-Za-z_][A-Za-z0-9_]*)\)="([A-Za-z0-9_ -]+)"$`)
	date := regexp.MustCompile(`(?i)^DATEVALUE\(([A-Za-z_][A-Za-z0-9_]*)\)$`)
	var current []string
	var previous [][]string
	seen := map[string]bool{}
	timeField := ""
	for i, b := range dates.Branches {
		if b.Default || b.Formula != "" || len(b.Steps) != 1 || len(b.Criteria) != 1 && !(i == 4 && len(b.Criteria) == 2) {
			return false
		}
		c := b.Criteria[0]
		if !eq(c.Field, dates.Criteria[0].Field) || !eq(c.Operation, "equals") || c.Value == "" || c.SourceField != "" {
			return false
		}
		s := b.Steps[0]
		if s.Kind != "fieldUpdate" || s.FaultTarget != "" || len(s.FaultBranch) != 0 || len(s.FieldUpdates) != 1 {
			return false
		}
		u := s.FieldUpdates[0]
		if !field(u.Field, FieldDate) || seen[strings.ToLower(u.Field)] || u.LiteralValue != "" || u.SourceField != "" || u.ActionOutput != "" || u.Formula == "" {
			return false
		}
		seen[strings.ToLower(u.Field)] = true
		f := capturedBranchedFormulaWhitespace(u.Formula)
		if i == 4 {
			if len(b.Criteria) != 2 || !eq(b.Criteria[1].Field, u.Field) || !eq(b.Criteria[1].Operation, "isNull") || b.Criteria[1].Value != "true" || b.Criteria[1].SourceField != "" {
				return false
			}
			m := date.FindStringSubmatch(f)
			if len(m) != 2 || !field(m[1], FieldDateTime) || eq(m[1], timeField) || c.Value != related.Criteria[1].Value {
				return false
			}
			continue
		}
		m := p.FindStringSubmatch(f)
		if len(m) != 6 || !eq(m[1], dates.Criteria[0].Field) || m[2] != c.Value || !eq(m[5], u.Field) || !field(m[4], FieldDateTime) {
			return false
		}
		if timeField == "" {
			timeField = m[4]
		} else if !eq(timeField, m[4]) {
			return false
		}
		part := m[3]
		if strings.HasPrefix(strings.ToUpper(part), "OR(") && strings.HasSuffix(part, ")") {
			part = part[3 : len(part)-1]
		}
		pieces := strings.Split(part, ",")
		if len(pieces) != []int{3, 4, 3, 1}[i] {
			return false
		}
		var vals []string
		for _, piece := range pieces {
			x := prior.FindStringSubmatch(piece)
			if len(x) != 3 || !eq(x[1], dates.Criteria[0].Field) {
				return false
			}
			vals = append(vals, x[2])
		}
		current = append(current, m[2])
		previous = append(previous, vals)
	}
	base := previous[0]
	distinct := func(x string) bool { return x != base[0] && x != base[1] && x != base[2] && x != current[0] }
	return current[0] == dates.Criteria[1].Value && base[0] == related.Criteria[1].Value && distinct(current[1]) && current[2] == base[2] && current[3] == base[1] && previous[1][0] == base[0] && previous[1][1] == base[1] && previous[1][2] == base[2] && previous[1][3] == current[0] && previous[2][0] == base[0] && previous[2][1] == base[1] && previous[2][2] == base[2] && previous[3][0] == base[0]
}

// Ignore syntax whitespace in the captured multiline date formulas without
// changing the quoted status values used by those formulas.
func capturedBranchedFormulaWhitespace(formula string) string {
	var out strings.Builder
	quoted, escaped := false, false
	for _, r := range formula {
		if !escaped && r == '"' {
			quoted = !quoted
		}
		if quoted || r != ' ' && r != '\n' && r != '\r' && r != '\t' {
			out.WriteRune(r)
		}
		if r == '\\' && !escaped {
			escaped = true
		} else {
			escaped = false
		}
	}
	return out.String()
}
