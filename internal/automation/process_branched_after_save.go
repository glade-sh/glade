package automation

import (
	"encoding/json"
	"encoding/xml"
	"os"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// processBuilderCapturedBranchedAfterSave restores only the source bindings
// retained by the two captured four-flow graphs. Nothing is installed until
// both the original metadata and the complete compiled plan have been checked.
func processBuilderCapturedBranchedAfterSave(idx *Index, object string, rules []storage.FlowRule, def storage.ObjectDefinition, definitions map[string]storage.ObjectDefinition, features ...[]string) bool {
	if len(rules) != 4 {
		return false
	}
	activeProcesses := 0
	for _, flow := range idx.Flows {
		if strings.EqualFold(flow.ObjectName, object) && flowIsActiveProcess(flow) {
			activeProcesses++
		}
	}
	if activeProcesses != 1 {
		return false
	}
	data, err := json.Marshal(rules)
	if err != nil {
		return false
	}
	var candidate []storage.FlowRule
	if json.Unmarshal(data, &candidate) != nil {
		return false
	}
	sort.Slice(candidate, func(i, j int) bool { return candidate[i].Name < candidate[j].Name })
	a, b := &candidate[0], &candidate[1]
	var dates *storage.FlowRule
	if len(candidate[2].Branches) == 5 {
		dates = &candidate[2]
	} else if len(candidate[3].Branches) == 5 {
		dates = &candidate[3]
	} else {
		return false
	}
	read := func(path string) (flowXML, bool) {
		var raw flowXML
		source, err := os.ReadFile(path)
		if err != nil || xml.Unmarshal(source, &raw) != nil {
			return raw, false
		}
		return raw, true
	}
	raw, ok := read(dates.File)
	if !ok || !strings.EqualFold(raw.Start.FilterLogic, "or") || len(raw.Start.Filters) != 2 || len(raw.Decisions) != 1 || len(raw.Decisions[0].Rules) != 5 || len(raw.RecordUpdates) != 5 || len(raw.Formulas) != 5 || len(raw.Assignments)+len(raw.RecordCreates)+len(raw.RecordDeletes)+len(raw.ActionCalls)+len(raw.RecordLookups)+len(raw.CustomErrors)+len(raw.Variables)+len(raw.Constants)+len(raw.TextTemplates)+len(raw.Subflows)+len(raw.Loops)+len(raw.Screens) != 0 {
		return false
	}
	filteredFirst := len(dates.Branches) == 5 && len(dates.Branches[0].Steps) == 1 && len(dates.Branches[0].Steps[0].FieldUpdates) == 1 && strings.HasPrefix(strings.ToUpper(strings.TrimSpace(dates.Branches[0].Steps[0].FieldUpdates[0].Formula)), "DATEVALUE(")
	for i, branch := range dates.Branches {
		if len(branch.Steps) != 1 || len(branch.Steps[0].FieldUpdates) != 1 {
			return false
		}
		update := branch.Steps[0].FieldUpdates[0]
		var found *flowRecordUpdateXML
		for j := range raw.RecordUpdates {
			if strings.EqualFold(raw.RecordUpdates[j].Name, update.Name) {
				if found != nil {
					return false
				}
				found = &raw.RecordUpdates[j]
			}
		}
		if found == nil || found.Object != "" || !strings.EqualFold(found.InputReference, "$Record") || len(found.Fields) != 1 || !strings.EqualFold(found.Fields[0].Field, update.Field) || found.Connector.TargetReference != "" || found.FaultConnector.TargetReference != "" || (found.FilterLogic != "" && !strings.EqualFold(found.FilterLogic, "and")) {
			return false
		}
		filtered := i == 4
		if filteredFirst {
			filtered = i == 0
		}
		if !filtered {
			if len(found.Filters) != 0 {
				return false
			}
		} else {
			if len(found.Filters) != 1 {
				return false
			}
			f := found.Filters[0]
			if !strings.EqualFold(f.Field, update.Field) || !strings.EqualFold(f.Operator, "IsNull") || !strings.EqualFold(f.Value.BooleanValue, "true") || f.Value != (flowValueXML{BooleanValue: f.Value.BooleanValue}) {
				return false
			}
			dates.Branches[i].Criteria = append(dates.Branches[i].Criteria, storage.WorkflowCriteriaItem{Field: update.Field, Operation: "isNull", Value: "true"})
		}
	}
	dates.CriteriaGroups = [][]int{{0}, {1}}
	if len(a.Steps) != 1 || len(a.Steps[0].Branches) != 2 || (len(a.Steps[0].Branches[0].Steps) != 1 && len(a.Steps[0].Branches[0].Steps) != 2) || len(a.Steps[0].Branches[1].Steps) != 1 {
		return false
	}
	replacement := len(a.Steps[0].Branches[0].Steps) == 2
	del := &a.Steps[0].Branches[0].Steps[0].RecordDelete
	if len(del.Criteria) != 3 {
		return false
	}
	rawA, ok := read(a.File)
	if !ok || len(rawA.RecordDeletes) != 1 {
		return false
	}
	for i := range del.Criteria {
		c := &del.Criteria[i]
		if !strings.EqualFold(c.Field, "UserOrGroupId") || c.SourceField != "" {
			continue
		}
		found := ""
		for _, f := range rawA.RecordDeletes[0].Filters {
			if strings.EqualFold(f.Field, "UserOrGroupId") {
				if found != "" {
					return false
				}
				found = f.Value.ElementReference
				if f.Value != (flowValueXML{ElementReference: found}) {
					return false
				}
			}
		}
		if !strings.HasPrefix(found, "$Record__Prior.") {
			return false
		}
		c.SourceField = found
	}
	// Verify that no source operands discarded by the ordinary projection can
	// change the captured share operation.
	if len(rawA.RecordCreates) != 1 || len(rawA.Decisions) != 1 || len(rawA.Formulas)+len(rawA.Constants)+len(rawA.Assignments)+len(rawA.RecordUpdates)+len(rawA.RecordLookups)+len(rawA.ActionCalls)+len(rawA.CustomErrors)+len(rawA.Subflows)+len(rawA.Loops)+len(rawA.Screens) != 0 {
		return false
	}
	sourceRef := func(name string) string {
		if strings.HasPrefix(name, "$Record__Prior.") {
			return name
		}
		return "$Record." + name
	}
	deleteTarget := rawA.RecordDeletes[0].Connector.TargetReference
	if len(rawA.RecordDeletes[0].Filters) != len(del.Criteria) || rawA.RecordDeletes[0].InputReference != "" || (!replacement && deleteTarget != "") || (replacement && !strings.EqualFold(deleteTarget, rawA.RecordCreates[0].Name)) || rawA.RecordDeletes[0].FaultConnector.TargetReference != "" || (rawA.RecordDeletes[0].FilterLogic != "" && !strings.EqualFold(rawA.RecordDeletes[0].FilterLogic, "and")) {
		return false
	}
	for _, f := range rawA.RecordDeletes[0].Filters {
		found := false
		for _, c := range del.Criteria {
			if !strings.EqualFold(f.Field, c.Field) {
				continue
			}
			found = true
			if !strings.EqualFold(f.Operator, "EqualTo") {
				return false
			}
			if c.SourceField != "" {
				ref := sourceRef(c.SourceField)
				if f.Value != (flowValueXML{ElementReference: ref}) {
					return false
				}
			} else if f.Value != (flowValueXML{StringValue: c.Value}) {
				return false
			}
		}
		if !found {
			return false
		}
	}
	create := a.Steps[0].Branches[1].Steps[0].RecordCreate
	rc := rawA.RecordCreates[0]
	if !strings.EqualFold(rc.Name, create.Name) || !strings.EqualFold(rc.Object, create.ObjectName) || rc.InputReference != "" || rc.AssignRecordIDToReference != "" || rc.StoreOutputAutomatically != replacement || rc.Connector.TargetReference != "" || rc.FaultConnector.TargetReference != "" || len(rc.Fields) != len(create.InputAssignments) {
		return false
	}
	for _, f := range rc.Fields {
		found := false
		for _, a := range create.InputAssignments {
			if !strings.EqualFold(f.Field, a.Field) {
				continue
			}
			found = true
			if a.SourceField != "" {
				ref := sourceRef(a.SourceField)
				if f.Value != (flowValueXML{ElementReference: ref}) {
					return false
				}
			} else if f.Value != (flowValueXML{StringValue: a.LiteralValue}) {
				return false
			}
		}
		if !found {
			return false
		}
	}
	if len(b.Steps) != 1 || len(b.Steps[0].Branches) != 2 || len(b.Steps[0].Branches[1].Steps) != 1 {
		return false
	}
	d := &b.Steps[0].Branches[1].Steps[0]
	if len(d.Branches) != 1 || len(d.Branches[0].Steps) != 1 {
		return false
	}
	action := &d.Branches[0].Steps[0].Action
	prefix, alertName, ok := strings.Cut(action.ActionName, ".")
	if !ok || !strings.EqualFold(prefix, object) || alertName == "" {
		return false
	}
	var recipient workflowRecipientXML
	template := ""
	found := 0
	for _, workflow := range idx.Workflows {
		if !strings.EqualFold(workflow.ObjectName, object) {
			continue
		}
		source, err := os.ReadFile(workflow.File)
		if err != nil {
			return false
		}
		var w workflowXML
		if xml.Unmarshal(source, &w) != nil {
			return false
		}
		for _, alert := range w.Alerts {
			if !strings.EqualFold(alert.FullName, alertName) {
				continue
			}
			if len(alert.Recipients) != 1 || len(alert.CCEmails) != 0 || alert.Template == "" {
				return false
			}
			recipient, template = alert.Recipients[0], alert.Template
			found++
		}
	}
	if found != 1 || recipient.Recipient != "" || recipient.Field == "" || !strings.EqualFold(recipient.Type, "userLookup") {
		return false
	}
	name, ok := storage.ResolveFieldName(def, "", recipient.Field)
	if !ok {
		return false
	}
	f := def.Fields[name]
	if f.Type != storage.FieldReference || len(f.ReferenceTo) != 1 || !strings.EqualFold(f.ReferenceTo[0], "User") {
		return false
	}
	if len(action.Inputs) != 1 {
		return false
	}
	action.Inputs = append(action.Inputs, storage.WorkflowFieldUpdate{Name: storage.CapturedMixedRecipientInput, Field: recipient.Type, SourceField: name}, storage.WorkflowFieldUpdate{Name: storage.CapturedMixedTemplateInput, LiteralValue: template})
	validatedDefinitions := make(map[string]storage.ObjectDefinition, len(definitions)+1)
	for name, value := range definitions {
		validatedDefinitions[name] = value
	}
	personAccounts := false
	for _, set := range features {
		for _, feature := range set {
			if strings.EqualFold(feature, "PersonAccounts") {
				personAccounts = true
			}
		}
	}
	if len(candidate[3].Branches) == 5 && !personAccounts {
		return false
	}
	if personAccounts {
		var account storage.ObjectDefinition
		for name, value := range definitions {
			if strings.EqualFold(name, "Account") {
				account = value.Clone()
				break
			}
		}
		if account.APIName == "" {
			account.APIName = "Account"
		}
		storage.EnsureStandardObjectFieldsForFeatures(&account, []string{"PersonAccounts"})
		validatedDefinitions["Account"] = account
	}

	if _, ok := storage.CapturedBranchedAfterSavePlan(candidate, def, validatedDefinitions); !ok {
		return false
	}
	for i := range idx.Flows {
		if !strings.EqualFold(idx.Flows[i].ObjectName, object) {
			continue
		}
		for j := range idx.Flows[i].Rules {
			for _, rule := range candidate {
				if strings.EqualFold(idx.Flows[i].Rules[j].Name, rule.Name) {
					idx.Flows[i].Rules[j] = rule
				}
			}
		}
	}
	filtered := idx.Diagnostics[:0]
	for _, d := range idx.Diagnostics {
		if d.File == dates.File && d.Code == "GLADEAUTO002" && strings.Contains(d.Message, "start filter logic with a filtered triggering-record update is not supported") {
			continue
		}
		filtered = append(filtered, d)
	}
	idx.Diagnostics = filtered
	return true
}
