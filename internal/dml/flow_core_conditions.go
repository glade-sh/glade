package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

func evaluateFlowCoreStringCriteria(item storage.WorkflowCriteriaItem, record storage.Record, definition storage.ObjectDefinition, namespace string) (bool, bool, bool) {
	operation := strings.ToLower(strings.TrimSpace(item.Operation))
	if (operation != "startswith" && operation != "endswith") || strings.TrimSpace(item.SourceField) != "" {
		return false, false, false
	}
	fieldName, ok := storage.ResolveFieldName(definition, namespace, strings.TrimSpace(item.Field))
	if !ok || definition.Fields[fieldName].Type != storage.FieldString {
		return false, false, false
	}
	value, present := record.Fields[fieldName]
	if !present || value.Kind == storage.ValueNull {
		return false, false, false
	}
	if value.Kind != storage.ValueString {
		return false, false, false
	}
	want := trimFormulaLiteral(item.Value)
	if operation == "startswith" {
		return strings.HasPrefix(value.String, want), true, true
	}
	return strings.HasSuffix(value.String, want), true, true
}

func evaluateFlowCoreEntryCriteria(item storage.WorkflowCriteriaItem, record storage.Record, definition storage.ObjectDefinition, namespace string, prior *storage.Record, rule storage.FlowRule, created bool) (bool, bool) {
	trigger := strings.TrimSpace(rule.TriggerType)
	recordTrigger := strings.TrimSpace(rule.RecordTriggerType)
	if (strings.EqualFold(trigger, "RecordBeforeSave") || strings.EqualFold(trigger, "RecordAfterSave")) &&
		(strings.EqualFold(recordTrigger, "Create") || strings.EqualFold(recordTrigger, "Update") || strings.EqualFold(recordTrigger, "CreateAndUpdate")) {
		if created && prior == nil && strings.EqualFold(strings.TrimSpace(item.Operation), "isChanged") {
			if _, ok := storage.ResolveFieldName(definition, namespace, strings.TrimSpace(item.Field)); !ok {
				return false, false
			}
			switch strings.ToLower(trimFormulaLiteral(item.Value)) {
			case "false":
				return true, true
			case "true":
				return false, true
			}
		}
		// A record-triggered Flow compares checkbox literals as booleans. In
		// particular, the text "false" must not be treated as truthy text.
		if created && strings.EqualFold(trigger, "RecordBeforeSave") && strings.EqualFold(strings.TrimSpace(item.Operation), "equals") && strings.TrimSpace(item.SourceField) == "" {
			fieldName, ok := storage.ResolveFieldName(definition, namespace, strings.TrimSpace(item.Field))
			want := strings.ToLower(trimFormulaLiteral(item.Value))
			if ok && definition.Fields[fieldName].Type == storage.FieldBoolean && (want == "true" || want == "false") {
				value, present := record.Fields[fieldName]
				return present && value.Kind == storage.ValueBoolean && value.Boolean == (want == "true"), true
			}
		}
		if matches, supported, handled := evaluateFlowCoreStringCriteria(item, record, definition, namespace); handled {
			return matches, supported
		}
	}
	return evaluateWorkflowCriteriaWithPrior(item, record, definition, namespace, prior)
}

func evaluateFlowCoreLookupCriteria(item storage.WorkflowCriteriaItem, record storage.Record, definition storage.ObjectDefinition, namespace string) (bool, bool) {
	if matches, supported, handled := evaluateFlowCoreStringCriteria(item, record, definition, namespace); handled {
		return matches, supported
	}
	return evaluateWorkflowCriteria(item, record, definition, namespace)
}

// evaluateCapturedAfterSaveChangedCriterion is used only for the separately
// gated heterogeneous after-save plan in the native afteraccept rows.
func evaluateCapturedAfterSaveChangedCriterion(item storage.WorkflowCriteriaItem, record storage.Record, definition storage.ObjectDefinition, namespace string, prior *storage.Record, rule storage.FlowRule, created bool) (bool, bool) {
	if !created && strings.EqualFold(strings.TrimSpace(item.Operation), "isChanged") {
		changed, ok := evaluateWorkflowCriteriaWithPrior(item, record, definition, namespace, prior)
		if !ok {
			return false, false
		}
		switch strings.ToLower(trimFormulaLiteral(item.Value)) {
		case "true":
			return changed, true
		case "false":
			return !changed, true
		default:
			return false, false
		}
	}
	return evaluateFlowCoreEntryCriteria(item, record, definition, namespace, prior, rule, created)
}

// evaluateCapturedCaseAfterSaveCriterion retains checkbox literal behavior for
// the captured Case after-save group before applying the shared change checks.
func evaluateCapturedCaseAfterSaveCriterion(item storage.WorkflowCriteriaItem, record storage.Record, definition storage.ObjectDefinition, namespace string, prior *storage.Record, rule storage.FlowRule, created bool) (bool, bool) {
	if strings.EqualFold(strings.TrimSpace(item.Operation), "equals") && item.SourceField == "" {
		name, ok := storage.ResolveFieldName(definition, namespace, strings.TrimSpace(item.Field))
		want := strings.ToLower(trimFormulaLiteral(item.Value))
		if ok && definition.Fields[name].Type == storage.FieldBoolean && (want == "true" || want == "false") {
			value, present := record.Fields[name]
			return present && value.Kind == storage.ValueBoolean && value.Boolean == (want == "true"), true
		}
		if ok && definition.Fields[name].Type == storage.FieldCalculated && strings.EqualFold(definition.Fields[name].DisplayType, string(storage.FieldString)) && strings.EqualFold(strings.TrimSpace(definition.Fields[name].Formula), "RecordType.DeveloperName") {
			value := formulaRecordTypeValue(definition, record, "$RecordType.DeveloperName")
			return compareFormulaValues(formulaValue{kind: formulaString, text: value}, formulaValue{kind: formulaString, text: trimFormulaLiteral(item.Value)}, "="), true
		}
	}
	return evaluateCapturedAfterSaveChangedCriterion(item, record, definition, namespace, prior, rule, created)
}
