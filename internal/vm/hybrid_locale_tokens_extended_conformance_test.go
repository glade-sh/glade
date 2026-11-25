package vm

import (
	"encoding/csv"
	"os"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestHybridLocaleTokensExtendedNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/hybrid_locale_tokens_extended/"
	readTSV := func(name string) [][]string {
		t.Helper()
		file, err := os.Open(fixture + name)
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
		return rows
	}
	rows, profiles := readTSV("org.tsv"), readTSV("profiles.tsv")
	if len(rows) != 25 || len(profiles) != 11 {
		t.Fatalf("incomplete native capture: %d samples and %d profiles", len(rows)-1, len(profiles)-1)
	}
	source, err := os.ReadFile(fixture + "observe.apex")
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "63.0", "64.0", "65.0", "66.0", "67.0", "68.0"} {
		program, err := CompileAnonymousWithOptions(string(source), CompileOptions{APIVersion: api})
		if err != nil {
			t.Fatal(err)
		}
		for _, profile := range profiles[1:] {
			locale, language := profile[0], profile[1]
			for _, route := range []string{"anonymous", "istest"} {
				t.Run(api+"/"+locale+"/"+route, func(t *testing.T) {
					machine := New(nil)
					machine.SetCurrentUser(storage.Record{Object: "User", Fields: map[string]storage.Value{
						"LocaleSidKey": storage.StringValue(locale), "LanguageLocaleKey": storage.StringValue(language), "TimeZoneSidKey": storage.StringValue("GMT"),
					}})
					var result Result
					var err error
					if route == "istest" {
						machine.EnableTestContext()
						machine.testContext.CurrentUser = machine.executionUser
						result, err = machine.ExecuteTestInClass(program, "Road473LocaleProbe")
					} else {
						result, err = machine.Execute(program)
					}
					checkHybridLocaleTokenResults(t, result, err, locale, language, rows[1:])
				})
			}
		}
	}
}

func checkHybridLocaleTokenResults(t *testing.T, result Result, err error, locale, language string, rows [][]string) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	routes := []string{"local", "explicit", "gmt"}
	if want := len(rows)*len(routes) + 1; len(result.Debug) != want {
		t.Fatalf("observed %d debug entries, want %d", len(result.Debug), want)
	}
	if want := "RD473_LOCALE=" + locale + "|" + language + "|GMT"; result.Debug[0] != want {
		t.Fatalf("context %q, native %q", result.Debug[0], want)
	}
	for i, row := range rows {
		for j, route := range routes {
			want := "RD473_ROW=" + row[0] + "|" + route + "|" + row[1]
			if got := result.Debug[1+i*len(routes)+j]; got != want {
				t.Fatalf("sample %s via %s: got %q, native %q", row[0], route, got, want)
			}
		}
	}
}
