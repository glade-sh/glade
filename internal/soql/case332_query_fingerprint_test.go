package soql

import (
	"encoding/json"
	"testing"
)

// The logged records and errors are compared across the two committed trees.
func TestCase332QueryFingerprints(t *testing.T) {
	org := aggregateTestOrg()
	cases := []struct {
		name, query string
		wantError   bool
	}{
		{"mixed", "SELECT Name, toLabel(Rating) ratingLabel, FORMAT(AnnualRevenue) formattedRevenue FROM Account ORDER BY Name", false},
		{"group", "SELECT Rating, COUNT(Name) total FROM Account GROUP BY Rating ORDER BY Rating", false},
		{"like", "SELECT Name FROM Account WHERE Name LIKE 'A%' ORDER BY Name", false},
		{"error", "SELECT Missing__c FROM Account", true},
	}
	for _, tc := range cases {
		result, err := ParseAndExecute(org, tc.query)
		if (err != nil) != tc.wantError {
			t.Fatalf("%s error = %v", tc.name, err)
		}
		data, err := json.Marshal(result)
		if err != nil {
			t.Fatal(err)
		}
		text := ""
		if tc.wantError {
			_, e := ParseAndExecute(org, tc.query)
			text = e.Error()
		}
		t.Logf("SOQL_FINGERPRINT %s result=%s error=%q", tc.name, data, text)
	}
}

func TestCase332SparseProjectionWork(t *testing.T) {
	org := aggregateTestOrg()
	query, err := Parse("SELECT Name, toLabel(Rating) ratingLabel, FORMAT(AnnualRevenue) formattedRevenue FROM Account WHERE Name = 'not-present'")
	if err != nil {
		t.Fatal(err)
	}
	cache := NewExecutionCache()
	for i := 0; i < 10000; i++ {
		result, err := ExecuteWithCache(org, query, cache)
		if err != nil {
			t.Fatal(err)
		}
		if result.Rows != 0 || len(result.Records) != 0 {
			t.Fatalf("unexpected result %#v", result)
		}
	}
}
