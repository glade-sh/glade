package vm

import (
	"encoding/csv"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Corpus expected-failure control (pattern) for the locale environment shape: a test that
// calls Date.parse('yyyy-mm-dd') fails for an en_US user and passes for an en_CA user. The rows were
// captured in a scratch org at APIs 62.0 and 67.0 (../apextest/testdata/conformance/expected_env_controls,
// date_parse_locale.tsv, ORG-RUN.txt). The en_US rows already agreed; the en_CA rows are the new
// modelled locale. Only inputs present in the capture are asserted.
func TestDateParseLocaleOrgRows(t *testing.T) {
	f, err := os.Open("../apextest/testdata/conformance/expected_env_controls/date_parse_locale.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) < 40 {
		t.Fatalf("rows=%d err=%v", len(rows), err)
	}
	seen := map[string]int{}
	for _, row := range rows[1:] {
		if len(row) != 4 {
			t.Fatalf("invalid row %q", row)
		}
		api, locale, input, want := row[0], row[1], row[2], row[3]
		seen[locale]++
		source := "String r; try { r = String.valueOf(Date.parse('" + input + "')); } catch (Exception e) { r = e.getTypeName() + ': ' + e.getMessage(); }\n" +
			"System.assertEquals('" + strings.ReplaceAll(want, "'", "\\'") + "', r);"
		program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
		if err != nil {
			t.Fatal(err)
		}
		machine := New(nil)
		machine.SetCurrentUser(storage.Record{ID: storage.ID("005-date-parse-org-row"), Object: "User", Fields: map[string]storage.Value{
			"LocaleSidKey": storage.StringValue(locale), "LanguageLocaleKey": storage.StringValue("en_US")}})
		if _, err := machine.Execute(program); err != nil {
			t.Errorf("api %s locale %s input %q: want %q: %v", api, locale, input, want, err)
		}
	}
	if seen["en_CA"] != 22 || seen["en_US"] != 22 {
		t.Fatalf("capture shape changed: %v", seen)
	}
}
