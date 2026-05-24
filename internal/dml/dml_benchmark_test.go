package dml

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func BenchmarkInsertBulkAccounts(b *testing.B) {
	records := makeBenchmarkAccounts(200)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		org := testOrg()
		engine := NewEngine(&org)
		results := engine.Insert(records)
		if len(results) != len(records) || !results[len(results)-1].Success {
			b.Fatalf("results = %#v", results)
		}
	}
}

func makeBenchmarkAccounts(count int) []storage.Record {
	records := make([]storage.Record, 0, count)
	for i := 0; i < count; i++ {
		records = append(records, storage.Record{
			Object: "Account",
			Fields: map[string]storage.Value{
				"Name": storage.StringValue(fmt.Sprintf("Account %03d", i)),
			},
		})
	}
	return records
}
