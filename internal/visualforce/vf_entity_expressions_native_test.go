package visualforce_test

import "testing"

// Synthetic pages with HTML entities inside and outside {!...} expressions, in
// attribute values and in markup text, captured with gen.py/run.sh. Natively an
// entity outside a string literal in an expression is a syntax error. In an
// attribute value or a string literal a known HTML 4 name (and apos) becomes its
// decimal reference as text, and everything else stays as typed; the value is
// then escaped on output. Markup text decodes entities normally.
func TestVisualforceEntityExpressionNativeRows(t *testing.T) {
	runPosFamily(t, "testdata/vf_entity_expressions", "", "", []string{"59.0", "62.0", "67.0", "68.0"}, nil)
}
