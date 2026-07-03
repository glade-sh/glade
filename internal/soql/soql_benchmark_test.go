package soql

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func BenchmarkParseAndExecute(b *testing.B) {
	org := benchmarkSOQLOrg(1000)
	query := "SELECT Id, Name FROM Account WHERE Active = true ORDER BY Name LIMIT 50"
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		result, err := ParseAndExecute(org, query)
		if err != nil {
			b.Fatal(err)
		}
		if result.Rows != 50 {
			b.Fatalf("rows = %d", result.Rows)
		}
	}
}

func BenchmarkParseAndExecuteIndexedWhereShapes(b *testing.B) {
	org := benchmarkSOQLOrg(10000)
	storage.RebuildIndexes(&org)
	queries := map[string]string{
		"equals": "SELECT Id, Name FROM Account WHERE Active = true ORDER BY Name LIMIT 50",
		"in":     "SELECT Id, Name FROM Account WHERE Active IN (true) ORDER BY Name LIMIT 50",
		"or":     "SELECT Id, Name FROM Account WHERE Active = true OR Active = false ORDER BY Name LIMIT 50",
	}
	for name, query := range queries {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				result, err := ParseAndExecute(org, query)
				if err != nil {
					b.Fatal(err)
				}
				if result.Rows != 50 {
					b.Fatalf("rows = %d", result.Rows)
				}
			}
		})
	}
}

func benchmarkSOQLOrg(records int) storage.OrgState {
	org := storage.NewOrgState()
	object := storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Account",
			Indexes: []storage.IndexDefinition{{
				Name:   "Account.Active",
				Object: "Account",
				Fields: []string{"Active"},
			}},
		},
		Records: make(map[storage.ID]storage.Record, records),
	}
	for i := 0; i < records; i++ {
		id := storage.ID(fmt.Sprintf("001%012d", i+1))
		object.Records[id] = storage.Record{
			ID:     id,
			Object: "Account",
			Fields: map[string]storage.Value{
				"Name":   storage.StringValue(fmt.Sprintf("Account %04d", records-i)),
				"Active": storage.BooleanValue(i%2 == 0),
			},
		}
	}
	org.Objects["Account"] = object
	return org
}
