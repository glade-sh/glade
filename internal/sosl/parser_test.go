package sosl_test

import (
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/sosl"
)

func TestParseSOSLReturningObjectsAndFields(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  sosl.Query
	}{
		{
			name:  "braced find",
			input: "FIND {Acme} RETURNING Account(Id, Name)",
			want: sosl.Query{Returning: []sosl.ReturningObject{{
				Object: "Account",
				Fields: []string{"Id", "Name"},
			}}},
		},
		{
			name:  "quoted find in all fields",
			input: "FIND 'Acme' IN ALL FIELDS RETURNING Account(Id, Name), Contact(Id)",
			want: sosl.Query{Returning: []sosl.ReturningObject{
				{Object: "Account", Fields: []string{"Id", "Name"}},
				{Object: "Contact", Fields: []string{"Id"}},
			}},
		},
		{
			name:  "bind find with where clause",
			input: "FIND :term RETURNING Invoice__c(Id, Amount__c WHERE Amount__c = :value)",
			want: sosl.Query{Returning: []sosl.ReturningObject{{
				Object:     "Invoice__c",
				Fields:     []string{"Id", "Amount__c"},
				WhereBinds: []sosl.WhereBind{{Field: "Amount__c", Name: "value"}},
			}}},
		},
		{
			name:  "like bind with where clause",
			input: "FIND :term RETURNING Account(Id, Name WHERE Name LIKE :value)",
			want: sosl.Query{Returning: []sosl.ReturningObject{{
				Object:     "Account",
				Fields:     []string{"Id", "Name"},
				WhereBinds: []sosl.WhereBind{{Field: "Name", Name: "value"}},
			}}},
		},
		{
			name:  "where bind stays with its returning object",
			input: "FIND :term RETURNING Account(Id), Contact(Id, LastName WHERE LastName = :value)",
			want: sosl.Query{Returning: []sosl.ReturningObject{
				{Object: "Account", Fields: []string{"Id"}},
				{
					Object:     "Contact",
					Fields:     []string{"Id", "LastName"},
					WhereBinds: []sosl.WhereBind{{Field: "LastName", Name: "value"}},
				},
			}},
		},
		{
			name:  "relationship field path",
			input: "FIND :term RETURNING Contact(LastName, Account.Name)",
			want: sosl.Query{Returning: []sosl.ReturningObject{{
				Object: "Contact",
				Fields: []string{"LastName", "Account.Name"},
			}}},
		},
		{
			name:  "bound limit",
			input: "FIND 'Acme' RETURNING Account(Id) LIMIT :limitValue",
			want:  sosl.Query{Returning: []sosl.ReturningObject{{Object: "Account", Fields: []string{"Id"}}}, LimitBind: "limitValue"},
		},
		{
			name:  "all documented SOSL bind clauses",
			input: "FIND :term IN ALL FIELDS RETURNING Account(Id, Name WHERE Name LIKE :name LIMIT :perObjectLimit OFFSET :offsetValue) WITH DIVISION = :division LIMIT :limitValue",
			want: sosl.Query{
				Returning: []sosl.ReturningObject{
					{Object: "Account", Fields: []string{"Id", "Name"}, WhereBinds: []sosl.WhereBind{{Field: "Name", Name: "name"}}, LimitBind: "perObjectLimit", OffsetBind: "offsetValue"},
				},
				DivisionBind: "division",
				LimitBind:    "limitValue",
			},
		},
		{
			name:  "literal limits",
			input: "FIND 'Acme' RETURNING Account(Id LIMIT 5 OFFSET 1) LIMIT 10",
			want:  sosl.Query{Returning: []sosl.ReturningObject{{Object: "Account", Fields: []string{"Id"}}}},
		},
		{
			name:  "literal division",
			input: "FIND 'Acme' RETURNING Account(Id) WITH DIVISION = 'Global'",
			want:  sosl.Query{Returning: []sosl.ReturningObject{{Object: "Account", Fields: []string{"Id"}}}},
		},
		{
			name:  "returning order by is not a field",
			input: "FIND 'Acme' RETURNING Account(Id, Name ORDER BY Name LIMIT 100 OFFSET 10)",
			want:  sosl.Query{Returning: []sosl.ReturningObject{{Object: "Account", Fields: []string{"Id", "Name"}}}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sosl.Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse returned error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Parse() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
