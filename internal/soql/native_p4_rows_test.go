package soql

import (
	"errors"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Rows below come from anonymous Apex run on a scratch org at API 62.0, 67.0
// and 68.0 (identical at all three). Each query was run through
// Database.query against a custom object with a Name text field, an Active__c
// checkbox and an Ord__c number.

func TestParseAcceptsStringLiteralGluedToWordOperator(t *testing.T) {
	// Native accepted LIKE with the string literal glued to the keyword.
	for _, where := range []string{
		"Name like'%Leader'",
		"Name LIKE'%Leader'",
		"Name like'%Leader' AND Name != 'zz'",
		"NOT Name like'%Leader'",
		"Name IN('abc','Other')",
		"Name IN ('abc')AND Name='abc'",
		"Name = 'abc'OR Name='Other'",
	} {
		t.Run(where, func(t *testing.T) {
			if _, err := Parse("SELECT Name FROM Parent__c WHERE " + where); err != nil {
				t.Fatalf("Parse(%q): %v", where, err)
			}
		})
	}
	query, err := Parse("SELECT Name FROM Parent__c WHERE Name like'%Leader'")
	if err != nil {
		t.Fatal(err)
	}
	if query.Where == nil || !strings.EqualFold(query.Where.Op, "LIKE") || query.Where.Value.Kind != storage.ValueString || query.Where.Value.String != "%Leader" {
		t.Fatalf("where = %#v", query.Where)
	}
}

func TestParseAcceptsNegatedBooleanBind(t *testing.T) {
	// Native bound `Active__c = :!b` and `:(!b)` to the negated Boolean.
	for _, row := range [][2]string{{":!b", ":!b"}, {":(!b)", ":(!b)"}, {":-n", ":-n"}, {":! b", ":!b"}} {
		bind, want := row[0], row[1]
		t.Run(bind, func(t *testing.T) {
			query, err := Parse("SELECT Id FROM Child__c WHERE Active__c = " + bind)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if query.Where == nil || query.Where.Value.Kind != storage.ValueID || string(query.Where.Value.ID) != want {
				t.Fatalf("where = %#v", query.Where)
			}
		})
	}
	if _, err := Parse("SELECT Id FROM Child__c WHERE Active__c = true AND ((Active__c = :b AND Ord__c > 0) OR (Active__c = :!b AND Ord__c > 1))"); err != nil {
		t.Fatalf("nested negated bind: %v", err)
	}
}

func TestParseNotPlacementMatchesNative(t *testing.T) {
	accept := []string{
		"NOT Name = 'x1'",
		"(NOT Name = 'x1') AND Active__c = true",
		"NOT (Name = 'x1' AND Active__c = true)",
		"(NOT Name = 'x1') AND (NOT Active__c = true)",
		"Active__c = true AND (NOT Name = 'x1')",
	}
	for _, where := range accept {
		t.Run("accept "+where, func(t *testing.T) {
			if _, err := Parse("SELECT Name FROM Child__c WHERE " + where); err != nil {
				t.Fatalf("Parse(%q): %v", where, err)
			}
		})
	}
	// Native QueryException text: unexpected token: '<token>'.
	reject := []struct{ where, token string }{
		{"NOT Name = 'x1' AND Active__c = true", "AND"},
		{"Active__c = true AND NOT Name = 'x1'", "NOT"},
		{"NOT Name = 'x1' OR Active__c = true", "OR"},
		{"Name = 'x1' OR NOT Name = 'y2' AND Active__c = true", "NOT"},
		{"NOT Name = 'x1' AND NOT Name = 'y2'", "AND"},
		{"NOT NOT Name = 'x1'", "NOT"},
		{"NOT Name LIKE 'x%' AND Ord__c > 1", "AND"},
		{"NOT Ord__c IN (1,2) AND Active__c = false", "AND"},
		{"NOT(Name = 'x1') AND Active__c = true", "AND"},
	}
	for _, tc := range reject {
		t.Run("reject "+tc.where, func(t *testing.T) {
			_, err := Parse("SELECT Name FROM Child__c WHERE " + tc.where)
			if err == nil {
				t.Fatalf("Parse(%q) accepted; native rejected", tc.where)
			}
			want := "unexpected token: '" + tc.token + "'"
			var queryErr *QueryError
			if !errors.As(err, &queryErr) || queryErr.Message != want {
				t.Fatalf("Parse(%q) error = %#v, want message %q", tc.where, err, want)
			}
		})
	}
}

func TestParseAcceptsPrefixNotLikeAndNotIn(t *testing.T) {
	// Native accepted `NOT field LIKE value` and `field NOT IN (...)`.
	if _, err := Parse("SELECT Name FROM Parent__c WHERE NOT Name LIKE '%Leader'"); err != nil {
		t.Fatalf("prefix NOT LIKE: %v", err)
	}
	if _, err := Parse("SELECT Name FROM Parent__c WHERE Name NOT IN ('Other')"); err != nil {
		t.Fatalf("NOT IN: %v", err)
	}
}
