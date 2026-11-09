package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// flowRecordWithCriteriaFormulas evaluates each stored formula field a flow
// Decision condition names on the triggering record and is not yet on it.
// Native rows (flow_assign_routes, API 62.0 and 67.0): an after-save Decision
// on $Record.<cross-object formula> matches the formula's value. Formula
// fields read finished, as other flow readers do (FlowContext).
func (e *Engine) flowRecordWithCriteriaFormulas(record storage.Record, definition storage.ObjectDefinition, criteria []storage.WorkflowCriteriaItem) storage.Record {
	if e == nil || e.Org == nil || len(definition.Fields) == 0 {
		return record
	}
	cloned := false
	for _, item := range criteria {
		name := strings.TrimSpace(item.Field)
		if name == "" || strings.HasPrefix(name, "$") || strings.Contains(name, ".") {
			continue
		}
		fieldName, ok := storage.ResolveFieldName(definition, e.Org.Namespace, name)
		if !ok {
			continue
		}
		field, ok := definition.Fields[fieldName]
		if !ok || field.Type != storage.FieldCalculated || strings.TrimSpace(field.Formula) == "" {
			continue
		}
		if _, present := record.Fields[fieldName]; present {
			continue
		}
		value, _, ok := EvaluateRecordFormulaValueInOrgWithOptions(field.Formula, field, e.Org, definition, record, e.flowFormulaEvaluationOptions())
		if !ok {
			continue
		}
		if !cloned {
			record = record.Clone()
			if record.Fields == nil {
				record.Fields = make(map[string]storage.Value)
			}
			cloned = true
		}
		record.Fields[fieldName] = value
	}
	return record
}
