package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// API67 Datetime.parse uses the local time zone and user locale format.
// Catalog SHA256 9d96c8f2472acb64d9d2860ab22ebb1e526edccf137b659c2bca441b1ce86a7b,
// /documents/2196/members/12 and /documents/2196/members/35; Datetime guide
// SHA256 e2dfc8a96658dd25bfc2d6b347ecabc95ca636e268a5c225cdbcc582a0e8417f.
// The admitted full-clock locale grammar joins the retained ICU 71.1 / CLDR 41
// rows to parse's explicit local-format contract. Retained manifest SHA256s:
// e8ac8b2d856d28f6aca9ecfd0a0ebb0abe7d7f226c8bd84da30a783463006ac0
// and 5e8ee4d63df0659ef0e8441d5d5ac6a475541ff8c518eafb0e747c635a9327dd.
// These four GMT contexts qualify local and GMT components through minute
// precision only, without seconds, milliseconds, epoch, date-only, DST,
// offset, padding, year-width, or whole-locale parity predicates.
func TestExecDatetimeParseUsesContextUserFullClockLocaleAPI67(t *testing.T) {
	cases := []struct {
		name     string
		locale   string
		language string
		text     string
	}{
		{name: "GermanLocaleEnglishLanguageGMT", locale: "de_DE", language: "en_US", text: "28.01.2008, 16:30"},
		{name: "BritishLocaleEnglishLanguageGMT", locale: "en_GB", language: "en_US", text: "28/01/2008, 16:30"},
		{name: "USLocaleEnglishLanguageGMT", locale: "en_US", language: "en_US", text: "1/28/2008, 4:30 PM"},
		{name: "USLocaleGermanLanguageGMT", locale: "en_US", language: "de", text: "1/28/2008, 4:30 PM"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := "System.assertEquals('" + tc.locale + "', UserInfo.getLocale());" +
				"System.assertEquals('" + tc.language + "', UserInfo.getLanguage());" +
				"TimeZone zone = UserInfo.getTimeZone(); System.assertEquals('GMT', zone.getID());" +
				"Datetime parsed = Datetime.parse('" + tc.text + "');" + `
System.assertEquals(2008, parsed.year());
System.assertEquals(1, parsed.month());
System.assertEquals(28, parsed.day());
System.assertEquals(16, parsed.hour());
System.assertEquals(30, parsed.minute());
System.assertEquals(2008, parsed.yearGmt());
System.assertEquals(1, parsed.monthGmt());
System.assertEquals(28, parsed.dayGmt());
System.assertEquals(16, parsed.hourGmt());
System.assertEquals(30, parsed.minuteGmt());
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
				ID:     storage.ID("005-datetime-parse-" + tc.name),
				Object: "User",
				Fields: map[string]storage.Value{
					"LocaleSidKey":      storage.StringValue(tc.locale),
					"LanguageLocaleKey": storage.StringValue(tc.language),
					"TimeZoneSidKey":    storage.StringValue("GMT"),
				},
			})
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
