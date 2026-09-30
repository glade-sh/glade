package soql

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestHavingFamilyContract(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		wantRating []string
		wantNull   bool
		wantErr    string
	}{
		{
			name:       "aggregate comparisons with AND",
			query:      "SELECT Rating, COUNT(Id) accountCount, SUM(AnnualRevenue) totalRevenue FROM Account GROUP BY Rating HAVING COUNT(Id) >= 2 AND SUM(AnnualRevenue) > 300",
			wantRating: []string{"Hot"},
		},
		{
			name:       "aggregate comparisons with OR",
			query:      "SELECT Rating, COUNT(Id) accountCount, SUM(AnnualRevenue) totalRevenue FROM Account GROUP BY Rating HAVING COUNT(Id) = 2 OR SUM(AnnualRevenue) = 250",
			wantRating: []string{"Hot", "Warm"},
		},
		{
			name:       "grouped field literal IN remains valid",
			query:      "SELECT Rating, COUNT(Id) accountCount FROM Account GROUP BY Rating HAVING Rating IN ('Hot', 'Warm') AND COUNT(Id) > 0",
			wantRating: []string{"Hot", "Warm"},
		},
		{
			name:     "grouped null control",
			query:    "SELECT Rating, COUNT(Id) accountCount FROM Account GROUP BY Rating HAVING Rating = NULL AND COUNT(Id) > 0",
			wantNull: true,
		},
		{
			name:  "filter may produce no groups",
			query: "SELECT Rating, COUNT(Id) accountCount FROM Account GROUP BY Rating HAVING COUNT(Id) > 10",
		},
		{
			name:    "ungrouped nonaggregate field is rejected",
			query:   "SELECT Rating, COUNT(Id) accountCount FROM Account GROUP BY Rating HAVING Name = 'Acme' AND COUNT(Id) > 0",
			wantErr: "must be grouped or aggregated",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseAndExecute(havingContractOrg(), tt.query)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}

			gotRating := make(map[string]bool, len(result.Records))
			gotNull := false
			for _, record := range result.Records {
				rating := record.Fields["Rating"]
				if rating.Kind == storage.ValueNull {
					gotNull = true
					continue
				}
				gotRating[rating.String] = true
			}
			if len(gotRating) != len(tt.wantRating) || gotNull != tt.wantNull {
				t.Fatalf("HAVING result = %#v (ratings %v, null %v), want ratings %v and null %v", result, gotRating, gotNull, tt.wantRating, tt.wantNull)
			}
			for _, rating := range tt.wantRating {
				if !gotRating[rating] {
					t.Errorf("HAVING result omitted rating %q: %#v", rating, result)
				}
			}
		})
	}

	for _, tt := range []struct {
		name string
		op   string
	}{
		{name: "semi-join", op: "IN"},
		{name: "anti-join", op: "NOT IN"},
	} {
		t.Run(tt.name+" remains valid in WHERE", func(t *testing.T) {
			result, err := ParseAndExecute(havingContractOrg(), "SELECT Id FROM Account WHERE Id "+tt.op+" (SELECT AccountId FROM Contact)")
			if err != nil {
				t.Fatal(err)
			}
			want := 1
			if tt.op == "NOT IN" {
				want = 3
			}
			if len(result.Records) != want {
				t.Fatalf("WHERE join returned %d records, want %d", len(result.Records), want)
			}
		})
		t.Run(tt.name+" rejected by parser", func(t *testing.T) {
			queryText := "SELECT Id, COUNT(Id) accountCount FROM Account GROUP BY Id HAVING COUNT(Id) > 0 AND Id " + tt.op + " (SELECT AccountId FROM Contact)"
			if _, err := Parse(queryText); !havingSubqueryError(err, tt.name) {
				t.Fatalf("parser error = %v, want HAVING %s subquery rejection", err, tt.name)
			}
		})
		t.Run(tt.name+" rejected by constructed query", func(t *testing.T) {
			query, err := Parse("SELECT Id, COUNT(Id) accountCount FROM Account GROUP BY Id HAVING COUNT(Id) > 0")
			if err != nil {
				t.Fatal(err)
			}
			aggregatePredicate := *query.Having
			query.Having = &Condition{And: []Condition{
				aggregatePredicate,
				{
					Field: "Id",
					Op:    tt.op,
					Subquery: &Query{
						Object: "Contact",
						Fields: []string{"AccountId"},
					},
				},
			}}
			if _, err := Execute(havingContractOrg(), query); !havingSubqueryError(err, tt.name) {
				t.Fatalf("constructed-query error = %v, want HAVING %s subquery rejection", err, tt.name)
			}
			query.Having = &Condition{Or: []Condition{*query.Having, aggregatePredicate}}
			if _, err := Execute(havingContractOrg(), query); !havingSubqueryError(err, tt.name) {
				t.Fatalf("nested OR error = %v, want HAVING %s subquery rejection", err, tt.name)
			}
		})
	}
}

func havingSubqueryError(err error, joinKind string) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "having") && strings.Contains(message, joinKind)
}

func havingContractOrg() storage.OrgState {
	org := aggregateTestOrg()
	account := org.Objects["Account"]
	account.Records["001000000000004"] = storage.Record{
		ID:     "001000000000004",
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":          storage.StringValue("No Rating"),
			"AnnualRevenue": storage.DecimalValue("50"),
		},
	}
	org.Objects["Account"] = account
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Contact", Fields: map[string]storage.Field{
			"AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
		}},
		Records: map[storage.ID]storage.Record{
			"003000000000001": {ID: "003000000000001", Object: "Contact", Fields: map[string]storage.Value{
				"AccountId": storage.IDValue("001000000000001"),
			}},
		},
	}
	return org
}
