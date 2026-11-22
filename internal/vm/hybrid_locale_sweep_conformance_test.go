package vm

import (
	"encoding/csv"
	"html"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// NATIVE hybrid_locale_tokens/fr_sweep/org.tsv:2 anonymous 62.0
// NATIVE hybrid_locale_tokens/fr_sweep/org.tsv:3 anonymous 67.0
// NATIVE hybrid_locale_tokens/fr_sweep/org.tsv:4 istest 62.0
// NATIVE hybrid_locale_tokens/fr_sweep/org.tsv:5 istest 67.0
// NATIVE hybrid_locale_tokens/de_sweep/org.tsv:2 anonymous 62.0
// NATIVE hybrid_locale_tokens/de_sweep/org.tsv:3 anonymous 67.0
// NATIVE hybrid_locale_tokens/de_sweep/org.tsv:4 istest 62.0
// NATIVE hybrid_locale_tokens/de_sweep/org.tsv:5 istest 67.0
func TestHybridLocaleSweepNativeConformance(t *testing.T) {
	for _, tc := range []struct{ fixture, locale, language string }{
		{"fr_sweep", "fr_FR", "fr"}, {"de_sweep", "de_DE", "de"},
	} {
		t.Run(tc.fixture, func(t *testing.T) {
			f, err := os.Open("testdata/conformance/hybrid_locale_tokens/" + tc.fixture + "/org.tsv")
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
					source := `String result = '';
for (Integer month = 1; month <= 12; month++) {
 Datetime d = Datetime.newInstanceGmt(1500,month,15,12,0,0);
 result += d.format('EEEE|EEE|MMMM|MMM|u','GMT') + ';';
}
for (Integer day = 1; day <= 7; day++) {
 Datetime d = Datetime.newInstanceGmt(1400,1,day,12,0,0);
 result += d.format('EEEE|EEE|MMMM|MMM|u','GMT') + ';';
}`
					program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: row[1]})
					if err != nil {
						t.Fatal(err)
					}
					machine := New(nil)
					machine.SetCurrentUser(storage.Record{Object: "User", Fields: map[string]storage.Value{"LocaleSidKey": storage.StringValue(tc.locale), "LanguageLocaleKey": storage.StringValue(tc.language), "TimeZoneSidKey": storage.StringValue("GMT")}})
					if row[2] == "istest" {
						machine.EnableTestContext()
						machine.testContext.CurrentUser = machine.executionUser
					}
					if _, err := machine.Execute(program); err != nil {
						t.Fatal(err)
					}
					if got := machine.Globals["result"].Text; got != want {
						t.Fatalf("got %q; native %q", got, want)
					}
				})
			}
		})
	}
}
