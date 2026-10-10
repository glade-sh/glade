package sosl_test

import (
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/sosl"
)

func TestSOSLCoreContractFamily(t *testing.T) {
	t.Run("clause grammar and order", func(t *testing.T) {
		query, err := sosl.Parse("FIND {Acme} IN NAME FIELDS RETURNING Account(Id, Name WHERE Name = 'Acme') LIMIT 5")
		if err != nil {
			t.Fatalf("Parse valid ordered query: %v", err)
		}
		if query.Scope != sosl.SearchScopeName || len(query.Returning) != 1 || query.Limit != (sosl.Window{Value: 5, HasValue: true}) {
			t.Fatalf("parsed ordered query = %#v", query)
		}
		if query.Returning[0].Where == nil || query.Returning[0].Where.Field != "Name" {
			t.Fatalf("RETURNING WHERE = %#v", query.Returning[0].Where)
		}

		if _, err := sosl.Parse("FIND {Acme} RETURNING Account(Id) IN NAME FIELDS"); err == nil {
			t.Fatal("Parse accepted search scope after RETURNING")
		}
	})

	t.Run("FIND operators, grouping, wildcards, and escapes", func(t *testing.T) {
		query, err := sosl.Parse("FIND {(Alpha OR Beta) AND Gamma*} RETURNING Contact(Id)")
		if err != nil {
			t.Fatalf("Parse grouped FIND expression: %v", err)
		}
		if !reflect.DeepEqual(query.Terms, []sosl.SearchTerm{{Text: "Alpha"}, {Text: "Beta"}, {Text: "Gamma", Prefix: true}}) {
			t.Fatalf("FIND terms = %#v", query.Terms)
		}
		if query.Expression == nil || query.Expression.Operator != "AND" || query.Expression.Left == nil || query.Expression.Left.Operator != "OR" {
			t.Fatalf("FIND precedence/grouping = %#v", query.Expression)
		}

		negative, err := sosl.Parse("FIND {Alpha AND NOT Beta} RETURNING Contact(Id)")
		if err != nil {
			t.Fatalf("Parse AND NOT FIND expression: %v", err)
		}
		if negative.Expression == nil || negative.Expression.Operator != "AND NOT" || !reflect.DeepEqual(negative.Terms, []sosl.SearchTerm{{Text: "Alpha"}, {Text: "Beta"}}) {
			t.Fatalf("AND NOT expression = %#v", negative.Expression)
		}

		literal, err := sosl.Parse(`FIND {Literal\*} RETURNING Contact(Id)`)
		if err != nil {
			t.Fatalf("Parse escaped wildcard as a literal: %v", err)
		}
		if !reflect.DeepEqual(literal.Terms, []sosl.SearchTerm{{Text: "Literal*"}}) {
			t.Fatalf("escaped wildcard terms = %#v", literal.Terms)
		}
		for _, tc := range []struct{ query, want string }{
			{`FIND {Literal\*\*} RETURNING Contact(Id)`, "Literal**"},
			{`FIND '"Literal\*\*"' RETURNING Contact(Id)`, "Literal**"},
			{"FIND 'Literal\\*\uE000\\*' RETURNING Contact(Id)", "Literal*\uE000*"},
			{`FIND '"Literal\*Token"' RETURNING Contact(Id)`, "Literal*Token"},
			{"FIND 'Literal\uE000Token' RETURNING Contact(Id)", "Literal\uE000Token"},
			{"FIND '\"Literal\uE000Token\"' RETURNING Contact(Id)", "Literal\uE000Token"},
		} {
			parsed, err := sosl.Parse(tc.query)
			if err != nil || !reflect.DeepEqual(parsed.Terms, []sosl.SearchTerm{{Text: tc.want}}) {
				t.Errorf("Parse(%q) terms=%#v err=%v", tc.query, parsed.Terms, err)
			}
		}
		rightAssociated, err := sosl.Parse("FIND {Red AND NOT Blue AND Alpha} RETURNING Contact(Id)")
		if err != nil || rightAssociated.Expression == nil || rightAssociated.Expression.Operator != "AND NOT" || rightAssociated.Expression.Right == nil || rightAssociated.Expression.Right.Operator != "AND" {
			t.Fatalf("right-associated AND NOT = %#v, err=%v", rightAssociated.Expression, err)
		}
	})

	t.Run("RETURNING filters, logic, and escapes", func(t *testing.T) {
		for _, operator := range []string{"=", "!=", "<", "<=", ">", ">=", "LIKE"} {
			t.Run("operator "+operator, func(t *testing.T) {
				query, err := sosl.Parse("FIND {Acme} RETURNING Account(Id, Name WHERE Name " + operator + " 'M')")
				if err != nil {
					t.Fatalf("Parse SOSL WHERE operator %s: %v", operator, err)
				}
				condition := query.Returning[0].Where
				if condition == nil || condition.Field != "Name" || condition.Operator != operator {
					t.Fatalf("SOSL WHERE condition = %#v, want operator %s", condition, operator)
				}
			})
		}

		compound, err := sosl.Parse(`FIND {Acme} RETURNING Account(Id, Name WHERE (Name = 'Acme' OR Name = 'Beta') AND Name != 'Other')`)
		if err != nil {
			t.Fatalf("Parse parenthesized SOSL WHERE logic: %v", err)
		}
		if compound.Returning[0].Where == nil {
			t.Fatal("parenthesized SOSL WHERE was not retained")
		}

		for _, tc := range []struct {
			literal string
			want    string
		}{
			{literal: `O\'Brien`, want: "O'Brien"},
			{literal: `C:\\Temp`, want: `C:\Temp`},
		} {
			query, err := sosl.Parse("FIND {Acme} RETURNING Account(Id, Name WHERE Name = '" + tc.literal + "')")
			if err != nil {
				t.Fatalf("Parse escaped SOSL WHERE value %q: %v", tc.literal, err)
			}
			if got := query.Returning[0].Where.Value; got != tc.want {
				t.Errorf("SOSL WHERE value = %q, want %q", got, tc.want)
			}
		}

		for _, query := range []string{
			"FIND {Acme} RETURNING Account(Id) LIMIT 2.5",
			"FIND {Acme} RETURNING Account(Id OFFSET 1.5)",
			"FIND {Acme} RETURNING Account(Id) LIMIT -1",
			"FIND {Acme} RETURNING Account(Id, Name WHERE Name CONTAINS 'A')",
			"FIND {Acme} RETURNING Account(Id, Name WHERE FORMAT(Name) = 'A')",
		} {
			if _, err := sosl.Parse(query); err == nil {
				t.Errorf("Parse accepted unsupported SOSL WHERE expression %q", query)
			}
		}
	})
}
