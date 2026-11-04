package soql

import (
	"reflect"
	"testing"
)

// Projection plans must preserve the full result across repeated executions.
func TestProjectionMixedFieldsRepeatedExecution(t *testing.T) {
	org := aggregateTestOrg()
	query, err := Parse("SELECT Name, toLabel(Rating) ratingLabel, FORMAT(AnnualRevenue) formattedRevenue FROM Account ORDER BY Name")
	if err != nil {
		t.Fatal(err)
	}
	cache := NewExecutionCache()
	first, err := ExecuteWithCache(org, query, cache)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Records) < 2 {
		t.Fatalf("expected multiple records: %#v", first)
	}
	for _, row := range first.Records {
		if _, ok := row.Fields["Name"]; !ok {
			t.Fatalf("missing Name: %#v", row)
		}
		if _, ok := row.Fields["ratingLabel"]; !ok {
			t.Fatalf("missing alias: %#v", row)
		}
	}
	second, err := ExecuteWithCache(org, query, cache)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("warm result differs: %#v vs %#v", first, second)
	}
}
