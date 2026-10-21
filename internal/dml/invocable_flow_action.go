package dml

import (
	"errors"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// Retain the ordinary Flow DML error and its display/As behavior. Only the
// Invocable.Action adapter reads the captured underlying Create Records fault.
type invocableFlowActionCreateError struct {
	legacy error
	cause  dmlError
}

type invocableFlowActionInsertError struct{ cause dmlError }

func (e *invocableFlowActionInsertError) Error() string { return e.cause.Error() }
func (e *invocableFlowActionInsertError) Unwrap() error { return e.cause }

func (e *invocableFlowActionCreateError) Error() string { return e.legacy.Error() }
func (e *invocableFlowActionCreateError) Unwrap() error { return e.legacy }

func invocableFlowActionRecordCreateError(objectName, node string, err error) error {
	legacy := dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow record create %s failed: %v", node, err)
	var cause dmlError
	if strings.EqualFold(objectName, "Account") && errors.As(err, &cause) {
		missingName := cause.code == "REQUIRED_FIELD_MISSING" && len(cause.fields) == 1 && cause.fields[0] == "Name"
		var inserted *invocableFlowActionInsertError
		callerError := errors.As(err, &inserted) && cause.code == "FIELD_CUSTOM_VALIDATION_EXCEPTION" && len(cause.fields) == 0
		if missingName || callerError {
			return &invocableFlowActionCreateError{legacy: legacy, cause: cause}
		}
	}
	return legacy
}

func (e *Engine) insertInvocableFlowRecord(record storage.Record) (storage.ID, error) {
	if e.InvocableFlowRecordCreator == nil {
		return e.insertOne(record, nil)
	}
	result, err := e.InvocableFlowRecordCreator(record)
	if err != nil {
		return "", err
	}
	if !result.Success {
		return "", &invocableFlowActionInsertError{cause: dmlError{code: result.StatusCode, fields: append([]string(nil), result.Fields...), message: result.Error}}
	}
	return result.ID, nil
}

func (e *Engine) updateInvocableFlowRecord(record storage.Record) error {
	if e.InvocableFlowRecordUpdater == nil {
		return e.updateOne(record)
	}
	result, err := e.InvocableFlowRecordUpdater(record)
	if err != nil {
		return err
	}
	if !result.Success {
		return dmlError{code: result.StatusCode, fields: append([]string(nil), result.Fields...), message: result.Error}
	}
	return nil
}

func (e *Engine) InvocableFlowActionError(err error) (Error, bool) {
	var create *invocableFlowActionCreateError
	if !errors.As(err, &create) {
		return Error{}, false
	}
	return Error{StatusCode: create.cause.code, Message: "This error occurred when the flow tried to create records: " + create.cause.code + ": " + create.cause.message + ". You can look up ExceptionCode values in the SOAP API Developer Guide."}, true
}

// invocableFlowRecordCriterion evaluates a decision condition whose left side
// is a field of a caller-supplied record variable, or a relationship such as
// record.RecordType.DeveloperName. A right side written {!variable.Field} is
// read from the frame. Scratch-org rows (API 62): Equals, Contains, null left
// operands and an unsaved record carrying only RecordTypeId.
func (e *Engine) invocableFlowRecordCriterion(item storage.WorkflowCriteriaItem, frame *flowFrame) (bool, bool, bool) {
	if frame == nil || e.Org == nil {
		return false, false, false
	}
	left := strings.TrimSpace(item.Field)
	value, ok := e.flowFrameReferenceValue(left, frame)
	if !ok {
		dot := strings.Index(left, ".")
		if dot <= 0 {
			return false, false, false
		}
		record, present := frame.records[flowFrameKey(left[:dot])]
		if !present {
			return false, false, false
		}
		value, ok = e.invocableFlowRecordPathValue(record, left[dot+1:])
		if !ok {
			return false, false, true
		}
	}
	want := item.Value
	rightNull := false
	trimmed := strings.TrimSpace(want)
	if strings.HasPrefix(trimmed, "{!") && strings.HasSuffix(trimmed, "}") {
		ref := strings.TrimSpace(trimmed[2 : len(trimmed)-1])
		rightValue, found := e.flowFrameReferenceValue(ref, frame)
		if !found {
			if name, path, nested := strings.Cut(ref, "."); nested {
				if right, present := frame.records[flowFrameKey(name)]; present {
					rightValue, found = e.invocableFlowRecordPathValue(right, path)
				}
			}
		}
		if !found {
			return false, false, true
		}
		want = ""
		if rightValue.Kind == storage.ValueNull {
			rightNull = true
		} else {
			want = workflowValueString(rightValue)
		}
		item.Value = "'" + want + "'"
	}
	if strings.EqualFold(strings.ReplaceAll(item.Operation, " ", ""), "contains") {
		// Native rows: Contains is false when either side is null, and true
		// for an empty string on the right.
		if value.Kind == storage.ValueNull || rightNull {
			return false, true, true
		}
		return strings.Contains(strings.ToLower(workflowValueString(value)), strings.ToLower(trimFormulaLiteral(item.Value))), true, true
	}
	matches, supported := evaluateFlowValueCriteria(value, item)
	return matches, supported, true
}

func (e *Engine) invocableFlowRecordPathValue(record storage.Record, path string) (storage.Value, bool) {
	path = strings.TrimSpace(path)
	if path == "" {
		return storage.Value{}, false
	}
	if rel, rest, nested := strings.Cut(path, "."); nested {
		if !strings.EqualFold(rel, "RecordType") {
			return storage.Value{}, false
		}
		id, present := sourceRecordFieldValue(record, "RecordTypeId")
		if !present || id.Kind == storage.ValueNull || id.ID == "" && id.String == "" {
			return storage.NullValue(), true
		}
		want := id.ID
		if want == "" {
			want = storage.ID(id.String)
		}
		canonical, found := storage.ResolveObjectName(*e.Org, record.Object)
		if !found {
			return storage.Value{}, false
		}
		for _, recordType := range e.Org.Objects[canonical].Definition.RecordTypes {
			if !storage.IDsEqual(recordType.ID, want) {
				continue
			}
			switch strings.ToLower(rest) {
			case "developername":
				return storage.StringValue(recordType.DeveloperName), true
			case "name":
				return storage.StringValue(recordType.Name), true
			}
			return storage.Value{}, false
		}
		return storage.NullValue(), true
	}
	value, present := sourceRecordFieldValue(record, path)
	if !present {
		return storage.NullValue(), true
	}
	return value, true
}
