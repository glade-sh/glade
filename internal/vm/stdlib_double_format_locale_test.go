package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// API67 Double.format uses the context user's locale: catalog SHA256
// 9d96c8f2472acb64d9d2860ab22ebb1e526edccf137b659c2bca441b1ce86a7b,
// /documents/2198/members/0; retained Double guide SHA256
// 40a6f94ac1fc4d3a7187bce569379a8565e07d53e7d7b9cbd976c6271bf17ed1,
// lines 30-47. Expected strings reuse the accepted API67 ICU71.1/CLDR41
// en_US/de_DE numeric locale predicate; no other locale or rounding claim.
func TestExecDoubleFormatUsesContextUserLocaleAPI67(t *testing.T) {
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
System.assertEquals('1,234,567.567', Double.valueOf('1234567.567').format());
System.assertEquals('-1,234,567.567', Double.valueOf('-1234567.567').format());
`,
		},
		{
			name: "enUSLocaleWinsOverGermanLanguage",
			source: `
System.assertEquals('en_US', UserInfo.getLocale());
System.assertEquals('de', UserInfo.getLanguage());
System.assertEquals('1,234,567.567', Double.valueOf('1234567.567').format());
System.assertEquals('-1,234,567.567', Double.valueOf('-1234567.567').format());
`,
			currentUser: storage.Record{
				ID:     "005-double-en-locale",
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
System.assertEquals('1.234.567,567', Double.valueOf('1234567.567').format());
System.assertEquals('-1.234.567,567', Double.valueOf('-1234567.567').format());
`,
			currentUser: storage.Record{
				ID:     "005-double-de-locale",
				Object: "User",
				Fields: map[string]storage.Value{
					"LocaleSidKey":      storage.StringValue("de_DE"),
					"LanguageLocaleKey": storage.StringValue("en_US"),
				},
			},
			setCurrentUser: true,
		},
		{
			name: "nestedRunAsRestoresContextLocales",
			source: `
System.assertEquals('en_US', UserInfo.getLocale());
System.assertEquals('de', UserInfo.getLanguage());
System.assertEquals('1,234,567.567', Double.valueOf('1234567.567').format());
System.assertEquals('-1,234,567.567', Double.valueOf('-1234567.567').format());
System.runAs(new User(Id = '005-double-runas-de', LocaleSidKey = 'de_DE', LanguageLocaleKey = 'en_US')) {
    System.assertEquals('de_DE', UserInfo.getLocale());
    System.assertEquals('en_US', UserInfo.getLanguage());
    System.assertEquals('1.234.567,567', Double.valueOf('1234567.567').format());
    System.assertEquals('-1.234.567,567', Double.valueOf('-1234567.567').format());
    System.runAs(new User(Id = '005-double-runas-en', LocaleSidKey = 'en_US', LanguageLocaleKey = 'de')) {
        System.assertEquals('en_US', UserInfo.getLocale());
        System.assertEquals('de', UserInfo.getLanguage());
        System.assertEquals('1,234,567.567', Double.valueOf('1234567.567').format());
        System.assertEquals('-1,234,567.567', Double.valueOf('-1234567.567').format());
    }
    System.assertEquals('de_DE', UserInfo.getLocale());
    System.assertEquals('en_US', UserInfo.getLanguage());
    System.assertEquals('1.234.567,567', Double.valueOf('1234567.567').format());
    System.assertEquals('-1.234.567,567', Double.valueOf('-1234567.567').format());
}
System.assertEquals('en_US', UserInfo.getLocale());
System.assertEquals('de', UserInfo.getLanguage());
System.assertEquals('1,234,567.567', Double.valueOf('1234567.567').format());
System.assertEquals('-1,234,567.567', Double.valueOf('-1234567.567').format());
`,
			currentUser: storage.Record{
				ID:     "005-double-outer-en",
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
