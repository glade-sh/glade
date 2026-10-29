//go:build cgo

package apexast

import (
	"strings"
	"testing"
)

func TestNormalizeInlineSOQLFormulaPredicateRoutes(t *testing.T) {
	tests := []struct {
		name   string
		source string
		calls  []string
	}{
		{
			name:   "anonymous_inline_query",
			source: `List<Account> rows = [SELECT Id FROM Account WHERE FORMULA('LEN(Name)') > 3];`,
			calls:  []string{`FORMULA('LEN(Name)')`},
		},
		{
			name:   "class_method_grouped_boolean_predicates",
			source: `class Grove { void run() { List<Account> rows = [SELECT Id FROM Account WHERE NOT (formula('LEN(Name)') < 2 OR FORMULA('Name = \'Oak\'') = true) AND Name != null ORDER BY Name LIMIT 2]; } }`,
			calls:  []string{`formula('LEN(Name)')`, `FORMULA('Name = \'Oak\'')`},
		},
		{
			name:   "multiple_inline_queries",
			source: `List<Account> a=[SELECT Id FROM Account WHERE FORMULA('true')=true]; List<Contact> b=[select Id from Contact where FoRmUlA('false')=false];`,
			calls:  []string{`FORMULA('true')`, `FoRmUlA('false')`},
		},
		{
			name:   "comments_whitespace_and_multibyte_literal",
			source: "List<Account> rows = [SELECT Id FROM Account WHERE FORMULA /* operand */ (\r\n 'Name = \\'Chêne\\'' /* end */\n) = true];",
			calls:  []string{"FORMULA /* operand */ (\r\n 'Name = \\'Chêne\\'' /* end */\n)"},
		},
		{
			name:   "comments_strings_dynamic_queries_and_other_brackets",
			source: "// [SELECT Id FROM Account WHERE FORMULA('true')=true]\n/* [SELECT Id FROM Account WHERE FORMULA('false')=false] */\nString q='[SELECT Id FROM Account WHERE FORMULA(\\'true\\')=true]'; Object x=rows[FORMULA('true')];",
		},
		{
			name:   "select_list_and_non_where_clauses",
			source: `[SELECT FORMULA('Name') FROM Account ORDER BY FORMULA('Name') LIMIT 1]`,
		},
		{
			name:   "qualified_fields_binds_and_nested_function_arguments",
			source: `[SELECT Id FROM Account WHERE Parent.FORMULA('true')=true OR Id=:FORMULA('true') OR DAY_ONLY(FORMULA('true'))=null]`,
		},
		{
			name:   "subquery_does_not_supply_outer_where_operand",
			source: `[SELECT Id,(SELECT Id FROM Contacts WHERE FORMULA('true')=true) FROM Account WHERE Name='FORMULA(\'true\')']`,
		},
		{
			name:   "malformed_calls_and_unclosed_query",
			source: `[SELECT Id FROM Account WHERE FORMULA(Name)=true OR FORMULA('true',Name)=true]; [SELECT Id FROM Account WHERE FORMULA('true')=true`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := tt.source
			for _, call := range tt.calls {
				// Expected operands occupy the same bytes and retain line endings.
				mask := []byte(strings.Repeat(" ", len(call)))
				copy(mask, "Id")
				for i := range call {
					if call[i] == '\r' || call[i] == '\n' {
						mask[i] = call[i]
					}
				}
				want = strings.ReplaceAll(want, call, string(mask))
			}
			got := normalizeInlineSOQLFormula(tt.source)
			if got != want {
				t.Fatalf("normalized source:\n%q\nwant:\n%q", got, want)
			}
			if len(got) != len(tt.source) {
				t.Fatalf("source bytes = %d, want %d", len(got), len(tt.source))
			}
			if again := normalizeInlineSOQLFormula(got); again != got {
				t.Fatal("normalization is not idempotent")
			}
		})
	}
}

func TestInlineSOQLFormulaCallEnd(t *testing.T) {
	for _, tt := range []struct {
		name string
		call string
		want int
	}{
		{name: "literal", call: "('true') trailing", want: len("('true')")},
		{name: "escaped_quote_and_parenthesis", call: "('Name = \\'Oak)\\'') = true", want: len("('Name = \\'Oak)\\'')")},
		{name: "comments", call: " /*a*/ ( /*b*/ 'true' /*c*/ ) tail", want: len(" /*a*/ ( /*b*/ 'true' /*c*/ )")},
		{name: "missing_open", call: "'true')", want: -1},
		{name: "nonliteral", call: "(Name)", want: -1},
		{name: "multiple_arguments", call: "('true',Name)", want: -1},
		{name: "missing_close", call: "('true'", want: -1},
		{name: "unterminated_string", call: "('true)", want: -1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			source := "FORMULA" + tt.call
			got := inlineSOQLFormulaCallEnd(source, len("FORMULA"), len(source))
			want := tt.want
			if want >= 0 {
				want += len("FORMULA")
			}
			if got != want {
				t.Fatalf("call end = %d, want %d", got, want)
			}
		})
	}
}
