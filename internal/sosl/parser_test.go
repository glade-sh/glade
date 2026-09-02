package sosl_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sosl"
)

func TestParseRuntimeSubsetCarriesSearchAndReturningClauses(t *testing.T) {
	query, err := sosl.Parse("FIND {Acme*} IN NAME FIELDS RETURNING Account(Id, FORMAT(AnnualRevenue) formatted WHERE Name LIKE :name ORDER BY Name DESC LIMIT :perObject OFFSET 2), Contact(Id) WITH SNIPPET LIMIT :globalLimit")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(query.Terms, []sosl.SearchTerm{{Text: "Acme", Prefix: true}}) {
		t.Fatalf("terms = %#v", query.Terms)
	}
	if query.Scope != sosl.SearchScopeName || !query.WithSnippet {
		t.Fatalf("query options = %#v", query)
	}
	if query.Limit != (sosl.Window{Bind: "globalLimit"}) {
		t.Fatalf("global limit = %#v", query.Limit)
	}
	if len(query.Returning) != 2 {
		t.Fatalf("returning = %#v", query.Returning)
	}
	account := query.Returning[0]
	if account.Object != "Account" || !reflect.DeepEqual(account.Fields, []sosl.SelectExpr{{Field: "Id"}, {Field: "AnnualRevenue", Func: "FORMAT", Alias: "formatted"}}) {
		t.Fatalf("account projection = %#v", account)
	}
	if account.Where == nil || !reflect.DeepEqual(*account.Where, sosl.Condition{Field: "Name", Operator: "LIKE", Bind: "name"}) {
		t.Fatalf("account condition = %#v", account.Where)
	}
	if !reflect.DeepEqual(account.OrderBy, []sosl.OrderSpec{{Field: "Name", Desc: true}}) || account.Limit != (sosl.Window{Bind: "perObject"}) || account.Offset != (sosl.Window{Value: 2, HasValue: true}) {
		t.Fatalf("account windows/order = %#v", account)
	}
}

func TestParseSOSLReturningWhereIn(t *testing.T) {
	query, err := sosl.Parse("FIND {Acme} RETURNING Account(Id WHERE Id IN ('001A', '001B'))")
	if err != nil {
		t.Fatal(err)
	}
	want := sosl.Condition{Field: "Id", Operator: "IN", Values: []string{"001A", "001B"}}
	if query.Returning[0].Where == nil || !reflect.DeepEqual(*query.Returning[0].Where, want) {
		t.Fatalf("where = %#v, want %#v", query.Returning[0].Where, want)
	}
}

func TestParseSOSLReturningWhereNotInBind(t *testing.T) {
	query, err := sosl.Parse("FIND {Acme} RETURNING Account(Id WHERE Id NOT IN :excludedIds)")
	if err != nil {
		t.Fatal(err)
	}
	want := sosl.Condition{Field: "Id", Operator: "NOT IN", Bind: "excludedIds"}
	if query.Returning[0].Where == nil || !reflect.DeepEqual(*query.Returning[0].Where, want) {
		t.Fatalf("where = %#v, want %#v", query.Returning[0].Where, want)
	}
}

func TestParseSOSLReturningWhereRejectsOtherNotForms(t *testing.T) {
	_, err := sosl.Parse("FIND {Acme} RETURNING Account(Id WHERE Id NOT LIKE '001%')")
	if err == nil || !strings.Contains(err.Error(), `unsupported SOSL WHERE operator "NOT"`) {
		t.Fatalf("error = %v, want unsupported NOT operator", err)
	}
}

func TestParseRejectsUnimplementedSOSLClause(t *testing.T) {
	_, err := sosl.Parse("FIND {Acme} IN ALL FIELDS WITH DATA CATEGORY Products ABOVE Hardware RETURNING Account(Id)")
	var unsupported *sosl.UnsupportedFeatureError
	if !errors.As(err, &unsupported) {
		t.Fatalf("error = %v, want UnsupportedFeatureError", err)
	}
}

func TestParseSOSLReturningObjectAndLiteralWindows(t *testing.T) {
	query, err := sosl.Parse("FIND 'Acme' RETURNING Account(Id, Name ORDER BY Name LIMIT 5 OFFSET 1) LIMIT 10")
	if err != nil {
		t.Fatal(err)
	}
	if len(query.Terms) != 1 || query.Terms[0].Text != "Acme" {
		t.Fatalf("terms = %#v", query.Terms)
	}
	if len(query.Returning) != 1 || query.Returning[0].Limit != (sosl.Window{Value: 5, HasValue: true}) || query.Returning[0].Offset != (sosl.Window{Value: 1, HasValue: true}) || query.Limit != (sosl.Window{Value: 10, HasValue: true}) {
		t.Fatalf("windows = %#v", query)
	}
}

func TestParseSOSLBindSearchAndDivision(t *testing.T) {
	query, err := sosl.Parse("FIND :term IN ALL FIELDS RETURNING Account(Id WHERE Name = :name) WITH DIVISION = :division")
	if err != nil {
		t.Fatal(err)
	}
	if len(query.Terms) != 1 || query.Terms[0].Text != ":term" || query.DivisionBind != "division" {
		t.Fatalf("query = %#v", query)
	}
	if query.Returning[0].Where == nil || query.Returning[0].Where.Bind != "name" {
		t.Fatalf("where = %#v", query.Returning[0].Where)
	}
}

func TestParseSOSLTracksExplicitEmptyDivision(t *testing.T) {
	query, err := sosl.Parse("FIND {Acme} RETURNING Account(Id) WITH DIVISION = ''")
	if err != nil {
		t.Fatal(err)
	}
	if !query.DivisionSpecified || query.DivisionBind != "" {
		t.Fatalf("division = %#v", query)
	}
}

func TestParseRejectsUnimplementedSearchOperators(t *testing.T) {
	for _, input := range []string{
		"FIND {Acme AND West} RETURNING Account(Id)",
		"FIND {Acme NOT West} RETURNING Account(Id)",
		"FIND {Acme?} RETURNING Account(Id)",
	} {
		t.Run(input, func(t *testing.T) {
			_, err := sosl.Parse(input)
			var unsupported *sosl.UnsupportedFeatureError
			if !errors.As(err, &unsupported) {
				t.Fatalf("error = %v, want UnsupportedFeatureError", err)
			}
		})
	}
}
