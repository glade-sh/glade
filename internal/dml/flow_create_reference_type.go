package dml

import (
	"errors"

	"github.com/glade-sh/glade/internal/storage"
)

// flowCreateReferenceTypeError marks a Create Records assignment whose Id value
// does not belong to the lookup's target object (FIELD_INTEGRITY_EXCEPTION) or
// is not an Id at all (MALFORMED_ID). Native rows: flow_relpath_decisions CR1,
// CR2, CR3 at API 62.0 and 67.0.
type flowCreateReferenceTypeError struct {
	code  string
	field string
	label string
	value string
}

func (e *flowCreateReferenceTypeError) Error() string {
	return "dml: flow record create: id value of incorrect type: " + e.value
}

// flowCreateReferenceTypeCheck validates the Id-shaped lookup values a Create
// Records element assigns. It returns nil for values it cannot classify.
func (e *Engine) flowCreateReferenceTypeCheck(target storage.ObjectDefinition, record storage.Record) error {
	for name, field := range target.Fields {
		if field.Type != storage.FieldReference || len(field.ReferenceTo) == 0 {
			continue
		}
		value, ok := record.Fields[name]
		if !ok || value.Kind == storage.ValueNull {
			continue
		}
		text := workflowValueString(value)
		if value.Kind == storage.ValueID {
			text = string(value.ID)
		}
		if text == "" {
			continue
		}
		label := field.Label
		if label == "" {
			label = name
		}
		if storage.ValidateID(storage.ID(text)) != nil {
			if value.Kind != storage.ValueString && value.Kind != storage.ValueID {
				continue
			}
			return &flowCreateReferenceTypeError{code: "MALFORMED_ID", field: name, label: label, value: text}
		}
		if !e.flowCreateReferencePrefixKnown(field) || e.validTaskWhatReferenceType(field, storage.ID(text)) {
			continue
		}
		return &flowCreateReferenceTypeError{code: "FIELD_INTEGRITY_EXCEPTION", field: name, label: label, value: text}
	}
	return nil
}

// flowCreateReferencePrefixKnown is true when every target of the lookup has a
// key prefix Glade can compare, so an unmatched prefix is a real type mismatch.
func (e *Engine) flowCreateReferencePrefixKnown(field storage.Field) bool {
	for _, target := range field.ReferenceTo {
		prefix := storage.StandardKeyPrefix(target)
		if canonical, ok := storage.ResolveObjectName(*e.Org, target); ok {
			prefix = e.Org.Objects[canonical].Definition.KeyPrefix
		}
		if prefix == "" {
			return false
		}
	}
	return true
}

// flowCoreRecordCreateFailure is an unhandled element failure of a
// record-triggered flow. saved marks the failures whose singleton save has a
// native row error (duplicate-rule rejections and Apex action failures); the
// others keep the error path they had.
type flowCoreRecordCreateFailure struct {
	cause error
	saved bool
}

func (e *flowCoreRecordCreateFailure) Error() string { return e.cause.Error() }
func (e *flowCoreRecordCreateFailure) Unwrap() error { return e.cause }

// flowCoreRecordCreateTriggerError wraps an after-save Create Records type
// error in the process-failure message observed natively. The Error ID is not
// reproducible, so Glade writes a fixed placeholder.
func flowCoreRecordCreateTriggerError(rule storage.FlowRule, typed *flowCreateReferenceTypeError) error {
	message := "We can't save this record because the “" + rule.Name + "” process failed. Give your Salesforce admin these details. CANNOT_EXECUTE_FLOW_TRIGGER: This error occurred when the flow tried to create records: " + typed.code + ": " + typed.label + ": id value of incorrect type: " + typed.value + ". You can look up ExceptionCode values in the SOAP API Developer Guide. Error ID: 0-0 (0)"
	return &flowCoreRecordCreateFailure{cause: dmlErrorf(typed.code, nil, "%s", message)}
}

// FlowRecordCreateFailureResult exposes the captured Create Records type
// failure as the row error of a statement insert.
func FlowRecordCreateFailureResult(err error) (Result, bool) {
	var failure *flowCoreRecordCreateFailure
	if !errors.As(err, &failure) {
		return Result{}, false
	}
	return resultFromError("", failure.cause), true
}

// FlowSavedRecordFailureResult is FlowRecordCreateFailureResult limited to the
// failures that native rows show as the row error of a singleton save.
func FlowSavedRecordFailureResult(err error) (Result, bool) {
	var failure *flowCoreRecordCreateFailure
	if !errors.As(err, &failure) || !failure.saved {
		return Result{}, false
	}
	return resultFromError("", failure.cause), true
}
