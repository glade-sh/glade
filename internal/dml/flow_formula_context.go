package dml

import (
	"html"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// Flow's $User.Id is a 15-character String, unlike the Id projected by Get
// Records. Keep the observed representation at this Flow resource boundary.
func (e *Engine) flowCurrentUserID() (storage.Value, bool) {
	id := e.systemUserID()
	if storage.ValidateID(id) != nil {
		return storage.Value{}, false
	}
	return storage.StringValue(string(id)[:15]), true
}

func (e *Engine) flowContextFormulaValue(expression string, frame *flowFrame) (storage.Value, bool) {
	if frame == nil {
		return storage.Value{}, false
	}
	record := flowFrameFormulaRecord(frame)
	tokens := tokenizeFormula(html.UnescapeString(expression))
	bound := make([]formulaToken, 0, len(tokens))
	for i := 0; i < len(tokens); i++ {
		token := tokens[i]
		if token.typ == formulaTokenIdent && i+2 < len(tokens) && tokens[i+1].typ == formulaTokenSymbol && tokens[i+1].text == ":" && tokens[i+2].typ == formulaTokenIdent {
			token.text += ":" + tokens[i+2].text
			i += 2
		}
		bound = append(bound, token)
	}
	resolve := func(reference string) (formulaValue, bool, bool) {
		if strings.EqualFold(reference, "$User.Id") {
			value, ok := e.flowCurrentUserID()
			return formulaStorageValue(value), true, ok
		}
		if strings.Contains(reference, ":") {
			value, ok := e.flowTypedUserID(reference, frame)
			return formulaStorageValue(value), true, ok
		}
		if e.InvocableFlow {
			if value, ok := e.flowFrameReferenceValue(reference, frame); ok {
				return formulaStorageValue(value), true, true
			}
		}
		return formulaValue{}, false, false
	}
	value, _, ok := evaluateFlowFormulaParser(formulaParser{tokens: bound, record: record, flowTextNulls: true, flowReference: resolve})
	return value, ok
}

// Flow text equality treats null and empty text alike without changing the
// projected resource value. Numeric, ordered and stored-formula comparisons
// retain their existing rules.
func compareFlowTextNulls(left, right formulaValue, operator string) (bool, bool) {
	if left.kind != formulaNull && right.kind != formulaNull {
		return false, false
	}
	if left.kind != formulaNull && left.kind != formulaString || right.kind != formulaNull && right.kind != formulaString {
		return false, false
	}
	switch operator {
	case "=":
		return left.asString() == right.asString(), true
	case "!=":
		return left.asString() != right.asString(), true
	}
	return false, false
}

// Resolve only the observed lookup-resource relationship:User.Id shape. The
// relationship and allowed target types come from metadata, never field names.
func (e *Engine) flowTypedUserID(reference string, frame *flowFrame) (storage.Value, bool) {
	left, right, typed := strings.Cut(reference, ":")
	if !typed || !strings.EqualFold(right, "User.Id") || e.Org == nil || frame == nil {
		return storage.Value{}, false
	}
	dot := strings.LastIndex(left, ".")
	if dot <= 0 || dot == len(left)-1 {
		return storage.Value{}, false
	}
	output, ok := frame.lookupOutputs[flowFrameKey(left[:dot])]
	if !ok {
		return storage.Value{}, false
	}
	relationship := left[dot+1:]
	for apiName, field := range output.definition.Fields {
		if field.Type != storage.FieldReference || !dmlRelationshipNameMatches(e.Org.Namespace, storage.ParentRelationshipName(field), relationship) {
			continue
		}
		allowed := map[string]bool{}
		for _, target := range field.ReferenceTo {
			allowed[strings.ToLower(target)] = true
		}
		if len(allowed) != 2 || !allowed["user"] || !allowed["group"] {
			return storage.Value{}, false
		}
		value, exists := sourceRecordFieldValue(output.record, apiName)
		if !exists || value.Kind == storage.ValueNull {
			return storage.NullValue(), true
		}
		id := idFromStorageValue(value)
		if storage.ValidateID(id) != nil {
			return storage.Value{}, false
		}
		for _, target := range field.ReferenceTo {
			canonical, found := storage.ResolveObjectName(*e.Org, target)
			if !found {
				continue
			}
			definition := e.Org.Objects[canonical].Definition
			if definition.KeyPrefix == "" || !strings.HasPrefix(string(id), definition.KeyPrefix) {
				continue
			}
			if strings.EqualFold(canonical, "User") {
				return value.Clone(), true
			}
			return storage.NullValue(), true
		}
		return storage.Value{}, false
	}
	return storage.Value{}, false
}
