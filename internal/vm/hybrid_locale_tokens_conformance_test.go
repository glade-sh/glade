package vm

import (
	"encoding/csv"
	"html"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// NATIVE hybrid_locale_tokens/fr/org.tsv:2 anonymous 62.0
// NATIVE hybrid_locale_tokens/fr/org.tsv:3 anonymous 67.0
// NATIVE hybrid_locale_tokens/fr/org.tsv:4 istest 62.0
// NATIVE hybrid_locale_tokens/fr/org.tsv:5 istest 67.0
// NATIVE hybrid_locale_tokens/de/org.tsv:2 anonymous 62.0
// NATIVE hybrid_locale_tokens/de/org.tsv:3 anonymous 67.0
// NATIVE hybrid_locale_tokens/de/org.tsv:4 istest 62.0
// NATIVE hybrid_locale_tokens/de/org.tsv:5 istest 67.0
func TestHybridLocaleTokensNativeConformance(t *testing.T) {
	for _, locale := range []string{"fr", "de"} {
		t.Run(locale, func(t *testing.T) { testHybridLocaleTokens(t, locale) })
	}
}

func testHybridLocaleTokens(t *testing.T, locale string) {
	f, err := os.Open("testdata/conformance/hybrid_locale_tokens/" + locale + "/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		row := row
		t.Run(row[1]+"/"+row[2], func(t *testing.T) {
			want := html.UnescapeString(row[5])
			if row[2] == "anonymous" {
				want = strings.TrimPrefix(want, "OBS=")
			} else {
				_, want, _ = strings.Cut(want, "Actual: ")
			}
			source := `Datetime d = Datetime.newInstanceGmt(1500,2,28,12,0,0); String observation = d.format('EEEE|EEE|MMMM|MMM|u','GMT') + '|' + d.addDays(1).format('EEEE|EEE|MMMM|MMM|u','GMT') + '|' + Datetime.newInstanceGmt(1583,1,1,12,0,0).format('EEEE|EEE|MMMM|MMM|u','GMT');`
			if locale == "de" {
				source = strings.Replace(source, "String observation =", "String observation = UserInfo.getLocale() + '|' +", 1)
				source = strings.Replace(source, "d.format('EEEE|EEE|MMMM|MMM|u','GMT')", "d.format('EEEE|EEE|MMMM|MMM|u')", 1)
			}
			program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: row[1]})
			if err != nil {
				t.Fatal(err)
			}
			localeKey, language := "fr_FR", "fr"
			if locale == "de" {
				localeKey, language = "de_DE", "de"
			}
			machine := New(nil)
			machine.SetCurrentUser(storage.Record{Object: "User", Fields: map[string]storage.Value{"LocaleSidKey": storage.StringValue(localeKey), "LanguageLocaleKey": storage.StringValue(language), "TimeZoneSidKey": storage.StringValue("GMT")}})
			if row[2] == "istest" {
				machine.EnableTestContext()
				machine.testContext.CurrentUser = machine.executionUser
			}
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
			if got := machine.Globals["observation"].Text; got != want {
				t.Fatalf("got %q; native %q", got, want)
			}
		})
	}
}

func TestHybridLocaleTokensUncapturedNeighbor(t *testing.T) {
	days := hybridDays(1500, 2, 28)
	for _, tc := range []struct{ pattern, locale, api string }{
		{"EEEEEE|uu", "fr_FR", "67.0"},
		{"EEE|EEEE|u", "es_MX", "67.0"},
		{"EEE|EEEE|u", "fr_FR", "61.0"},
	} {
		fields := newHybridDatetimeFields(days, tc.locale, tc.api)
		for _, token := range strings.Split(tc.pattern, "|") {
			if got := fields.formatToken(token); got != "" {
				t.Errorf("locale %s, API %s: token %q unexpectedly rendered as %q", tc.locale, tc.api, token, got)
			}
		}
	}
}

func TestHybridLocaleDatetimePatternRegression(t *testing.T) {
	for _, tc := range []struct {
		name, source, want string
	}{
		{"adjacent tokens", `String observation = d.format('Eu', 'GMT');`, "Fri5"},
		{"adjacent quoted literal", `String observation = d.format('E\'X\'', 'GMT');`, "FriX"},
		{"invalid token", `String observation;
			try {
				d.format('E|QQ', 'GMT');
			} catch (System.StringException e) {
				observation = e.getMessage();
			}`, "Unrecognized format: E|QQ"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := `Datetime d = Datetime.newInstanceGmt(1582, 12, 31, 12, 0, 0);` + tc.source
			program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: "67.0"})
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			machine.SetCurrentUser(storage.Record{Object: "User", Fields: map[string]storage.Value{
				"LocaleSidKey":      storage.StringValue("es_ES"),
				"LanguageLocaleKey": storage.StringValue("es"),
				"TimeZoneSidKey":    storage.StringValue("GMT"),
			}})
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
			if got := machine.Globals["observation"].Text; got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
