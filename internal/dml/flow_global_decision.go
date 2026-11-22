package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// flowGlobalDecisionCriterion handles captured direct global equality operands.
// Both flat and ordered Flow graphs must use the same running-user context.
func (e *Engine) flowGlobalDecisionCriterion(item storage.WorkflowCriteriaItem) (bool, bool, bool) {
	if !strings.EqualFold(strings.TrimSpace(item.Operation), "equals") {
		return false, false, false
	}
	reference := strings.TrimSpace(item.Field)
	if strings.EqualFold(reference, "$User.Id") {
		value, ok := e.flowCurrentUserID()
		if !ok {
			return false, false, true
		}
		if right := flowCriteriaResourceReference(item); right != "" {
			if !strings.EqualFold(right, "$User.Id") {
				return false, false, true
			}
			item.Value = value.String
		} else if n := len(strings.TrimSpace(item.Value)); n == 15 || n == 18 {
			// Literal ID normalization and case behavior have not been captured.
			return false, false, true
		}
		matched, supported := evaluateFlowValueCriteria(value, item)
		return matched, supported, true
	}
	if len(reference) > len("$Permission.") && strings.EqualFold(reference[:len("$Permission.")], "$Permission.") && flowCriteriaResourceReference(item) == "" {
		value, known := e.flowCustomPermission(reference[len("$Permission."):])
		if !known {
			return false, false, true
		}
		matched, supported := evaluateFlowValueCriteria(storage.BooleanValue(value), item)
		return matched, supported, true
	}
	if strings.EqualFold(reference, "$Profile.Name") && flowCriteriaResourceReference(item) == "" && e.Org != nil {
		parser := formulaParser{org: e.Org, userID: e.systemUserID()}
		profile := parser.profileNameValue()
		if profile.kind == formulaNull {
			return false, false, true
		}
		matched, supported := evaluateFlowValueCriteria(storage.StringValue(profile.asString()), item)
		return matched, supported, true
	}
	return false, false, false
}

// flowAfterCreateProfileRecordCriterion covers direct profile equality against
// a triggering-record text field in an after-save create Flow. The two API
// versions and both routes are captured in flow_direct_profile_decision/org.tsv.
func (e *Engine) flowAfterCreateProfileRecordCriterion(item storage.WorkflowCriteriaItem, record storage.Record, definition storage.ObjectDefinition) (bool, bool, bool) {
	if e.Org == nil || !strings.EqualFold(strings.TrimSpace(item.Field), "$Profile.Name") || !strings.EqualFold(strings.TrimSpace(item.Operation), "equals") {
		return false, false, false
	}
	reference := flowCriteriaResourceReference(item)
	const prefix = "$Record."
	if len(reference) <= len(prefix) || !strings.EqualFold(reference[:len(prefix)], prefix) {
		return false, false, false
	}
	field := reference[len(prefix):]
	if strings.Contains(field, ".") {
		return false, false, false
	}
	name, ok := storage.ResolveFieldName(definition, e.Org.Namespace, field)
	if !ok || definition.Fields[name].Type != storage.FieldString || !strings.EqualFold(definition.Fields[name].DisplayType, "STRING") {
		return false, false, false
	}
	parser := formulaParser{org: e.Org, userID: e.systemUserID()}
	profile := parser.profileNameValue()
	if profile.kind == formulaNull {
		return false, false, true
	}
	profileName := profile.asString()
	if strings.TrimSpace(profileName) != profileName {
		return false, false, false
	}
	for _, char := range profileName {
		if char >= 128 {
			return false, false, false
		}
	}
	value, present := sourceRecordFieldValue(record, name)
	if !present || value.Kind == storage.ValueNull {
		return false, true, true
	}
	if value.Kind != storage.ValueString {
		return false, false, true
	}
	for _, text := range []string{value.String} {
		if strings.TrimSpace(text) != text {
			return false, false, false
		}
		for _, char := range text {
			if char >= 128 {
				return false, false, false
			}
		}
	}
	return strings.EqualFold(profileName, value.String), true, true
}

func flowGlobalDecisionReference(reference string) bool {
	reference = strings.TrimSpace(reference)
	return strings.EqualFold(reference, "$Profile.Name") || strings.EqualFold(reference, "$User.Id") || len(reference) > len("$Permission.") && strings.EqualFold(reference[:len("$Permission.")], "$Permission.")
}
