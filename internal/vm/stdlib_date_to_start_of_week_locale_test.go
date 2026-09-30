package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The API 67.0 source is apex/apex_methods_system_date.md, catalog SHA-256
// 9d96c8f2472acb64d9d2860ab22ebb1e526edccf137b659c2bca441b1ce86a7b:
// /documents/2195/members/15 and /documents/2195/examples/15. The example
// distinguishes Sunday in the United States from Monday in European locales.
// The supported-locale table is
// https://help.salesforce.com/s/articleView?id=sf.admin_supported_date_time_format.htm&language=en_US&type=5
// and its ICU/CLDR policy is
// https://help.salesforce.com/s/articleView?id=sf.admin_locales_icu.htm&language=en_US&type=5 .
// This regression is bounded to en_US (Sunday) and de_DE (Monday).
func TestExecDateToStartOfWeekUsesContextUserLocaleAPI67(t *testing.T) {
	cases := []struct {
		name              string
		source            string
		currentUser       storage.Record
		setCurrentUser    bool
		enableTestContext bool
	}{
		{
			name: "fallbackLocaleSundayControl",
			source: `
System.assertEquals('en_US', UserInfo.getLocale());
Date thursday = Date.newInstance(2024, 2, 29);
System.assertEquals(Date.newInstance(2024, 2, 25), thursday.toStartOfWeek());
`,
		},
		{
			name: "enUSLocaleWinsOverGermanLanguage",
			source: `
System.assertEquals('en_US', UserInfo.getLocale());
System.assertEquals('de', UserInfo.getLanguage());
Date wednesday = Date.newInstance(2024, 2, 28);
Date thursday = Date.newInstance(2024, 2, 29);
System.assertEquals(Date.newInstance(2024, 2, 25), wednesday.toStartOfWeek());
System.assertEquals(Date.newInstance(2024, 2, 25), thursday.toStartOfWeek());
`,
			currentUser: storage.Record{
				ID:     "005-week-en-locale",
				Object: "User",
				Fields: map[string]storage.Value{
					"LocaleSidKey":      storage.StringValue("en_US"),
					"LanguageLocaleKey": storage.StringValue("de"),
				},
			},
			setCurrentUser: true,
		},
		{
			name: "deDECurrentUserThursdayStartsMonday",
			source: `
System.assertEquals('de_DE', UserInfo.getLocale());
System.assertEquals('en_US', UserInfo.getLanguage());
Date thursday = Date.newInstance(2024, 2, 29);
System.assertEquals(Date.newInstance(2024, 2, 26), thursday.toStartOfWeek());
`,
			currentUser: storage.Record{
				ID:     "005-week-de-locale-thursday",
				Object: "User",
				Fields: map[string]storage.Value{
					"LocaleSidKey":      storage.StringValue("de_DE"),
					"LanguageLocaleKey": storage.StringValue("en_US"),
				},
			},
			setCurrentUser: true,
		},
		{
			name: "deDESundayStartsPriorMonday",
			source: `
System.assertEquals('de_DE', UserInfo.getLocale());
System.assertEquals('en_US', UserInfo.getLanguage());
Date sunday = Date.newInstance(2024, 3, 3);
System.assertEquals(Date.newInstance(2024, 2, 26), sunday.toStartOfWeek());
`,
			currentUser: storage.Record{
				ID:     "005-week-de-locale-sunday",
				Object: "User",
				Fields: map[string]storage.Value{
					"LocaleSidKey":      storage.StringValue("de_DE"),
					"LanguageLocaleKey": storage.StringValue("en_US"),
				},
			},
			setCurrentUser: true,
		},
		{
			name: "runAsDeDEStartsMonday",
			source: `
System.assertEquals('en_US', UserInfo.getLocale());
Date thursday = Date.newInstance(2024, 2, 29);
System.runAs(new User(Id = '005-week-runas-de', LocaleSidKey = 'de_DE', LanguageLocaleKey = 'en_US')) {
    System.assertEquals('de_DE', UserInfo.getLocale());
    System.assertEquals('en_US', UserInfo.getLanguage());
    System.assertEquals(Date.newInstance(2024, 2, 26), thursday.toStartOfWeek());
}
`,
			currentUser: storage.Record{
				ID:     "005-week-outer-en",
				Object: "User",
				Fields: map[string]storage.Value{
					"LocaleSidKey":      storage.StringValue("en_US"),
					"LanguageLocaleKey": storage.StringValue("de"),
				},
			},
			setCurrentUser:    true,
			enableTestContext: true,
		},
		{
			name: "runAsRestoresOuterLocale",
			source: `
System.assertEquals('en_US', UserInfo.getLocale());
Date thursday = Date.newInstance(2024, 2, 29);
System.runAs(new User(Id = '005-week-restore-de', LocaleSidKey = 'de_DE', LanguageLocaleKey = 'en_US')) {
    System.assertEquals('de_DE', UserInfo.getLocale());
}
System.assertEquals('en_US', UserInfo.getLocale());
System.assertEquals('de', UserInfo.getLanguage());
System.assertEquals(Date.newInstance(2024, 2, 25), thursday.toStartOfWeek());
`,
			currentUser: storage.Record{
				ID:     "005-week-restore-en",
				Object: "User",
				Fields: map[string]storage.Value{
					"LocaleSidKey":      storage.StringValue("en_US"),
					"LanguageLocaleKey": storage.StringValue("de"),
				},
			},
			setCurrentUser:    true,
			enableTestContext: true,
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
			if tc.setCurrentUser {
				machine.SetCurrentUser(tc.currentUser)
			}
			if tc.enableTestContext {
				machine.EnableTestContext()
			}
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
