package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// Fault-connected after-save updates must reject malformed parent text copied
// into a self lookup before any assignment is persisted. Other source kinds
// and update contexts retain the existing reference validation.
func (e *Engine) validateFlowRelationshipUpdate(flowName string, step storage.FlowStep, source storage.Record, definition storage.ObjectDefinition) error {
	if e == nil || e.Org == nil || step.FaultTarget == "" || source.ID == "" {
		return nil
	}
	admitted := false
	for _, rule := range definition.FlowRules {
		if rule.Name == flowName && strings.EqualFold(strings.TrimSpace(rule.TriggerType), "RecordAfterSave") && strings.EqualFold(strings.TrimSpace(rule.RecordTriggerType), "CreateAndUpdate") {
			admitted = true
			break
		}
	}
	update := step.RecordUpdate
	if !admitted || update.InputReference != "" || len(update.Criteria) != 1 {
		return nil
	}
	criterion := update.Criteria[0]
	if !strings.EqualFold(criterion.Field, "Id") || !strings.EqualFold(criterion.Operation, "equals") || !strings.EqualFold(criterion.SourceField, "Id") {
		return nil
	}
	targetName, ok := storage.ResolveObjectName(*e.Org, update.ObjectName)
	if !ok || !strings.EqualFold(targetName, definition.APIName) {
		return nil
	}
	for _, assignment := range update.InputAssignments {
		if strings.Count(strings.TrimSpace(assignment.SourceField), ".") != 1 {
			continue
		}
		fieldName, found := storage.ResolveFieldName(definition, e.Org.Namespace, assignment.Field)
		if !found {
			continue
		}
		field := definition.Fields[fieldName]
		if field.Type != storage.FieldReference || field.MasterDetail || len(field.ReferenceTo) != 1 {
			continue
		}
		parent, modeled := relationshipLookupField(definition, e.Org.Namespace, strings.Split(strings.TrimSpace(assignment.SourceField), ".")[0])
		if !modeled || !strings.EqualFold(parent.APIName, fieldName) {
			continue
		}
		referenceName, found := storage.ResolveObjectName(*e.Org, field.ReferenceTo[0])
		if !found || !strings.EqualFold(referenceName, targetName) {
			continue
		}
		parentValue, present := sourceRecordFieldValue(source, fieldName)
		if !present {
			continue
		}
		_, parentRecord, stored := storage.LookupRecordByID(e.Org.Objects[targetName].Records, idFromStorageValue(parentValue))
		if !stored || parentRecord.System.IsDeleted {
			continue
		}
		value, resolved := e.flowRefsRelationshipValue(assignment.SourceField, source, definition)
		if resolved && value.Kind == storage.ValueString && value.String != "" && storage.ValidateID(storage.ID(value.String)) != nil {
			return dmlErrorf("FIELD_INTEGRITY_EXCEPTION", []string{fieldName}, "dml: invalid reference %s.%s", source.Object, fieldName)
		}
	}
	return nil
}
