package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The API 67.0 source is apex/apex_methods_system_decimal.md, catalog SHA-256
// 9d96c8f2472acb64d9d2860ab22ebb1e526edccf137b659c2bca441b1ce86a7b:
// /documents/2197/members/12 requires Decimal.format to use the context user's
// locale. The supported number-format table gives the exact positive and
// negative examples for 1234567.567 in en_US and de_DE:
// https://help.salesforce.com/s/articleView?id=admin_supported_locales.htm&language=en_US&type=5
// https://help.salesforce.com/s/articleView?id=xcloud.admin_supported_locales.htm&language=en_US&type=5
// The ICU 71.1 / CLDR 41 profile is specified by
// https://help.salesforce.com/s/articleView?id=sf.admin_locales_icu.htm&language=en_US&type=5 .
// This regression is bounded to those API 67.0 ICU locale profiles.
func TestExecDecimalFormatUsesContextUserLocaleAPI67(t *testing.T) {
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
System.assertEquals('1,234,567.567', Decimal.valueOf('1234567.567').format());
System.assertEquals('-1,234,567.567', Decimal.valueOf('-1234567.567').format());
`,
		},
		{
			name: "enUSLocaleWinsOverGermanLanguage",
			source: `
System.assertEquals('en_US', UserInfo.getLocale());
System.assertEquals('de', UserInfo.getLanguage());
System.assertEquals('1,234,567.567', Decimal.valueOf('1234567.567').format());
System.assertEquals('-1,234,567.567', Decimal.valueOf('-1234567.567').format());
`,
			currentUser: storage.Record{
				ID:     "005-decimal-en-locale",
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
System.assertEquals('1.234.567,567', Decimal.valueOf('1234567.567').format());
System.assertEquals('-1.234.567,567', Decimal.valueOf('-1234567.567').format());
`,
			currentUser: storage.Record{
				ID:     "005-decimal-de-locale",
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
System.runAs(new User(Id = '005-decimal-runas-de', LocaleSidKey = 'de_DE', LanguageLocaleKey = 'en_US')) {
    System.assertEquals('de_DE', UserInfo.getLocale());
    System.assertEquals('en_US', UserInfo.getLanguage());
    System.assertEquals('1.234.567,567', Decimal.valueOf('1234567.567').format());
    System.assertEquals('-1.234.567,567', Decimal.valueOf('-1234567.567').format());
}
`,
			currentUser: storage.Record{
				ID:     "005-decimal-outer-en",
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
System.runAs(new User(Id = '005-decimal-restore-de', LocaleSidKey = 'de_DE', LanguageLocaleKey = 'en_US')) {
    System.assertEquals('de_DE', UserInfo.getLocale());
    System.assertEquals('en_US', UserInfo.getLanguage());
}
System.assertEquals('en_US', UserInfo.getLocale());
System.assertEquals('de', UserInfo.getLanguage());
System.assertEquals('1,234,567.567', Decimal.valueOf('1234567.567').format());
System.assertEquals('-1,234,567.567', Decimal.valueOf('-1234567.567').format());
`,
			currentUser: storage.Record{
				ID:     "005-decimal-restore-en",
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
