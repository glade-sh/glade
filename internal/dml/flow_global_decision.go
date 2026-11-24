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

// flowAfterCreateProfileRecordCriterion covers $Profile.Name equality and
// inequality in an after-save record-triggered Flow, against a triggering-record
// text field or a string literal. Both compare case-insensitively; a null record
// field never equals and always differs. Captured at 62.0 and 67.0 on both
// routes in flow_direct_profile_decision, flow_profile_comparison_boundaries,
// flow_profile_record_normalization and flow_criteria_shapes (org.tsv). Other
// operators, non-ASCII or padded text and other operands are not handled.
// After-create equality also handles a one-parent text reference, captured in
// flow_profile_parent_rhs at APIs 62.0 through 67.0 on both routes.
func (e *Engine) flowAfterCreateProfileRecordCriterion(item storage.WorkflowCriteriaItem, record storage.Record, definition storage.ObjectDefinition, allowParent bool) (bool, bool, bool) {
	if e.Org == nil || !strings.EqualFold(strings.TrimSpace(item.Field), "$Profile.Name") {
		return false, false, false
	}
	operation := strings.TrimSpace(item.Operation)
	equals := strings.EqualFold(operation, "equals")
	if !equals && !strings.EqualFold(operation, "notEqual") {
		return false, false, false
	}
	reference := flowCriteriaResourceReference(item)
	name := ""
	var related storage.Value
	parent := false
	if reference != "" {
		const prefix = "$Record."
		if len(reference) <= len(prefix) || !strings.EqualFold(reference[:len(prefix)], prefix) {
			return false, false, false
		}
		field := reference[len(prefix):]
		if strings.Contains(field, ".") {
			parts := strings.Split(field, ".")
			if !allowParent || !equals || len(parts) != 2 {
				return false, false, false
			}
			lookup, ok := relationshipLookupField(definition, e.Org.Namespace, parts[0])
			// Setup-user relationships have no native rows for this comparison.
			if !ok || len(lookup.ReferenceTo) != 1 || strings.EqualFold(lookup.ReferenceTo[0], "User") {
				return false, false, false
			}
			parentField, _, ok := formulaRelationshipFieldDefinition(*e.Org, lookup, parts[1:], e.Org.Namespace)
			if !ok || parentField.Type != storage.FieldString || !strings.EqualFold(parentField.DisplayType, "STRING") || parentField.Formula != "" {
				return false, false, false
			}
			related, ok = e.flowRefsRelationshipValue(field, record, definition)
			if !ok {
				return false, false, false
			}
			parent = true
		} else {
			resolved, ok := storage.ResolveFieldName(definition, e.Org.Namespace, field)
			if !ok || definition.Fields[resolved].Type != storage.FieldString || !strings.EqualFold(definition.Fields[resolved].DisplayType, "STRING") {
				return false, false, false
			}
			name = resolved
		}
	} else if item.Value == "" || !flowProfileComparableText(item.Value) {
		return false, false, false
	}
	parser := formulaParser{org: e.Org, userID: e.systemUserID()}
	profile := parser.profileNameValue()
	if profile.kind == formulaNull {
		return false, false, true
	}
	profileName := profile.asString()
	if !flowProfileComparableText(profileName) {
		return false, false, false
	}
	text := item.Value
	if name != "" || parent {
		value, present := related, parent
		if !parent {
			value, present = sourceRecordFieldValue(record, name)
		}
		if !present || value.Kind == storage.ValueNull {
			return !equals, true, true
		}
		if value.Kind != storage.ValueString {
			return false, false, true
		}
		if !flowProfileComparableText(value.String) {
			return false, false, false
		}
		text = value.String
	}
	if parent && strings.EqualFold(profileName, text) && profileName != text {
		return false, false, false
	}
	return strings.EqualFold(profileName, text) == equals, true, true
}

// flowProfileComparableText limits the captured comparison to trimmed ASCII.
func flowProfileComparableText(text string) bool {
	if strings.TrimSpace(text) != text {
		return false
	}
	for _, char := range text {
		if char >= 128 {
			return false
		}
	}
	return true
}

func flowGlobalDecisionReference(reference string) bool {
	reference = strings.TrimSpace(reference)
	return strings.EqualFold(reference, "$Profile.Name") || strings.EqualFold(reference, "$User.Id") || len(reference) > len("$Permission.") && strings.EqualFold(reference[:len("$Permission.")], "$Permission.")
}
