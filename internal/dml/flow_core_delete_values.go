package dml

import (
	"errors"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

type flowCoreDeleteCriteriaError struct{}

func (*flowCoreDeleteCriteriaError) Error() string {
	return "No records in Salesforce match your delete criteria."
}

// The observed no-match boundary is a conjunctive three-String Equals filter.
// Collection deletes and other criteria keep the existing behavior.
func flowCoreDeleteCriteriaShape(del storage.FlowRecordDelete, definition storage.ObjectDefinition, namespace string) bool {
	if del.InputReference != "" || len(del.Criteria) != 3 {
		return false
	}
	for _, item := range del.Criteria {
		field, ok := storage.ResolveFieldName(definition, namespace, item.Field)
		if !ok || definition.Fields[field].Type != storage.FieldString {
			return false
		}
		switch strings.ToLower(strings.TrimSpace(item.Operation)) {
		case "equals":
		default:
			return false
		}
	}
	return true
}

func flowCoreRecordTriggerError(rule storage.FlowRule, err error) error {
	var noMatch *flowCoreDeleteCriteriaError
	if !strings.EqualFold(rule.TriggerType, "RecordAfterSave") {
		return err
	}
	var unknownOutput *flowCoreUnknownSubflowOutputError
	var unstoredInput *flowUnstoredSubflowInputFieldError
	if errors.As(err, &unknownOutput) || errors.As(err, &unstoredInput) {
		return &flowCoreSubflowOutputFailure{cause: dmlErrorf("CANNOT_EXECUTE_FLOW_TRIGGER", nil, "We can't save this record because the “%s” process failed. Give your Salesforce admin these details. <b>An unhandled fault has occurred in this flow</b><br>An unhandled fault has occurred while processing the flow.  Please contact your system administrator for more information.", rule.Name)}
	}
	if !errors.As(err, &noMatch) {
		return err
	}
	return &flowCoreRecordDeleteFailure{cause: dmlErrorf("CANNOT_EXECUTE_FLOW_TRIGGER", nil, "We can't save this record because the “%s” process failed. Give your Salesforce admin these details. %s", rule.Name, noMatch.Error())}
}

type flowCoreRecordDeleteFailure struct{ cause error }

func (e *flowCoreRecordDeleteFailure) Error() string { return e.cause.Error() }
func (e *flowCoreRecordDeleteFailure) Unwrap() error { return e.cause }

// FlowRecordDeleteFailureResult identifies a deferred no-match failure while
// preserving the row error used by direct engine execution.
func FlowRecordDeleteFailureResult(err error) (Result, bool) {
	var failure *flowCoreRecordDeleteFailure
	if !errors.As(err, &failure) {
		return Result{}, false
	}
	return resultFromError("", failure.cause), true
}
