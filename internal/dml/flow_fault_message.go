package dml

import (
	"errors"
	"sort"
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// $Flow.FaultMessage text of a failed Create Records, Update Records, Delete
// Records or Get Records element. Native rows: platform_event_flow_elements/faults
// (APIs 62.0 and 67.0, anonymous and @IsTest routes) cover event-triggered,
// record-triggered and autolaunched flows alike.
const flowFaultReferenceLink = "You can look up ExceptionCode values in the <a href='https://developer.salesforce.com/docs/atlas.en-us.api.meta/api/sforce_api_calls_concepts_core_data_objects.htm#'>SOAP API Developer Guide</a>."

// flowFaultCreateCodes are the status codes whose Create Records text a row
// captured: STRING_TOO_LONG (E01), REQUIRED_FIELD_MISSING (E02),
// FIELD_CUSTOM_VALIDATION_EXCEPTION (E03) and DUPLICATE_VALUE (E04). The
// reference-type codes come from flowCreateReferenceTypeCheck (E05, E06).
var flowFaultCreateCodes = map[string]bool{
	"STRING_TOO_LONG":                   true,
	"REQUIRED_FIELD_MISSING":            true,
	"FIELD_CUSTOM_VALIDATION_EXCEPTION": true,
	"DUPLICATE_VALUE":                   true,
	"DUPLICATES_DETECTED":               true,
}

func flowCreateFaultText(code, text string) string {
	return "This error occurred when the flow tried to create records: " + code + ": " + text + ". " + flowFaultReferenceLink
}

// flowRecordsFaultText is the text of a failed Update Records or Delete
// Records element (verb "update" or "delete") that touched ids.
func flowRecordsFaultText(verb string, ids []storage.ID, code, text string) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = string(id)
	}
	return "The flow tried to " + verb + " these records: " + strings.Join(parts, ", ") + ". This error occurred: " + code + ": " + text + ". " + flowFaultReferenceLink
}

// flowCreateError reports a Create Records element that the save rejected.
// flowFaultCurrentAPI reports API 67.0 or later for the running flow.
func (e *Engine) flowFaultCurrentAPI() bool {
	if e == nil || e.Org == nil {
		return false
	}
	raw := e.Org.APIVersion
	for _, rule := range e.Org.Metadata.Flows {
		if strings.EqualFold(rule.Name, e.activeFlowName) && strings.TrimSpace(rule.APIVersion) != "" {
			raw = rule.APIVersion
			break
		}
	}
	for _, object := range e.Org.Objects {
		for _, rule := range object.Definition.FlowRules {
			if strings.EqualFold(rule.Name, e.activeFlowName) && strings.TrimSpace(rule.APIVersion) != "" {
				raw = rule.APIVersion
			}
		}
	}
	version, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	return err == nil && version >= 67
}

func (e *Engine) flowFaultInTest() bool {
	return e != nil && e.FlowFaultInTest != nil && e.FlowFaultInTest()
}

func (e *Engine) flowCreateError(failure, cause error) error {
	var rejected dmlError
	if !errors.As(cause, &rejected) || !flowFaultCreateCodes[rejected.code] {
		return &flowElementFault{err: failure}
	}
	text := rejected.message
	if rejected.code == "DUPLICATE_VALUE" && e.flowFaultInTest() {
		// Native E04 and R04 (deliver route): a test run names neither the
		// field nor the record in the duplicate message.
		text = "duplicate value found: <unknown> duplicates value on record with id: <unknown>"
	}
	return &flowElementFault{err: failure, message: flowCreateFaultText(rejected.code, text), verb: "create", code: rejected.code, text: text, inTest: e.flowFaultInTest(), current: e.flowFaultCurrentAPI()}
}

// flowCreateTypedFault gives a Create Records reference-type rejection its
// fault text (native E05 FIELD_INTEGRITY_EXCEPTION and E06 MALFORMED_ID).
func (e *Engine) flowCreateTypedFault(err error) error {
	var typed *flowCreateReferenceTypeError
	if !errors.As(err, &typed) {
		return err
	}
	text := typed.label + ": id value of incorrect type: " + typed.value
	return &flowElementFault{err: err, message: flowCreateFaultText(typed.code, text), verb: "create", code: typed.code, text: text, inTest: e.flowFaultInTest(), current: e.flowFaultCurrentAPI()}
}

// flowUpdateError reports an Update Records element whose save failed. Only a
// single-record update has a captured text (native E07 and R07).
func (e *Engine) flowUpdateError(failure, cause error, ids []storage.ID) error {
	var rejected dmlError
	if len(ids) != 1 || !errors.As(cause, &rejected) || (rejected.code != "STRING_TOO_LONG" && rejected.code != "DUPLICATES_DETECTED") {
		return &flowElementFault{err: failure}
	}
	return &flowElementFault{err: failure, message: flowRecordsFaultText("update", ids, rejected.code, rejected.message), verb: "update", code: rejected.code, text: rejected.message, inTest: e.flowFaultInTest(), current: e.flowFaultCurrentAPI()}
}

// flowDeleteError reports a Delete Records element whose delete failed. Only a
// restricted lookup with one child record has a captured text (native E10
// and R10).
func (e *Engine) flowDeleteError(failure, cause error, objectName string, id storage.ID) error {
	var rejected dmlError
	if !errors.As(cause, &rejected) || rejected.code != "DELETE_FAILED" {
		return &flowElementFault{err: failure}
	}
	text, ok := e.restrictedDeleteText(objectName, id)
	if !ok {
		return &flowElementFault{err: failure}
	}
	return &flowElementFault{err: failure, message: flowRecordsFaultText("delete", []storage.ID{id}, rejected.code, text), verb: "delete", code: rejected.code, text: text, inTest: e.flowFaultInTest(), current: e.flowFaultCurrentAPI()}
}

// restrictedDeleteText renders the DELETE_FAILED text of a record that exactly
// one child record blocks through a restricted lookup.
func (e *Engine) restrictedDeleteText(objectName string, id storage.ID) (string, bool) {
	parent, ok := e.liveRecord(objectName, id)
	if !ok {
		return "", false
	}
	name := ""
	if value := parent.Fields["Name"]; value.Kind == storage.ValueString {
		name = value.String
	}
	var childNames []string
	label := ""
	for _, relation := range e.restrictedDeleteRelations(objectName, nil) {
		childObject := e.Org.Objects[relation.childObject]
		for _, childID := range e.referenceIndexForRelation(relation.childObject, relation.field, nil)[id] {
			child, found := childObject.Records[childID]
			if !found || child.System.IsDeleted {
				continue
			}
			if value, set := child.Fields[relation.field]; !set || idFromStorageValue(value) != id {
				continue
			}
			item := ""
			if value := child.Fields["Name"]; value.Kind == storage.ValueString {
				item = value.String
			}
			childNames = append(childNames, item)
			label = strings.ToLower(childObject.Definition.PluralLabel)
		}
	}
	if len(childNames) != 1 || label == "" {
		return "", false
	}
	sort.Strings(childNames)
	return "Your attempt to delete " + name + " could not be completed because it is associated with the following " + label + ".: " + childNames[0] + "\n", true
}

// flowDeleteErrorOf is flowDeleteError for a loop over ids: only a delete of
// exactly one record has a captured text.
func (e *Engine) flowDeleteErrorOf(failure, cause error, objectName string, id storage.ID, count int) error {
	if count != 1 {
		return &flowElementFault{err: failure}
	}
	return e.flowDeleteError(failure, cause, objectName, id)
}

func flowRecordIDs(records []storage.Record) []storage.ID {
	ids := make([]storage.ID, len(records))
	for i, record := range records {
		ids[i] = record.ID
	}
	return ids
}

// flowLookupFault faults a Get Records element that filters one Id field with a
// literal that is not an Id (native E13 and R13 and the unhandled E14 and R14:
// the query fails with "invalid ID field"). The text spells the query that
// Flow built, which selects only Id here because nothing reads other fields.
// Any other shape keeps its earlier behavior.
func (e *Engine) flowLookupFault(lookup storage.FlowRecordLookup) error {
	if e == nil || e.Org == nil || !lookup.GetFirstRecordOnly || len(lookup.Criteria) != 1 ||
		len(lookup.QueriedFields) != 0 || len(lookup.OutputAssignments) != 0 || strings.TrimSpace(lookup.OutputReference) != "" ||
		strings.TrimSpace(lookup.SortField) != "" {
		return nil
	}
	item := lookup.Criteria[0]
	value := item.Value
	if !strings.EqualFold(strings.TrimSpace(item.Field), "Id") || !strings.EqualFold(strings.TrimSpace(item.Operation), "equals") ||
		item.SourceField != "" || value == "" || strings.ContainsAny(value, "'\\\n") || storage.ValidateID(storage.ID(value)) == nil {
		return nil
	}
	objectName, ok := storage.ResolveObjectName(*e.Org, lookup.ObjectName)
	if !ok {
		return nil
	}
	prefix := "SELECT Id FROM " + objectName + " WHERE (("
	query := prefix + "Id = '" + value + "'))  LIMIT 1"
	text := "\n" + query + "\n" + strings.Repeat(" ", len(prefix)) + "^\nERROR at Row:1:Column:" + strconv.Itoa(len(prefix)+1) + "\ninvalid ID field: " + value
	failure := dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow record lookup %s failed: invalid ID field: %s", lookup.Name, value)
	return &flowElementFault{err: failure, message: "This error occurred when the flow tried to look up records: " + text + ". " + flowFaultReferenceLink, verb: "look up", text: text, inTest: e.flowFaultInTest(), current: e.flowFaultCurrentAPI()}
}

const flowUnhandledGeneric = "An unhandled fault has occurred while processing the flow.  Please contact your system administrator for more information."

// flowUnhandledTriggerError is the failure of a record-triggered after-save
// flow whose Create, Update, Delete or Get Records element faulted with no fault
// connector. Native R08, R11, R14, R16 and R17 (APIs 62.0 and 67.0, anonymous
// and deliver routes): the statement fails with the element's own status code
// and text, except a failed Get Records, which reports the generic fault. A
// delete, a lookup or a duplicate-rule rejection (rows in
// automation_residual_paths) adds an Error ID outside tests.
func flowUnhandledTriggerError(rule storage.FlowRule, fault *flowElementFault) (error, bool) {
	if !fault.unhandledCaptured() {
		return nil, false
	}
	errorID := ""
	if !fault.inTest && (fault.verb == "delete" || fault.verb == "look up" || fault.code == "DUPLICATES_DETECTED") {
		errorID = " Error ID: 0-0 (0)"
	}
	if fault.verb == "look up" {
		message := processFailureMessage(rule, "<b>An unhandled fault has occurred in this flow</b><br>"+flowUnhandledGeneric+errorID)
		return &flowCoreRecordCreateFailure{cause: dmlErrorf("CANNOT_EXECUTE_FLOW_TRIGGER", nil, "%s", message)}, true
	}
	detail := "This error occurred when the flow tried to " + fault.verb + " records: " + fault.code + ": " + fault.text +
		". You can look up ExceptionCode values in the SOAP API Developer Guide."
	if fault.current {
		detail = fault.message
	}
	message := processFailureMessage(rule, "CANNOT_EXECUTE_FLOW_TRIGGER: "+detail+errorID)
	return &flowCoreRecordCreateFailure{cause: dmlErrorf(fault.code, nil, "%s", message)}, true
}

// FlowUnhandledDetail is the FlowException message that Flow.Interview.start
// raises when an element faulted with no fault connector. Native anonymous-run rows
// for these elements: a test sees the element's text, any other run the generic text.
func FlowUnhandledDetail(err error) (string, bool) {
	var fault *flowElementFault
	if !errors.As(err, &fault) || !fault.unhandledCaptured() {
		return "", false
	}
	if !fault.inTest {
		return " An unhandled fault has occurred in this flow  " + flowUnhandledGeneric, true
	}
	if fault.verb == "look up" {
		// A native anonymous-run row words this link differently from the other elements.
		return "This error occurred when the flow tried to look up records: " + fault.text +
			". You can look up ExceptionCode values in the <a href='https://developer.salesforce.com/docs/atlas.en-us.api.meta/api/sforce_api_calls_concepts_core_data_objects.htm#'>SOAP API Developer Guide .", true
	}
	if fault.current {
		return strings.Replace(fault.message, "</a>", " ", 1), true
	}
	return "This error occurred when the flow tried to " + fault.verb + " records: " + fault.code + ": " + fault.text +
		". You can look up ExceptionCode values in the SOAP API Developer Guide.", true
}

// unhandledCaptured is true for the element failures whose unhandled form has
// native rows: Update, Delete and Get Records, and a Create Records rejected with
// STRING_TOO_LONG (R16 and R17). Rows 01-06 cover the other Create codes
// and the reference-type rejections only with a fault connector, so those keep
// their earlier behavior when nothing catches them.
func (f *flowElementFault) unhandledCaptured() bool {
	if f.verb == "" {
		return false
	}
	return f.verb != "create" || f.code == "STRING_TOO_LONG" || f.code == "DUPLICATES_DETECTED"
}
