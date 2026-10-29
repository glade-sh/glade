package apextest

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// customAddressFollowupGaps lists followup.tsv rows Glade does not match for
// reasons unrelated to Address fields or restricted picklists.
var customAddressFollowupGaps = map[string]string{
	"R01": "the generated standard Account catalog carries no restrictedPicklist facet for BillingGeocodeAccuracy (generator lives in glade-tools)",
}

// customAddressFollowupSObjectJSON lists rows that serialize a whole SObject.
// Glade's SObject key order and attributes.url differ from native for ordinary
// fields too (generic gap), so only the nested Address object is compared.
var customAddressFollowupSObjectJSON = map[string]string{
	"J07": "BillingAddress", "J07b": "BillingAddress", "J07c": "BillingAddress", "J10": "Home__c", "J11": "Home__c",
}

// customAddressNestedObject returns the JSON object text that follows key.
func customAddressNestedObject(text, key string) string {
	_, rest, ok := strings.Cut(text, `"`+key+`":`)
	if !ok || !strings.HasPrefix(rest, "{") {
		return ""
	}
	depth := 0
	for i, r := range rest {
		switch r {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return rest[:i+1]
			}
		}
	}
	return ""
}

var customAddressRecordID = regexp.MustCompile(`with id [A-Za-z0-9]{15,18}`)

func customAddressFollowupNativeRows(t *testing.T, api string) []customAddressRow {
	t.Helper()
	f, err := os.Open(filepath.Join(customAddressFixture, "followup.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	table, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	var rows []customAddressRow
	for _, row := range table[1:] {
		if row[0] == api {
			rows = append(rows, customAddressRow{row[1], row[2], customAddressRecordID.ReplaceAllString(row[3], "with id <ID>")})
		}
	}
	if len(rows) != 70 {
		t.Fatalf("followup API %s rows = %d; want 70", api, len(rows))
	}
	return rows
}

// TestCustomAddressFollowupNativeConformance replays the second native capture
// (fresh Developer scratch org, API 62.0 and 67.0): JSON of standard and
// custom Address values, restricted-picklist messages for standard, custom and
// dependent fields, and latitude/longitude/state/country validation on insert
// and on update of an existing record.
func TestCustomAddressFollowupNativeConformance(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			index := customAddressProject(t, api)
			out, err := customAddressRun(t, index, api, "for (String r : AddressFollowup.run()) System.debug(LoggingLevel.ERROR, 'ADDR|' + r);")
			if err != nil {
				t.Fatalf("followup: %v", err)
			}
			got := map[string]string{}
			for _, row := range out {
				id, observation, _ := strings.Cut(row, "|")
				got[id] = customAddressRecordID.ReplaceAllString(observation, "with id <ID>")
			}
			for _, row := range customAddressFollowupNativeRows(t, api) {
				if _, gap := customAddressFollowupGaps[row.id]; gap {
					continue
				}
				want, have := row.observation, got[row.id]
				if key, nested := customAddressFollowupSObjectJSON[row.id]; nested {
					want, have = customAddressNestedObject(want, key), customAddressNestedObject(have, key)
					if want == "" {
						t.Fatalf("%s: no %s object in native row", row.id, key)
					}
				}
				if have != want {
					t.Errorf("%s: native <%s> glade <%s>", row.id, want, have)
				}
			}
		})
	}
}
