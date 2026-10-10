package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// API67 Date.parse uses the local date format: catalog SHA256
// 9d96c8f2472acb64d9d2860ab22ebb1e526edccf137b659c2bca441b1ce86a7b,
// /documents/2195/members/7 and /documents/2195/members/13. The admitted
// local-format inverse inference uses the retained ICU 71.1 / CLDR 41 table's
// date components for 2008-01-28: en_US 1/28/2008, de_DE 28.01.2008 and
// en_GB 28/01/2008. It does not establish padding, two-digit years,
// cross-locale rejection, Datetime semantics, or whole-locale parity.
// Date guide SHA256 7d09f0431d0c1d81cd04692194b4f3b18b1336e41165e4a083cba7a65e703c08;
// retained ICU date collection manifest SHA256
// e8ac8b2d856d28f6aca9ecfd0a0ebb0abe7d7f226c8bd84da30a783463006ac0
// and dot/slash manifest SHA256
// 5e8ee4d63df0659ef0e8441d5d5ac6a475541ff8c518eafb0e747c635a9327dd.
func TestExecDateParseUsesContextUserLocaleAPI67(t *testing.T) {
	cases := []struct {
		name     string
		locale   string
		language string
		text     string
	}{
		{name: "GermanLocaleEnglishLanguage", locale: "de_DE", language: "en_US", text: "28.01.2008"},
		{name: "BritishLocaleEnglishLanguage", locale: "en_GB", language: "en_US", text: "28/01/2008"},
		{name: "USLocaleGermanLanguage", locale: "en_US", language: "de", text: "1/28/2008"},
		{name: "USLocaleEnglishLanguage", locale: "en_US", language: "en_US", text: "1/28/2008"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := "System.assertEquals('" + tc.locale + "', UserInfo.getLocale());" +
				"System.assertEquals('" + tc.language + "', UserInfo.getLanguage());" +
				"Date parsed = Date.parse('" + tc.text + "');" + `
System.assertEquals(2008, parsed.year());
System.assertEquals(1, parsed.month());
System.assertEquals(28, parsed.day());
`
			program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: "67.0"})
			if err != nil {
				t.Fatal(err)
			}
			if program.APIVersion != "67.0" {
				t.Fatalf("compiled API version = %q, want 67.0", program.APIVersion)
			}
			machine := New(nil)
			machine.SetCurrentUser(storage.Record{
				ID:     storage.ID("005-date-parse-" + tc.name),
				Object: "User",
				Fields: map[string]storage.Value{
					"LocaleSidKey":      storage.StringValue(tc.locale),
					"LanguageLocaleKey": storage.StringValue(tc.language),
				},
			})
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
