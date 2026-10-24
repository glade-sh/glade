package dml

import (
	"html"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// Decision formulas keep their record/prior context while Get Records fields
// resolve from the interview. Resolving identifiers through the parser preserves
// quoted strings and lazy evaluation; an absent lookup row supplies null.
func (e *Engine) evaluateFlowDecisionFormula(expression string, source storage.Record, definition storage.ObjectDefinition, frame *flowFrame) (bool, bool) {
	parser := formulaParser{tokens: tokenizeFormula(html.UnescapeString(expression)), record: source, org: e.Org, definition: definition, namespace: formulaSourceNamespace("", definition), prior: e.flowPriorRecord(source.ID)}
	parser.flowReference = func(reference string) (formulaValue, bool, bool) {
		name, _, nested := strings.Cut(reference, ".")
		if frame == nil || !nested {
			return formulaValue{}, false, false
		}
		if _, found := frame.lookupOutputs[flowFrameKey(name)]; !found {
			return formulaValue{}, false, false
		}
		namespace := ""
		if e.Org != nil {
			namespace = e.Org.Namespace
		}
		value, null, ok := flowLookupSourceValue(reference, frame.lookupOutputs, namespace)
		if null {
			value = storage.NullValue()
		}
		return formulaStorageValue(value), true, ok
	}
	value, ok := parser.parseExpression()
	return value.truthy(), ok && parser.peek().typ == formulaTokenEOF
}
