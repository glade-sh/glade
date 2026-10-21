package soql

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The query table matches the owned named Apex probe at source APIs 62, 63,
// and 67. Sort-only aggregates must not change the projected result fields.
func TestUnselectedAggregateOrderConformance(t *testing.T) {
	data, err := os.ReadFile("testdata/unselected_aggregate_order.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Query string   `json:"query"`
		Names []string `json:"names"`
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) != 10 {
		t.Fatalf("query table has %d cases, want 10", len(cases))
	}
	for _, tc := range cases {
		t.Run(tc.Query, func(t *testing.T) {
			result, err := ParseAndExecute(unselectedAggregateOrderOrg(), tc.Query)
			if err != nil {
				t.Fatal(err)
			}
			var names []string
			for _, row := range result.Records {
				names = append(names, row.Fields["Name"].String)
				for field := range row.Fields {
					if field != "Name" && field != "total" && field != "lowest" && field != "expr0" {
						t.Fatalf("sort-only field leaked: %q", field)
					}
					if field != "Name" && !strings.Contains(tc.Query, " total") && !strings.Contains(tc.Query, " lowest") {
						t.Fatalf("unselected aggregate projected: %q", field)
					}
				}
			}
			if !reflect.DeepEqual(names, tc.Names) {
				t.Fatalf("names = %v, want %v", names, tc.Names)
			}
		})
	}
}

func TestUnselectedAggregateOrderPreservesParsedQuery(t *testing.T) {
	text := "SELECT Name FROM Account GROUP BY Name ORDER BY MIN(CreatedDate) DESC LIMIT 2"
	query, err := Parse(text)
	if err != nil {
		t.Fatal(err)
	}
	before := cloneQuery(query)
	for range 2 {
		if _, err := Execute(unselectedAggregateOrderOrg(), query); err != nil {
			t.Fatal(err)
		}
		cached, err := Parse(text)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(query, before) || !reflect.DeepEqual(cached, before) {
			t.Fatal("execution mutated the caller or cached query")
		}
	}
}

func TestUnselectedAggregateOrderControls(t *testing.T) {
	org := unselectedAggregateOrderOrg()
	for _, query := range []string{
		"SELECT Name FROM Account GROUP BY Name ORDER BY Missing__c",
		"SELECT Name FROM Account GROUP BY Name ORDER BY AnnualRevenue",
		"SELECT Name FROM Account GROUP BY Name ORDER BY MIN(Missing__c)",
		"SELECT Name FROM Account GROUP BY Name ORDER BY SUM(Name)",
		"SELECT Name FROM Account ORDER BY MIN(AnnualRevenue)",
	} {
		t.Run(query, func(t *testing.T) {
			if _, err := ParseAndExecute(org, query); err == nil {
				t.Fatal("invalid reference was accepted")
			}
		})
	}
	for _, query := range []string{
		"SELECT Name FROM Account GROUP BY Name ORDER BY Name DESC",
		"SELECT Name, MIN(AnnualRevenue) lowest FROM Account GROUP BY Name ORDER BY lowest DESC NULLS LAST",
	} {
		t.Run(query, func(t *testing.T) {
			if _, err := ParseAndExecute(org, query); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func unselectedAggregateOrderOrg() storage.OrgState {
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	account := org.Objects["Account"]
	for i, seed := range []struct{ name, revenue, day string }{
		{"Alpha", "10", "01"}, {"Alpha", "40", "04"}, {"Beta", "20", "02"}, {"Gamma", "", "03"},
	} {
		id := storage.ID(fmt.Sprintf("001%012d", i+1))
		revenue := storage.NullValue()
		if seed.revenue != "" {
			revenue = storage.DecimalValue(seed.revenue)
		}
		account.Records[id] = storage.Record{
			ID: id, Object: "Account",
			Fields: map[string]storage.Value{"Name": storage.StringValue(seed.name), "AnnualRevenue": revenue},
			System: storage.SystemFields{CreatedDate: "2024-01-" + seed.day + "T00:00:00.000Z"},
		}
	}
	org.Objects["Account"] = account
	return org
}
