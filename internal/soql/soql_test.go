package soql

import (
	"testing"

	"github.com/open-aer/oaer/internal/storage"
)

func TestParseSimpleQuery(t *testing.T) {
	query, err := Parse("SELECT Id, Name FROM Account WHERE Name = 'Acme' ORDER BY Name LIMIT 10 OFFSET 1")
	if err != nil {
		t.Fatal(err)
	}
	if query.Object != "Account" || len(query.Fields) != 2 || query.Where.Field != "Name" || query.Limit != 10 || query.Offset != 1 {
		t.Fatalf("query = %#v", query)
	}
}

func TestExecuteFiltersProjectsAndOrders(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Records: map[storage.ID]storage.Record{
			"001000000000002": {
				ID:     "001000000000002",
				Object: "Account",
				Fields: map[string]storage.Value{
					"Name":   storage.StringValue("Beta"),
					"Active": storage.BooleanValue(true),
				},
			},
			"001000000000001": {
				ID:     "001000000000001",
				Object: "Account",
				Fields: map[string]storage.Value{
					"Name":   storage.StringValue("Acme"),
					"Active": storage.BooleanValue(true),
				},
			},
			"001000000000003": {
				ID:     "001000000000003",
				Object: "Account",
				Fields: map[string]storage.Value{
					"Name":   storage.StringValue("Dormant"),
					"Active": storage.BooleanValue(false),
				},
			},
		},
	}

	result, err := ParseAndExecute(org, "SELECT Id, Name FROM Account WHERE Active = true ORDER BY Name LIMIT 1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 {
		t.Fatalf("rows = %d", result.Rows)
	}
	record := result.Records[0]
	if record.ID != "001000000000001" || record.Fields["Name"].String != "Acme" {
		t.Fatalf("record = %#v", record)
	}
	if _, ok := record.Fields["Active"]; ok {
		t.Fatalf("unprojected field leaked: %#v", record.Fields)
	}
}

func TestParseUnsupportedReturnsError(t *testing.T) {
	if _, err := Parse("SELECT Name FROM Account WHERE Name LIKE 'A%'"); err == nil {
		t.Fatal("expected unsupported operator error")
	}
}
