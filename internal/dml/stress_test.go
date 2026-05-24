package dml

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestStressBulkDMLPartialResults(t *testing.T) {
	org := testOrg()
	engine := NewEngine(&org)
	records := make([]storage.Record, 0, 240)
	for i := 0; i < 240; i++ {
		fields := map[string]storage.Value{"Name": storage.StringValue(fmt.Sprintf("Account %03d", i))}
		if i%25 == 0 {
			fields = map[string]storage.Value{}
		}
		records = append(records, storage.Record{Object: "Account", Fields: fields})
	}
	results := engine.Insert(records)
	if len(results) != len(records) {
		t.Fatalf("results = %d", len(results))
	}
	failures := 0
	for i, result := range results {
		if i%25 == 0 {
			if result.Success || len(result.Errors) == 0 {
				t.Fatalf("record %d should have failed: %#v", i, result)
			}
			failures++
			continue
		}
		if !result.Success || result.ID == "" {
			t.Fatalf("record %d should have succeeded: %#v", i, result)
		}
	}
	if failures != 10 {
		t.Fatalf("failures = %d", failures)
	}
	if got := len(org.Objects["Account"].Records); got != len(records)-failures {
		t.Fatalf("persisted records = %d", got)
	}
}
