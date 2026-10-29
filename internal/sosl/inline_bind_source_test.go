package sosl

import (
	"strings"
	"testing"
)

// Native API62/67 inline_index/inline_call and dynamic_mixed rows in
// apextest/testdata/conformance/soql_quoted_numeric_binds/org.tsv establish the
// accepted operand shapes. These controls check source retention and offsets.
func TestInlineSOSLBindSource(t *testing.T) {
	for _, operand := range []string{"terms[0]", "terms.get(0)", " terms . get(0)"} {
		query, err := ParseInline("FIND :" + operand + " RETURNING Account(Id WHERE Name IN (1,:needle))")
		if err != nil {
			t.Fatal(err)
		}
		if query.SearchBind != strings.TrimSpace(operand) {
			t.Fatalf("bind=%q operand=%q", query.SearchBind, operand)
		}
		where := query.Returning[0].Where
		if len(where.Values) != 2 || where.Values[0] != "1" || where.Values[1] != ":needle" || !where.UnquotedValues[0] || where.UnquotedValues[1] {
			t.Fatalf("literal/bind source lost: %+v", where)
		}
	}
	query, err := ParseInline("FIND 'quiet:terms[0]' RETURNING Account(Id WHERE Name = :terms.get(0))")
	if err != nil {
		t.Fatal(err)
	}
	if query.SearchBind != "" || query.SearchText != "quiet:terms[0]" || query.Returning[0].Where.Bind != "terms.get(0)" {
		t.Fatalf("quoted term or WHERE bind changed: %+v", query)
	}
	for _, operand := range []string{"holder?.Name", "terms[0]?.Name", "flag ? 'first' : 'second'"} {
		if _, err := ParseInline("FIND :" + operand + " RETURNING Account(Id)"); err == nil {
			t.Fatalf("uncaptured expression became accepted: %s", operand)
		}
	}
}
