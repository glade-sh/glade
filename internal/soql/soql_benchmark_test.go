package soql

import (
	"fmt"
	"testing"

	"github.com/open-aer/oaer/internal/storage"
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

func benchmarkSOQLOrg(records int) storage.OrgState {
	org := storage.NewOrgState()
	object := storage.ObjectState{
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
