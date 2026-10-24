package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

func flowFirstRecordNullDecisionValue(item storage.WorkflowCriteriaItem, frame *flowFrame) (storage.Value, bool) {
	if frame == nil || !strings.EqualFold(strings.TrimSpace(item.Operation), "isnull") {
		return storage.Value{}, false
	}
	want := trimFormulaLiteral(item.Value)
	if !strings.EqualFold(want, "true") && !strings.EqualFold(want, "false") {
		return storage.Value{}, false
	}
	reference := strings.TrimSpace(item.Field)
	if lookup, field, hasField := strings.Cut(reference, "."); hasField {
		if !strings.EqualFold(strings.TrimSpace(field), "Id") {
			return storage.Value{}, false
		}
		reference = strings.TrimSpace(lookup)
	}
	output, ok := frame.lookupOutputs[flowFrameKey(reference)]
	if !ok {
		return storage.Value{}, false
	}
	if output.record.ID == "" {
		return storage.NullValue(), true
	}
	return storage.IDValue(output.record.ID), true
}
