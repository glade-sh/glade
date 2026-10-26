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
