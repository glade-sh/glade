package vm

import (
	"encoding/csv"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The rows in ../apextest/testdata/conformance/date_parse_pivot_locales_c4
// (captured 2026-11-07 at API 62.0 and 67.0, admin in America/Los_Angeles,
// LocaleSidKey de_DE, en_GB, fr_FR, no and no_NO): the probe.apex of
// date_parse_pivot_c14 run once per locale. Every row re-runs its call as a
// user with the captured locale and compares the result text.
func TestDateParsePivotLocaleRowsC4(t *testing.T) {
	f, err := os.Open("../apextest/testdata/conformance/date_parse_pivot_locales_c4/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 2491 {
		t.Fatalf("rows=%d err=%v", len(rows), err)
	}
	seen := map[string]int{}
	for _, row := range rows[1:] {
		if len(row) != 5 {
			t.Fatalf("invalid row %q", row)
		}
		api, locale, method, input, want := row[0], row[1], row[2], row[3], row[4]
		seen[locale+"/"+method]++
		literal := "'" + strings.ReplaceAll(input, "'", "\\'") + "'"
		var call string
		switch method {
		case "Date.parse":
			call = "String.valueOf(Date.parse(" + literal + "))"
		case "Datetime.parse":
			call = "Datetime.parse(" + literal + ").formatGmt('yyyy-MM-dd HH:mm')"
		case "Date.format":
			call = "Date.newInstance(2008, 1, 28).format()"
		case "Datetime.format":
			call = "Datetime.newInstance(2008, 1, 28, 15, 4, 0).format()"
		default:
			t.Fatalf("unknown method %q", method)
		}
		source := "String r; try { r = " + call + "; } catch (Exception e) { r = 'ERR:' + e.getTypeName(); }\n" +
			"System.assertEquals('" + strings.ReplaceAll(want, "'", "\\'") + "', r);"
		t.Run(api+"/"+locale+"/"+method+"/"+input, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			machine.SetCurrentUser(storage.Record{ID: storage.ID("005-date-parse-pivot-c4"), Object: "User", Fields: map[string]storage.Value{
				"LocaleSidKey": storage.StringValue(locale), "LanguageLocaleKey": storage.StringValue("en_US"), "TimeZoneSidKey": storage.StringValue("America/Los_Angeles")}})
			if _, err := machine.Execute(program); err != nil {
				t.Fatalf("input %q: want %q: %v", input, want, err)
			}
		})
	}
	for _, locale := range []string{"de_DE", "en_GB", "fr_FR", "no", "no_NO"} {
		if seen[locale+"/Date.parse"] != 242 || seen[locale+"/Datetime.parse"] != 252 || seen[locale+"/Date.format"] != 2 || seen[locale+"/Datetime.format"] != 2 {
			t.Fatalf("capture shape changed: %v", seen)
		}
	}
}
