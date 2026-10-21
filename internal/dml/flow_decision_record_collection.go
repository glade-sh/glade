package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// Assignment-built record collections live separately from Get Records output.
// An uninitialized declared record collection is empty as well.
func flowRecordCollectionIsEmptyCriterion(item storage.WorkflowCriteriaItem, frame *flowFrame) (matches, supported, handled bool) {
	if frame == nil || !strings.EqualFold(strings.TrimSpace(item.Operation), "isEmpty") {
		return false, false, false
	}
	key := flowFrameKey(item.Field)
	records, known := frame.collections[key]
	if !known {
		variable, declared := frame.variables[key]
		if !declared || !variable.IsCollection || !strings.EqualFold(variable.DataType, "SObject") {
			return false, false, false
		}
	}
	if item.SourceField != "" {
		return false, false, true
	}
	switch strings.ToLower(trimFormulaLiteral(item.Value)) {
	case "true":
		return len(records) == 0, true, true
	case "false":
		return len(records) != 0, true, true
	default:
		return false, false, true
	}
}
