package sosl

import "testing"

// Salesforce SOSL WHERE documents unquoted dates using SOQL date formats:
// https://developer.salesforce.com/docs/platform/salesforce-soql-sosl/guide/sforce-api-calls-sosl-where.html
// https://developer.salesforce.com/docs/platform/salesforce-soql-sosl/guide/sforce-api-calls-soql-select-dateformats.html
func TestParseSOSLReturningISODateValues(t *testing.T) {
	for _, literal := range []string{
		"2025-04-18",
		"2025-04-18T12:34:56Z",
		"2025-04-18T12:34:56+05:30",
		"2025-04-18T12:34:56-08:00",
		"2025-04-18T12:34:56.123Z",
		"2025-04-18T12:34:56.123+05:30",
	} {
		t.Run(literal, func(t *testing.T) {
			query, err := Parse("FIND :term IN ALL FIELDS RETURNING Account(Id WHERE CreatedDate >= " + literal + " AND Name != 'excluded' ORDER BY CreatedDate DESC LIMIT 25)")
			if err != nil {
				t.Fatal(err)
			}
			returning := query.Returning[0]
			if returning.Where == nil || len(returning.Where.And) != 2 {
				t.Fatalf("WHERE = %#v", returning.Where)
			}
			date := returning.Where.And[0]
			if date.Field != "CreatedDate" || date.Operator != ">=" || date.Value != literal || date.ValueQuoted || date.Bind != "" {
				t.Fatalf("date condition = %#v", date)
			}
			if len(returning.Fields) != 1 || returning.Fields[0].Field != "Id" || len(returning.OrderBy) != 1 || !returning.OrderBy[0].Desc || returning.Limit.Value != 25 {
				t.Fatalf("RETURNING clauses = %#v", returning)
			}
		})
	}
}

func TestParseSOSLDateLexingControls(t *testing.T) {
	for _, literal := range []string{"-12", "+12", "-12.5", "0", "25.5", ":cutoff", "'2025-04-18T12:34:56Z'"} {
		t.Run(literal, func(t *testing.T) {
			if _, err := Parse("FIND :term RETURNING Account(Id WHERE AnnualRevenue >= " + literal + " LIMIT 10)"); err != nil {
				t.Fatal(err)
			}
		})
	}
	for _, malformed := range []string{
		"2025-04-18suffix", "2025-04-18T12:34:56Z.Name", "2025-02-30",
		"2025-04-18T25:34:56Z", "2025-04-18T1:34:56Z",
		"2025-04-18T12:34:56+24:00", "2025-04-18T12:34:56+05:60",
	} {
		t.Run(malformed, func(t *testing.T) {
			if _, err := Parse("FIND :term RETURNING Account(Id WHERE CreatedDate >= " + malformed + ")"); err == nil {
				t.Fatal("malformed date or identifier accepted")
			}
		})
	}
	// Existing numeric lexing stays responsible for exponent-shaped input;
	// the date scanner must not absorb it into a date/identifier token.
	for _, text := range []string{"-1.25e+3", "+1.25e-3"} {
		tokens := lex(text)
		if tokens[0].kind != tokenNumber || tokens[0].text != text[:5] {
			t.Fatalf("numeric prefix of %q = %#v", text, tokens[0])
		}
	}
}
