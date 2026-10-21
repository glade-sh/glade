package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// Captured before-save CreateAndUpdate Picklist decisions compare the immediate
// prior record, including an update after an insert in the same transaction.
// Other decision shapes retain their existing transaction-provenance boundary.
func evaluateFlowDecisionChange(item storage.WorkflowCriteriaItem, source storage.Record, definition storage.ObjectDefinition, namespace string, frame *flowFrame) (bool, bool) {
	field := strings.TrimSpace(item.Field)
	if strings.Contains(field, ".") || item.SourceField != "" {
		return false, false
	}
	name, ok := storage.ResolveFieldName(definition, namespace, field)
	if !ok {
		return false, false
	}
	want := trimFormulaLiteral(item.Value)
	if !strings.EqualFold(want, "true") && !strings.EqualFold(want, "false") {
		return false, false
	}
	changed := false
	prior := frame.records[flowFrameKey("$Record__Prior")]
	if prior.ID != "" {
		if frame.recordCreations == nil {
			return false, false
		}
		compareImmediatePrior := frame.beforeSaveDecisions && definition.Fields[name].Type == storage.FieldPicklist
		if compareImmediatePrior || !frame.recordCreations.Contains(source.ID) {
			currentValue, found := sourceRecordFieldValue(source, name)
			if !found {
				currentValue = storage.NullValue()
			}
			priorValue, found := sourceRecordFieldValue(prior, name)
			if !found {
				priorValue = storage.NullValue()
			}
			changed = !storage.ValuesEqual(currentValue, priorValue)
		}
	}
	return changed == strings.EqualFold(want, "true"), true
}
