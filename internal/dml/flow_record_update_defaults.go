package dml

import (
	"errors"
	"strings"
)

// Owned Flow record-binding observations distinguish an untouched record
// variable from a partial record without Id. Update Records ignores the former.
func (f *flowFrame) flowRecordUpdateInputIsNull(reference string) bool {
	key := flowFrameKey(reference)
	if _, scalar := f.scalars[key]; scalar || f.assigned[key] {
		return false
	}
	if record, present := f.records[key]; present {
		return f.nullRecordDefaults[key] && record.ID == "" && len(record.Fields) == 0 && len(record.ExplicitNulls) == 0
	}
	variable, declared := f.variables[key]
	return declared && strings.EqualFold(variable.DataType, "SObject") && strings.TrimSpace(variable.ObjectType) != ""
}

type flowRecordUpdateMissingID struct{ cause error }

func (e *flowRecordUpdateMissingID) Error() string { return e.cause.Error() }
func (e *flowRecordUpdateMissingID) Unwrap() error { return e.cause }

// IsFlowRecordUpdateMissingID identifies the scalar Update Records fault that
// Flow.Interview.start exposes to Apex as a FlowException.
func IsFlowRecordUpdateMissingID(err error) bool {
	var failure *flowRecordUpdateMissingID
	return errors.As(err, &failure)
}
