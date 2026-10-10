package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// API 67.0 Date.format() uses the context user's locale (Summer '26 catalog
// 9d96c8f2472acb64d9d2860ab22ebb1e526edccf137b659c2bca441b1ce86a7b,
// apex/apex_methods_system_date.md /documents/2195/members/7). The official
// ICU 71.1 / CLDR 41 supported date/time table lists 28/01/2008, 16:30 for
// en_GB and 28/01/2008 16:30 for fr_FR on 2008-01-28. The Date-only expected
// values below infer 28/01/2008 from those exact rows; they qualify only this
// input and these two locale profiles, not generalized date padding or all
// locales. The en_US row is a control. Current profiles en_US/de_DE retain
// their separately tested behavior.
// https://help.salesforce.com/s/articleView?id=sf.admin_supported_date_time_format.htm&language=en_US&type=5
// https://help.salesforce.com/s/articleView?id=sf.admin_locales_code_methods.htm&language=en_US&type=5
// https://help.salesforce.com/s/articleView?id=sf.admin_locales_icu.htm&language=en_US&type=5
func TestExecDateFormatUsesContextUserDayFirstSlashLocaleGroupAPI67(t *testing.T) {
	cases := []struct {
		name        string
		source      string
		currentUser storage.Record
	}{
		{
			name: "enUSControlGermanLanguage",
			source: `
System.assertEquals('en_US', UserInfo.getLocale());
System.assertEquals('de', UserInfo.getLanguage());
Date sample = Date.newInstance(2008, 1, 28);
System.assertEquals('1/28/2008', sample.format());
`,
			currentUser: storage.Record{
				ID:     "005-date-format-dmy-en-us",
				Object: "User",
				Fields: map[string]storage.Value{
					"LocaleSidKey":      storage.StringValue("en_US"),
					"LanguageLocaleKey": storage.StringValue("de"),
				},
			},
		},
		{
			name: "enGBLocaleWithGermanLanguage",
			source: `
System.assertEquals('en_GB', UserInfo.getLocale());
System.assertEquals('de', UserInfo.getLanguage());
Date sample = Date.newInstance(2008, 1, 28);
System.assertEquals('28/01/2008', sample.format());
`,
			currentUser: storage.Record{
				ID:     "005-date-format-dmy-en-gb",
				Object: "User",
				Fields: map[string]storage.Value{
					"LocaleSidKey":      storage.StringValue("en_GB"),
					"LanguageLocaleKey": storage.StringValue("de"),
				},
			},
		},
		{
			name: "frFRLocaleWithEnglishLanguage",
			source: `
System.assertEquals('fr_FR', UserInfo.getLocale());
System.assertEquals('en_US', UserInfo.getLanguage());
Date sample = Date.newInstance(2008, 1, 28);
System.assertEquals('28/01/2008', sample.format());
`,
			currentUser: storage.Record{
				ID:     "005-date-format-dmy-fr-fr",
				Object: "User",
				Fields: map[string]storage.Value{
					"LocaleSidKey":      storage.StringValue("fr_FR"),
					"LanguageLocaleKey": storage.StringValue("en_US"),
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(tc.source, CompileOptions{APIVersion: "67.0"})
			if err != nil {
				t.Fatal(err)
			}
			if program.APIVersion != "67.0" {
				t.Fatalf("compiled API version = %q, want 67.0", program.APIVersion)
			}

			machine := New(nil)
			machine.SetCurrentUser(tc.currentUser)
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
