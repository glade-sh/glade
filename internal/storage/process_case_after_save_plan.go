package storage

import (
	"regexp"
	"sort"
	"strings"
)

// This entry formula is the two-branch status transition captured in
// process_builder_review_aftercaseformula. Both comparisons in each branch
// must name the same status, so each branch is exactly Status == value AND
// Status changed. Other formulas remain behind the process gate.
var capturedCaseTransitionFormula = regexp.MustCompile(`(?i)^\s*OR\s*\(\s*AND\s*\(\s*ISPICKVAL\s*\(\s*Status\s*,\s*'([^']+)'\s*\)\s*,\s*TEXT\s*\(\s*\$Record__Prior\.Status\s*\)\s*!=\s*"([^"]+)"\s*\)\s*,\s*AND\s*\(\s*ISPICKVAL\s*\(\s*Status\s*,\s*'([^']+)'\s*\)\s*,\s*TEXT\s*\(\s*\$Record__Prior\.Status\s*\)\s*!=\s*"([^"]+)"\s*\)\s*\)\s*$`)

// CapturedCaseAfterSavePlan recognizes the three-flow Case arrangement observed
// in process_builder_review_aftercase and its mixed partial-save companion.
// Other arrangements remain behind the Process Builder warning gate.
func CapturedCaseAfterSavePlan(rules []FlowRule, def ObjectDefinition) ([]FlowRule, bool) {
	eq := strings.EqualFold
	if len(rules) != 3 || !eq(def.APIName, "Case") {
		return nil, false
	}
	plan := append([]FlowRule(nil), rules...)
	sort.Slice(plan, func(i, j int) bool { return plan[i].Name < plan[j].Name })
	for i, r := range plan {
		if !r.Active || !eq(r.ProcessType, "AutoLaunchedFlow") || !eq(r.TriggerType, "RecordAfterSave") || r.TriggerOrder != 0 || r.RunInMode != "" || (i != 2 && (r.Formula != "" || len(r.Variables) != 0)) || r.DoesRequireRecordChangedToMeetCriteria || len(r.Paths) != 0 || len(r.StepTargets) != 0 || len(r.Branches) != 0 || len(r.TextTemplates) != 0 || len(r.ProcessStringConstants) != 0 || len(r.FieldUpdates) != 0 || len(r.Actions) != 0 || len(r.RecordCreates) != 0 || len(r.RecordDeletes) != 0 || len(r.ApexPlugins) != 0 {
			return nil, false
		}
	}
	a, b, c := plan[0], plan[1], plan[2]
	transition := c.Formula != "" || len(c.Variables) != 0
	if transition {
		m := capturedCaseTransitionFormula.FindStringSubmatch(c.Formula)
		if len(m) != 5 || m[1] != m[2] || m[3] != m[4] || m[1] == m[3] || len(c.Variables) != 2 {
			return nil, false
		}
		seen := map[string]bool{}
		types := map[string]bool{}
		for _, v := range c.Variables {
			if v.Name == "" || seen[strings.ToLower(v.Name)] || !v.IsCollection || !v.IsInput || !v.IsOutput || v.DefaultValue != nil || (!eq(v.DataType, "String") && !eq(v.DataType, "SObject")) || (eq(v.DataType, "SObject") && v.ObjectType == "") || (eq(v.DataType, "String") && v.ObjectType != "") {
				return nil, false
			}
			seen[strings.ToLower(v.Name)] = true
			types[strings.ToLower(v.DataType)] = true
		}
		if !types["string"] || !types["sobject"] {
			return nil, false
		}
		c.Formula = ""
		c.Variables = nil
		c.Criteria = []WorkflowCriteriaItem{{Field: "Status", Operation: "equals", Value: m[1]}, {Field: "Status", Operation: "isChanged", Value: "true"}, {Field: "Status", Operation: "equals", Value: m[3]}}
		c.CriteriaGroups = [][]int{{0, 1}, {2, 1}}
	}
	if len(a.RecordLookups) != 1 || len(b.RecordLookups) != 0 || len(c.RecordLookups) != 1 || len(a.CustomErrors) != 0 || len(b.CustomErrors) != 0 || len(c.CustomErrors) != 2 {
		return nil, false
	}
	if !(a.Name < b.Name && b.Name < c.Name) || !eq(a.RecordTriggerType, "CreateAndUpdate") || !eq(b.RecordTriggerType, "Update") || !eq(c.RecordTriggerType, "Update") || len(a.Criteria) != 5 || len(b.Criteria) != 5 || (!transition && (len(c.Criteria) != 0 || len(c.CriteriaGroups) != 0)) || len(a.CriteriaGroups) != 0 || len(b.CriteriaGroups) != 2 || (len(a.Steps) != 2 && len(a.Steps) != 3) || len(b.Steps) != 1 || len(c.Steps) != 1 {
		return nil, false
	}
	want := [][]int{{0, 1, 2, 4}, {0, 1, 3, 4}}
	for i, g := range want {
		if len(b.CriteriaGroups[i]) != len(g) {
			return nil, false
		}
		for j, v := range g {
			if b.CriteriaGroups[i][j] != v {
				return nil, false
			}
		}
	}
	criterion := func(x WorkflowCriteriaItem, op, value string) bool {
		return x.Field != "" && eq(x.Operation, op) && x.Value == value && x.SourceField == ""
	}
	fieldType := func(name string, typ FieldType) bool {
		name, ok := ResolveFieldName(def, "", name)
		return ok && def.Fields[name].Type == typ && eq(def.Fields[name].DisplayType, string(typ))
	}
	caseTextCriterion := func(name string) bool {
		if fieldType(name, FieldString) {
			return true
		}
		name, ok := ResolveFieldName(def, "", name)
		if !ok {
			return false
		}
		f := def.Fields[name]
		return f.Type == FieldCalculated && eq(f.DisplayType, string(FieldString)) && eq(strings.TrimSpace(f.Formula), "RecordType.DeveloperName")
	}
	ac, bc := a.Criteria, b.Criteria
	if !criterion(ac[0], "equals", ac[0].Value) || ac[0].Value == "" || !fieldType(ac[0].Field, FieldPicklist) || !eq(ac[1].Field, "Status") || !criterion(ac[1], "equals", ac[1].Value) || ac[1].Value == "" || !criterion(ac[2], "equals", "false") || !fieldType(ac[2].Field, FieldBoolean) || !criterion(ac[3], "isNull", "true") || !fieldType(ac[3].Field, FieldDateTime) || !criterion(ac[4], "equals", ac[4].Value) || ac[4].Value == "" || !caseTextCriterion(ac[4].Field) {
		return nil, false
	}
	if !eq(bc[0].Field, "Status") || !criterion(bc[0], "equals", bc[0].Value) || bc[0].Value == "" || bc[0].Value == ac[1].Value || !criterion(bc[1], "equals", "true") || !fieldType(bc[1].Field, FieldBoolean) || !eq(bc[2].Field, "Status") || !criterion(bc[2], "isChanged", "true") || !eq(bc[3].Field, bc[1].Field) || !criterion(bc[3], "isChanged", "true") || !eq(bc[4].Field, ac[4].Field) || !criterion(bc[4], "equals", ac[4].Value) {
		return nil, false
	}
	plain := func(s FlowStep) bool {
		return s.FaultTarget == "" && len(s.FaultBranch) == 0 && s.Target == "" && s.VisitKey == ""
	}
	lookup := func(s FlowStep, obj string, count int) bool {
		if s.Kind != "recordLookup" || !plain(s) {
			return false
		}
		l := s.RecordLookup
		sortOK := l.SortField == "" && l.SortOrder == "" || eq(obj, "Group") && eq(l.SortField, "Id") && eq(l.SortOrder, "Asc")
		return eq(l.ObjectName, obj) && eq(l.CriteriaLogic, "and") && len(l.Criteria) == count && l.GetFirstRecordOnly && l.StoreOutputAutomatically && !l.AssignNullValuesIfNoRecordsFound && sortOK && l.OutputReference == "" && len(l.QueriedFields) == 0 && len(l.OutputAssignments) == 0
	}
	if !lookup(a.Steps[0], "Group", 2) || a.Steps[len(a.Steps)-1].Kind != "fieldUpdate" || !plain(a.Steps[len(a.Steps)-1]) || len(a.Steps[len(a.Steps)-1].FieldUpdates) != 1 {
		return nil, false
	}
	lc := a.Steps[0].RecordLookup.Criteria
	if !eq(lc[0].Field, "Type") || !criterion(lc[0], "equals", "Queue") || !eq(lc[1].Field, "DeveloperName") || !criterion(lc[1], "equals", lc[1].Value) || lc[1].Value == "" {
		return nil, false
	}
	u := a.Steps[len(a.Steps)-1].FieldUpdates[0]
	if !eq(u.Field, "Status") || u.LiteralValue != ac[1].Value || u.SourceField != "" || u.Formula != "" || u.ActionOutput != "" {
		return nil, false
	}
	if len(a.Steps) == 3 {
		assign := a.Steps[1]
		if !transition || !eq(a.Steps[0].RecordLookup.SortField, "Id") || !eq(a.Steps[0].RecordLookup.SortOrder, "Asc") || assign.Kind != "fieldUpdate" || !plain(assign) || len(assign.FieldUpdates) != 1 {
			return nil, false
		}
		owner := assign.FieldUpdates[0]
		if !eq(owner.Field, "OwnerId") || !eq(owner.SourceField, a.Steps[0].RecordLookup.Name+".Id") || owner.LiteralValue != "" || owner.Formula != "" || owner.ActionOutput != "" {
			return nil, false
		}
		update := a.Steps[2]
		update.FieldUpdates = []WorkflowFieldUpdate{owner, u}
		a.Steps = []FlowStep{a.Steps[0], update}
	} else if transition || a.Steps[0].RecordLookup.SortField != "" || a.Steps[0].RecordLookup.SortOrder != "" {
		return nil, false
	}
	apex := func(s FlowStep, n int) bool {
		return s.Kind == "action" && plain(s) && eq(s.Action.ActionType, "apex") && s.Action.ActionName != "" && len(s.Action.Inputs) == n && len(s.Action.Outputs) == 0
	}
	if !apex(b.Steps[0], 1) {
		return nil, false
	}
	in := b.Steps[0].Action.Inputs[0]
	if !eq(in.Name, "caseIds") || !eq(in.Field, in.Name) || !eq(in.SourceField, "Id") || in.LiteralValue != "" || in.Formula != "" || in.ActionOutput != "" {
		return nil, false
	}
	decision := func(s FlowStep) bool {
		return s.Kind == "decision" && plain(s) && len(s.Branches) == 2 && !s.Branches[0].Default && s.Branches[0].Formula == "" && len(s.Branches[0].Criteria) == 1 && s.Branches[1].Default && s.Branches[1].Formula == "" && len(s.Branches[1].Criteria) == 0 && len(s.Branches[1].Steps) == 1
	}
	customError := func(s FlowStep) bool {
		if s.Kind != "customError" || !plain(s) || len(s.CustomError.Messages) != 1 {
			return false
		}
		m := s.CustomError.Messages[0]
		return m.Message != "" && !strings.Contains(m.Message, "{!") && m.Field == "" && len(m.Formulas) == 0 && len(m.NumberConstants) == 0 && len(m.ResourceFormulas) == 0
	}
	d1 := c.Steps[0]
	if !decision(d1) || !customError(d1.Branches[1].Steps[0]) || len(d1.Branches[0].Steps) != 1 {
		return nil, false
	}
	first := d1.Branches[0].Criteria[0]
	name, ok := ResolveFieldName(def, "", first.Field)
	if !ok || !criterion(first, "notEqual", "") {
		return nil, false
	}
	fd := def.Fields[name]
	if fd.Type != FieldReference || len(fd.ReferenceTo) != 1 || !eq(fd.ReferenceTo[0], "Contact") || !strings.HasSuffix(strings.ToLower(name), "__c") {
		return nil, false
	}
	d2 := d1.Branches[0].Steps[0]
	if !decision(d2) || !customError(d2.Branches[1].Steps[0]) || len(d2.Branches[0].Steps) != 2 {
		return nil, false
	}
	second := d2.Branches[0].Criteria[0]
	relationship := name[:len(name)-3] + "__r.Email"
	if !eq(second.Field, relationship) || !criterion(second, "notEqual", "") {
		return nil, false
	}
	st := d2.Branches[0].Steps
	if !lookup(st[0], "Case", 1) || !apex(st[1], 2) {
		return nil, false
	}
	cl := st[0].RecordLookup.Criteria[0]
	if !eq(cl.Field, "Id") || !eq(cl.Operation, "equals") || !eq(cl.SourceField, "Id") || cl.Value != "" {
		return nil, false
	}
	x, y := st[1].Action.Inputs[0], st[1].Action.Inputs[1]
	if !eq(x.Name, "caseStatus") || !eq(x.Field, x.Name) || !fieldType(x.SourceField, FieldPicklist) || eq(x.SourceField, ac[0].Field) || !eq(y.Name, "recordId") || !eq(y.Field, y.Name) || !eq(y.SourceField, "Id") || x.LiteralValue != "" || x.Formula != "" || x.ActionOutput != "" || y.LiteralValue != "" || y.Formula != "" || y.ActionOutput != "" {
		return nil, false
	}
	plan[0], plan[2] = a, c
	return plan, true
}
