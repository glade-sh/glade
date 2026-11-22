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
func TestHybridLocaleTokensNativeConformance(t *testing.T) {
	f, err := os.Open("testdata/conformance/hybrid_locale_tokens/fr/org.tsv")
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
			program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: row[1]})
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			machine.SetCurrentUser(storage.Record{ID: "005-synthetic-locale", Object: "User", Fields: map[string]storage.Value{"LocaleSidKey": storage.StringValue("fr_FR"), "LanguageLocaleKey": storage.StringValue("fr"), "TimeZoneSidKey": storage.StringValue("GMT")}})
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
