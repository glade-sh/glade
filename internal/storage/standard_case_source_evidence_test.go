package storage

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"testing"
)

// Availability comes from the isolated capture; P11 only supplies field facets.
func requireCaseSourceNativeMembership(t *testing.T, api string) {
	t.Helper()
	file, err := os.Open("../apextest/testdata/conformance/standard_case_source_evidence/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	routes := map[string]bool{}
	for _, row := range rows[1:] {
		if row[0] != api+".0" || row[2] != "membership" {
			continue
		}
		if row[3] != "true" || routes[row[1]] {
			t.Fatalf("invalid native Case source route: %s/%s", api, row[1])
		}
		var fields map[string]bool
		if err := json.Unmarshal([]byte(row[4]), &fields); err != nil {
			t.Fatal(err)
		}
		want := map[string]bool{"SourceId": true, "Subject": true, "HasCommentsUnreadByOwner": false, "IsVisibleInSelfService": false, "OwnedMissing__c": false}
		if len(fields) != len(want) {
			t.Fatal("native Case source membership changed")
		}
		for field, expected := range want {
			if got, exists := fields[field]; !exists || got != expected {
				t.Fatalf("native Case source membership differs: %s/%s", api, field)
			}
		}
		routes[row[1]] = true
	}
	if len(routes) != 3 || !routes["anonymous"] || !routes["named"] || !routes["test-runner"] {
		t.Fatalf("incomplete native Case source routes for API %s: %v", api, routes)
	}
}

// standard_case_source_evidence/org.tsv:2-4,14-16 captures SourceId present
// with LiveAgent enabled, while the neighboring two Case fields remain absent.
// P0 named maps at API62/67 capture the baseline absence. No other messaging
// combination is attributed to this gate.
func TestStandardFieldEvidenceLiveAgentStorageRoutes(t *testing.T) {
	definition := ObjectDefinition{APIName: "Case"}
	for _, row := range []struct {
		name     string
		features []string
		present  bool
	}{
		{"baseline", nil, false},
		{"live-agent", []string{"LiveAgent", "LiveAgentEnabled"}, true},
		{"return-to-baseline", nil, false},
	} {
		t.Run(row.name, func(t *testing.T) {
			EnsureStandardObjectFieldsForFeatures(&definition, row.features)
			for field, want := range map[string]bool{"SourceId": row.present, "Subject": true, "HasCommentsUnreadByOwner": false, "IsVisibleInSelfService": false} {
				if _, got := ResolveFieldName(definition, "", field); got != want {
					t.Errorf("%s membership=%t, want %t", field, got, want)
				}
			}
			if StandardObjectFieldsNeedWrite(definition) {
				t.Fatal("steady-state overlay requests repair")
			}
		})
	}
}
