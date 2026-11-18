package dml

import (
	"html"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// Decision formulas keep their record/prior context while Get Records fields
// resolve from the interview. Resolving identifiers through the parser preserves
// quoted strings and lazy evaluation; an absent lookup row supplies null. A
// stored formula field reads finished (flow_assign_routes, API 62.0 and 67.0:
// LEN of an after-save $Record formula with BR() is 3).
func (e *Engine) evaluateFlowDecisionFormula(expression string, source storage.Record, definition storage.ObjectDefinition, frame *flowFrame) (bool, bool) {
	parser := formulaParser{tokens: tokenizeFormula(html.UnescapeString(expression)), record: source, org: e.Org, definition: definition, namespace: formulaSourceNamespace("", definition), prior: e.flowPriorRecord(source.ID), userID: e.systemUserID(), permissionUserFields: e.PermissionUserFields, flowRecordContext: true, flowFinishFields: true}
	// process_builder_shapes pri rows: ISNEW() in a criterion reads the creating save.
	parser.isNew = frame != nil && (frame.processInterview || e.capturedMixedAfterSave) && frame.createDecision
	parser.flowReference = func(reference string) (formulaValue, bool, bool) {
		if frame != nil && frame.processInterview && strings.EqualFold(reference, "$User.Alias") && e.Org != nil {
			_, user, found := storage.LookupRecordByID(e.Org.Objects["User"].Records, e.systemUserID())
			if !found {
				return formulaValue{}, true, false
			}
			alias, found := sourceRecordFieldValue(user, "Alias")
			return formulaStorageValue(alias), true, found
		}
		name, _, nested := strings.Cut(reference, ".")
		if frame == nil {
			return formulaValue{}, false, false
		}
		if !nested {
			// An interview variable is declared in the frame; a record field of
			// the triggering source never is, so the two cannot be confused.
			if value, found := frame.scalars[flowFrameKey(reference)]; found {
				return formulaStorageValue(value.Clone()), true, true
			}
			if frame.capturedBeforeCreate {
				variable, declared := frame.variables[flowFrameKey(reference)]
				if declared && !variable.IsCollection && variable.DefaultValue == nil && strings.EqualFold(variable.DataType, "String") {
					return formulaStorageValue(storage.NullValue()), true, true
				}
			}
			return formulaValue{}, false, false
		}
		output, found := frame.lookupOutputs[flowFrameKey(name)]
		if !found {
			return formulaValue{}, false, false
		}
		namespace := ""
		if e.Org != nil {
			namespace = e.Org.Namespace
		}
		value, null, ok := flowLookupSourceValue(reference, frame.lookupOutputs, namespace)
		if !ok {
			// Get Records outputs also expose relationship fields such as
			// RecordType.DeveloperName through the record's own schema.
			if related, resolved := e.flowRefsRelationshipValue(reference[len(name)+1:], output.record, output.definition); resolved {
				value, null, ok = related, related.Kind == storage.ValueNull, true
			}
		}
		if null {
			value = storage.NullValue()
		}
		return formulaStorageValue(value), true, ok
	}
	value, ok := parser.parseExpression()
	return value.truthy(), ok && parser.peek().typ == formulaTokenEOF
}
