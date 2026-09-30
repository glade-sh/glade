package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// API 67.0 Date.format() in the retained Summer '26 catalog
// 9d96c8f2472acb64d9d2860ab22ebb1e526edccf137b659c2bca441b1ce86a7b:
// apex/apex_methods_system_date.md /documents/2195/members/7, public String
// format(), returns the Date in the context user's locale. Salesforce's
// locale-neutral-code guide confirms Apex format() uses that locale. The ICU
// 71.1/CLDR 41 supported-format table shows short date/time examples for the
// exact input as en_US 1/28/2008, 4:30 PM and de_DE 28.01.2008, 16:30; this
// test checks their date components:
// https://help.salesforce.com/s/articleView?id=sf.admin_locales_code_methods.htm&language=en_US&type=5
// https://help.salesforce.com/s/articleView?id=sf.admin_supported_date_time_format.htm&language=en_US&type=5
// https://help.salesforce.com/s/articleView?id=sf.admin_locales_icu.htm&language=en_US&type=5
// This regression is limited to en_US and de_DE under API 67.0.
func TestExecDateFormatUsesContextUserLocaleAPI67(t *testing.T) {
	cases := []struct {
		name              string
		source            string
		currentUser       storage.Record
		setCurrentUser    bool
		enableTestContext bool
	}{
		{
			name: "fallbackLocaleUSControl",
			source: `
System.assertEquals('en_US', UserInfo.getLocale());
Date sample = Date.newInstance(2008, 1, 28);
System.assertEquals('1/28/2008', sample.format());
`,
		},
		{
			name: "enUSLocaleWinsOverGermanLanguage",
			source: `
System.assertEquals('en_US', UserInfo.getLocale());
System.assertEquals('de', UserInfo.getLanguage());
Date sample = Date.newInstance(2008, 1, 28);
System.assertEquals('1/28/2008', sample.format());
`,
			currentUser: storage.Record{
				ID:     "005-date-format-en-locale",
				Object: "User",
				Fields: map[string]storage.Value{
					"LocaleSidKey":      storage.StringValue("en_US"),
					"LanguageLocaleKey": storage.StringValue("de"),
				},
			},
			setCurrentUser: true,
		},
		{
			name: "deDELocaleUsesLocalizedDateWithEnglishLanguage",
			source: `
System.assertEquals('de_DE', UserInfo.getLocale());
System.assertEquals('en_US', UserInfo.getLanguage());
Date sample = Date.newInstance(2008, 1, 28);
System.assertEquals('28.01.2008', sample.format());
`,
			currentUser: storage.Record{
				ID:     "005-date-format-de-locale",
				Object: "User",
				Fields: map[string]storage.Value{
					"LocaleSidKey":      storage.StringValue("de_DE"),
					"LanguageLocaleKey": storage.StringValue("en_US"),
				},
			},
			setCurrentUser: true,
		},
		{
			name: "runAsUsesGermanLocaleForDateFormat",
			source: `
System.assertEquals('en_US', UserInfo.getLocale());
Date sample = Date.newInstance(2008, 1, 28);
System.runAs(new User(Id = '005-date-format-runas-de', LocaleSidKey = 'de_DE', LanguageLocaleKey = 'en_US')) {
    System.assertEquals('de_DE', UserInfo.getLocale());
    System.assertEquals('en_US', UserInfo.getLanguage());
    System.assertEquals('28.01.2008', sample.format());
}
`,
			currentUser: storage.Record{
				ID:     "005-date-format-outer-en",
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
			name: "runAsRestoresOuterUSDateFormat",
			source: `
System.assertEquals('en_US', UserInfo.getLocale());
System.assertEquals('de', UserInfo.getLanguage());
Date sample = Date.newInstance(2008, 1, 28);
System.runAs(new User(Id = '005-date-format-restore-de', LocaleSidKey = 'de_DE', LanguageLocaleKey = 'en_US')) {
    System.assertEquals('de_DE', UserInfo.getLocale());
}
System.assertEquals('en_US', UserInfo.getLocale());
System.assertEquals('de', UserInfo.getLanguage());
System.assertEquals('1/28/2008', sample.format());
`,
			currentUser: storage.Record{
				ID:     "005-date-format-restore-en",
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
