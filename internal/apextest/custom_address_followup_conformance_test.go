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
var customAddressFollowupGaps = map[string]string{}

// customAddressFollowupURLVersion finds the org release in the captured
// record URLs (J07..J11). Native writes the org's release, v68.0, at both
// source APIs, so the replay sets the org REST release to that version.
var customAddressFollowupURLVersion = regexp.MustCompile(`/services/data/v([0-9]+\.0)/sobjects/`)

func customAddressFollowupRuntimeAPI(t *testing.T, rows []customAddressRow) string {
	t.Helper()
	version := ""
	for _, row := range rows {
		for _, match := range customAddressFollowupURLVersion.FindAllStringSubmatch(row.observation, -1) {
			if version != "" && version != match[1] {
				t.Fatalf("followup rows mix URL versions %s and %s", version, match[1])
			}
			version = match[1]
		}
	}
	if version == "" {
		t.Fatal("followup rows carry no record URL")
	}
	return version
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
			native := customAddressFollowupNativeRows(t, api)
			out, err := customAddressRunAt(t, index, api, customAddressFollowupRuntimeAPI(t, native), "for (String r : AddressFollowup.run()) System.debug(LoggingLevel.ERROR, 'ADDR|' + r);")
			if err != nil {
				t.Fatalf("followup: %v", err)
			}
			got := map[string]string{}
			for _, row := range out {
				id, observation, _ := strings.Cut(row, "|")
				got[id] = customAddressRecordID.ReplaceAllString(observation, "with id <ID>")
			}
			for _, row := range native {
				if _, gap := customAddressFollowupGaps[row.id]; gap {
					continue
				}
				want, have := row.observation, got[row.id]
				if have != want {
					t.Errorf("%s: native <%s> glade <%s>", row.id, want, have)
				}
			}
		})
	}
}
