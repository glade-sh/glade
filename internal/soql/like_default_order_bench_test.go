package soql

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// likeBenchOrg builds n Account rows whose names are a fixed pseudo-random
// permutation, so storage (Id) order is not already name order.
func likeBenchOrg(n int) storage.OrgState {
	org := storage.NewOrgState()
	definition := storage.ObjectDefinition{APIName: "Account", Fields: map[string]storage.Field{
		"Id":          {APIName: "Id", Type: storage.FieldID},
		"Name":        {APIName: "Name", Type: storage.FieldString},
		"BillingCity": {APIName: "BillingCity", Type: storage.FieldString},
	}}
	state := storage.ObjectState{Definition: definition, Records: map[storage.ID]storage.Record{}}
	for i := 0; i < n; i++ {
		perm := (i * 7919) % n
		id := storage.ID(fmt.Sprintf("001%012d", i+1))
		state.Records[id] = storage.Record{ID: id, Object: "Account", Fields: map[string]storage.Value{
			"Name":        storage.StringValue(fmt.Sprintf("Account %06d", perm)),
			"BillingCity": storage.StringValue(fmt.Sprintf("City %06d", perm)),
		}}
	}
	org.Objects["Account"] = state
	return org
}

func BenchmarkLikeDefaultOrder(b *testing.B) {
	for _, n := range []int{10000, 1000} {
		org := likeBenchOrg(n)
		for _, shape := range []struct{ name, where string }{
			{"prefix", "Name LIKE 'Account%'"},
			{"contains", "Name LIKE '%ccount%'"},
			{"control", "BillingCity LIKE 'City%'"},
		} {
			for _, limit := range []string{"", " LIMIT 50"} {
				text := "SELECT Id, Name, BillingCity FROM Account WHERE " + shape.where + limit
				name := fmt.Sprintf("rows%d/%s/limit%t", n, shape.name, limit != "")
				b.Run(name, func(b *testing.B) {
					query, err := Parse(text)
					if err != nil {
						b.Fatal(err)
					}
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						if _, err := ExecuteWithCacheAndContext(org, query, nil, ExecutionContext{}); err != nil {
							b.Fatal(err)
						}
					}
				})
			}
		}
	}
}
