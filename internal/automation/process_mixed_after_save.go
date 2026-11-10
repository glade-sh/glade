package automation

import (
	"encoding/xml"
	"os"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// The captured graph has exactly one workflow-alert recipient, either an Email
// field or a User lookup. Preserve its source binding on the compiled action so
// the runtime can distinguish the captured success and zero-recipient rows.
func processBuilderCapturedMixedAfterSave(idx *Index, object string, rules []storage.FlowRule, definition storage.ObjectDefinition, definitions map[string]storage.ObjectDefinition) bool {
	plan, ok := storage.CapturedMixedAfterSavePlan(rules, definition, definitions)
	if !ok {
		return false
	}
	action := plan[3].Steps[0].Branches[0].Steps[0].Action
	prefix, alertName, hasPrefix := strings.Cut(action.ActionName, ".")
	if !hasPrefix || !strings.EqualFold(prefix, object) || alertName == "" {
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
		var raw workflowXML
		if xml.Unmarshal(source, &raw) != nil {
			return false
		}
		for _, alert := range raw.Alerts {
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
	if found != 1 || recipient.Recipient != "" || recipient.Field == "" {
		return false
	}
	field, ok := storage.ResolveFieldName(definition, "", recipient.Field)
	if !ok {
		return false
	}
	fd := definition.Fields[field]
	switch {
	case strings.EqualFold(recipient.Type, "email"):
		if fd.Type != storage.FieldString || !strings.EqualFold(fd.DisplayType, "EMAIL") {
			return false
		}
	case strings.EqualFold(recipient.Type, "userLookup"):
		if fd.Type != storage.FieldReference || len(fd.ReferenceTo) != 1 || !strings.EqualFold(fd.ReferenceTo[0], "User") {
			return false
		}
	default:
		return false
	}
	for i := range idx.Flows {
		flow := &idx.Flows[i]
		if !strings.EqualFold(flow.ObjectName, object) {
			continue
		}
		for j := range flow.Rules {
			rule := &flow.Rules[j]
			if !strings.EqualFold(rule.Name, plan[3].Name) {
				continue
			}
			action := &rule.Steps[0].Branches[0].Steps[0].Action
			if len(action.Inputs) == 3 {
				return action.Inputs[1].Name == storage.CapturedMixedRecipientInput && strings.EqualFold(action.Inputs[1].Field, recipient.Type) && strings.EqualFold(action.Inputs[1].SourceField, field) && action.Inputs[2].Name == storage.CapturedMixedTemplateInput && action.Inputs[2].LiteralValue == template
			}
			if len(action.Inputs) != 1 {
				return false
			}
			action.Inputs = append(action.Inputs,
				storage.WorkflowFieldUpdate{Name: storage.CapturedMixedRecipientInput, Field: recipient.Type, SourceField: field},
				storage.WorkflowFieldUpdate{Name: storage.CapturedMixedTemplateInput, LiteralValue: template})
			return true
		}
	}
	return false
}
