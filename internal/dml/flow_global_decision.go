package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// flowGlobalDecisionCriterion handles captured direct global equality operands.
// Both flat and ordered Flow graphs must use the same running-user context.
func (e *Engine) flowGlobalDecisionCriterion(item storage.WorkflowCriteriaItem) (bool, bool, bool) {
	return e.flowGlobalDecisionCriterionMode(item, false)
}

// flowGlobalDecisionCriterionMode adds the after-save Update route: a
// record-triggered after-save Flow on Update evaluates $Profile.Name with
// both EqualTo and NotEqualTo against the running user's profile
// (flow_profile_after_save_update, API 62.0 and 67.0). Other global operands
// and operators keep the before-save equality route only.
func (e *Engine) flowGlobalDecisionCriterionMode(item storage.WorkflowCriteriaItem, afterSaveUpdate bool) (bool, bool, bool) {
	reference := strings.TrimSpace(item.Field)
	operation := strings.ToLower(strings.TrimSpace(item.Operation))
	if afterSaveUpdate && strings.EqualFold(reference, "$Profile.Name") && flowProfileNameOperation(operation) {
		return e.flowProfileNameCriterion(item)
	}
	if !strings.EqualFold(strings.TrimSpace(item.Operation), "equals") {
		return false, false, false
	}
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
	if strings.EqualFold(reference, "$Profile.Name") {
		return e.flowProfileNameCriterion(item)
	}
	return false, false, false
}

func flowProfileNameOperation(operation string) bool {
	switch operation {
	case "equals", "equal", "eq", "notequal", "notequalto", "not equal", "notequals", "not equals", "ne":
		return true
	}
	return false
}

func (e *Engine) flowProfileNameCriterion(item storage.WorkflowCriteriaItem) (bool, bool, bool) {
	if flowCriteriaResourceReference(item) != "" || e.Org == nil {
		return false, false, false
	}
	parser := formulaParser{org: e.Org, userID: e.systemUserID()}
	profile := parser.profileNameValue()
	if profile.kind == formulaNull {
		return false, false, true
	}
	matched, supported := evaluateFlowValueCriteria(storage.StringValue(profile.asString()), item)
	return matched, supported, true
}

func flowGlobalDecisionReference(reference string) bool {
	reference = strings.TrimSpace(reference)
	return strings.EqualFold(reference, "$Profile.Name") || strings.EqualFold(reference, "$User.Id") || len(reference) > len("$Permission.") && strings.EqualFold(reference[:len("$Permission.")], "$Permission.")
}
