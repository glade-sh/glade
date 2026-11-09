package vm

import (
	"encoding/csv"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The rows in ../apextest/testdata/conformance/date_parse_pivot_c14 (captured
// 2026-11-07 at API 62.0 and 67.0 under en_US and en_CA admins in
// America/Los_Angeles): Date.parse and Datetime.parse two-digit years pivot at
// 60 (01..59 are 20yy, 60..99 are 19yy), one- and three-digit years pivot the
// same way, 100..999 and five-digit years throw, "0" throws and "00" is 2000
// only in the en_US slash grammar; en_CA Datetime.parse reads
// "yyyy-M-d, h:mm p.m." (PM and pm too) and rejects the 24-hour, slash and
// comma-less forms. Each parse row re-runs the call as the captured user.
func TestDateParsePivotOrgRows(t *testing.T) {
	f, err := os.Open("../apextest/testdata/conformance/date_parse_pivot_c14/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 997 {
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
			machine.SetCurrentUser(storage.Record{ID: storage.ID("005-date-parse-pivot-row"), Object: "User", Fields: map[string]storage.Value{
				"LocaleSidKey": storage.StringValue(locale), "LanguageLocaleKey": storage.StringValue("en_US"), "TimeZoneSidKey": storage.StringValue("America/Los_Angeles")}})
			if _, err := machine.Execute(program); err != nil {
				t.Fatalf("input %q: want %q: %v", input, want, err)
			}
		})
	}
	for key, want := range map[string]int{"en_US/Date.parse": 242, "en_CA/Date.parse": 242, "en_US/Datetime.parse": 252, "en_CA/Datetime.parse": 252, "en_US/Date.format": 2, "en_CA/Datetime.format": 2} {
		if seen[key] != want {
			t.Fatalf("capture shape changed: %v", seen)
		}
	}
}

// Locales outside date_parse_pivot_c14 and date_parse_pivot_locales_c4 keep
// the earlier month-first grammar (Go's 69 pivot, two- or four-digit years)
// until rows exist for them. The c4 day-first locales reject the month-first
// 12-hour form.
func TestDateParsePivotUncapturedLocalesKeepLegacyGrammar(t *testing.T) {
	for _, locale := range []string{"de_DE", "no", "no_NO", "en_GB", "fr_FR"} {
		if value, err := parseDatetimeParseTextForLocale("1/2/65, 3:04 PM", "America/Los_Angeles", locale); err == nil {
			t.Fatalf("%s Datetime.parse month-first 12-hour form parsed: %v", locale, value)
		}
	}
	for _, locale := range []string{"ja_JP", "it_IT", "es_ES"} {
		value, err := parseDatetimeParseTextForLocale("1/2/65, 3:04 PM", "America/Los_Angeles", locale)
		if err != nil || value.Year() != 2065 {
			t.Fatalf("%s Datetime.parse two-digit 65: %v %v", locale, value, err)
		}
		if _, err := parseDatetimeParseTextForLocale("1/2/5, 3:04 PM", "America/Los_Angeles", locale); err == nil {
			t.Fatalf("%s Datetime.parse one-digit year parsed", locale)
		}
	}
	for _, locale := range []string{"fr_CA", "ja_JP"} {
		value, err := parseDateParseTextForLocale("1/2/65", locale)
		if err != nil || value.Year() != 2065 {
			t.Fatalf("%s Date.parse two-digit 65: %v %v", locale, value, err)
		}
	}
	if value, err := parseDateParseTextForLocale("1/2/65", "en_US"); err != nil || value.Year() != 1965 {
		t.Fatalf("en_US Date.parse 65: %v %v", value, err)
	}
}
