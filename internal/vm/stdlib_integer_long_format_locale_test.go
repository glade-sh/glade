package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The API 67.0 source catalog SHA-256 is
// 9d96c8f2472acb64d9d2860ab22ebb1e526edccf137b659c2bca441b1ce86a7b.
// Integer.format uses the context user's locale, as specified by
// apex/apex_methods_system_integer.md at /documents/2203/members/0.
// Long.format has the same locale requirement in
// apex/apex_methods_system_long.md at /documents/2206/members/0.
// The supported number-format table specifies comma grouping for en_US and
// dot grouping for de_DE, with an ASCII minus sign for both locales:
// https://help.salesforce.com/s/articleView?id=admin_supported_locales.htm&language=en_US&type=5
// https://help.salesforce.com/s/articleView?id=xcloud.admin_supported_locales.htm&language=en_US&type=5
// The ICU 71.1 / CLDR 41 profile is specified by
// https://help.salesforce.com/s/articleView?id=sf.admin_locales_icu.htm&language=en_US&type=5 .
// This regression uses only integral values in those API 67.0 ICU profiles.
func TestExecIntegerLongFormatUsesContextUserLocaleAPI67(t *testing.T) {
	numberTypes := []struct {
		name         string
		declarations string
	}{
		{
			name: "Integer",
			declarations: `
Integer positive = 1234567;
Integer negative = -1234567;
`,
		},
		{
			name: "Long",
			declarations: `
Long positive = 1234567L;
Long negative = -1234567L;
`,
		},
	}
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
System.assertEquals('1,234,567', positive.format());
System.assertEquals('-1,234,567', negative.format());
`,
		},
		{
			name: "enUSLocaleWinsOverGermanLanguage",
			source: `
System.assertEquals('en_US', UserInfo.getLocale());
System.assertEquals('de', UserInfo.getLanguage());
System.assertEquals('1,234,567', positive.format());
System.assertEquals('-1,234,567', negative.format());
`,
			currentUser: storage.Record{
				ID:     "005-integral-en-locale",
				Object: "User",
				Fields: map[string]storage.Value{
					"LocaleSidKey":      storage.StringValue("en_US"),
					"LanguageLocaleKey": storage.StringValue("de"),
				},
			},
			setCurrentUser: true,
		},
		{
			name: "deDELocaleWinsOverEnglishLanguage",
			source: `
System.assertEquals('de_DE', UserInfo.getLocale());
System.assertEquals('en_US', UserInfo.getLanguage());
System.assertEquals('1.234.567', positive.format());
System.assertEquals('-1.234.567', negative.format());
`,
			currentUser: storage.Record{
				ID:     "005-integral-de-locale",
				Object: "User",
				Fields: map[string]storage.Value{
					"LocaleSidKey":      storage.StringValue("de_DE"),
					"LanguageLocaleKey": storage.StringValue("en_US"),
				},
			},
			setCurrentUser: true,
		},
		{
			name: "runAsUsesGermanLocale",
			source: `
System.assertEquals('en_US', UserInfo.getLocale());
System.assertEquals('de', UserInfo.getLanguage());
System.runAs(new User(Id = '005-integral-runas-de', LocaleSidKey = 'de_DE', LanguageLocaleKey = 'en_US')) {
    System.assertEquals('de_DE', UserInfo.getLocale());
    System.assertEquals('en_US', UserInfo.getLanguage());
    System.assertEquals('1.234.567', positive.format());
    System.assertEquals('-1.234.567', negative.format());
}
`,
			currentUser: storage.Record{
				ID:     "005-integral-outer-en",
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
System.assertEquals('de', UserInfo.getLanguage());
System.runAs(new User(Id = '005-integral-restore-de', LocaleSidKey = 'de_DE', LanguageLocaleKey = 'en_US')) {
    System.assertEquals('de_DE', UserInfo.getLocale());
    System.assertEquals('en_US', UserInfo.getLanguage());
}
System.assertEquals('en_US', UserInfo.getLocale());
System.assertEquals('de', UserInfo.getLanguage());
System.assertEquals('1,234,567', positive.format());
System.assertEquals('-1,234,567', negative.format());
`,
			currentUser: storage.Record{
				ID:     "005-integral-restore-en",
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

	for _, numberType := range numberTypes {
		t.Run(numberType.name, func(t *testing.T) {
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					program, err := CompileAnonymousWithOptions(numberType.declarations+tc.source, CompileOptions{APIVersion: "67.0"})
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
		})
	}
}
