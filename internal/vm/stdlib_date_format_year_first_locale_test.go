package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// API 67.0 Date.format() returns a string in the context user's locale
// (Summer '26 catalog 9d96c8f2472acb64d9d2860ab22ebb1e526edccf137b659c2bca441b1ce86a7b,
// apex/apex_methods_system_date.md /documents/2195/members/7). Salesforce's
// supported ICU date/time table, using ICU 71.1 / CLDR 41, lists these exact
// rows for 2008-01-28:
//
//	ja_JP: 2008/01/28 16:30
//	zh_CN: 2008/1/28 16:30
//	sv_SE: 2008-01-28 16:30
//
// The Date-only assertions infer their expected values from the date portions
// of those date/time examples. They establish no generalized pattern or
// padding rule. en_US and de_DE are controls; scope is API 67.0 and this one
// input, not other dates or locale versions.
// https://help.salesforce.com/s/articleView?id=xcloud.admin_supported_date_time_format.htm&language=en_US&type=5
// https://help.salesforce.com/s/articleView?id=sf.admin_locales_code_methods.htm&language=en_US&type=5
// https://help.salesforce.com/s/articleView?id=sf.admin_locales_icu.htm&language=en_US&type=5
func TestExecDateFormatUsesContextUserYearFirstLocaleGroupAPI67(t *testing.T) {
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
				ID:     "005-date-format-year-first-en-us",
				Object: "User",
				Fields: map[string]storage.Value{
					"LocaleSidKey":      storage.StringValue("en_US"),
					"LanguageLocaleKey": storage.StringValue("de"),
				},
			},
		},
		{
			name: "deDEControlEnglishLanguage",
			source: `
System.assertEquals('de_DE', UserInfo.getLocale());
System.assertEquals('en_US', UserInfo.getLanguage());
Date sample = Date.newInstance(2008, 1, 28);
System.assertEquals('28.01.2008', sample.format());
`,
			currentUser: storage.Record{
				ID:     "005-date-format-year-first-de-de",
				Object: "User",
				Fields: map[string]storage.Value{
					"LocaleSidKey":      storage.StringValue("de_DE"),
					"LanguageLocaleKey": storage.StringValue("en_US"),
				},
			},
		},
		{
			name: "jaJPLocaleWithEnglishLanguage",
			source: `
System.assertEquals('ja_JP', UserInfo.getLocale());
System.assertEquals('en_US', UserInfo.getLanguage());
Date sample = Date.newInstance(2008, 1, 28);
System.assertEquals('2008/01/28', sample.format());
`,
			currentUser: storage.Record{
				ID:     "005-date-format-year-first-ja-jp",
				Object: "User",
				Fields: map[string]storage.Value{
					"LocaleSidKey":      storage.StringValue("ja_JP"),
					"LanguageLocaleKey": storage.StringValue("en_US"),
				},
			},
		},
		{
			name: "zhCNLocaleWithEnglishLanguage",
			source: `
System.assertEquals('zh_CN', UserInfo.getLocale());
System.assertEquals('en_US', UserInfo.getLanguage());
Date sample = Date.newInstance(2008, 1, 28);
System.assertEquals('2008/1/28', sample.format());
`,
			currentUser: storage.Record{
				ID:     "005-date-format-year-first-zh-cn",
				Object: "User",
				Fields: map[string]storage.Value{
					"LocaleSidKey":      storage.StringValue("zh_CN"),
					"LanguageLocaleKey": storage.StringValue("en_US"),
				},
			},
		},
		{
			name: "svSELocaleWithEnglishLanguage",
			source: `
System.assertEquals('sv_SE', UserInfo.getLocale());
System.assertEquals('en_US', UserInfo.getLanguage());
Date sample = Date.newInstance(2008, 1, 28);
System.assertEquals('2008-01-28', sample.format());
`,
			currentUser: storage.Record{
				ID:     "005-date-format-year-first-sv-se",
				Object: "User",
				Fields: map[string]storage.Value{
					"LocaleSidKey":      storage.StringValue("sv_SE"),
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
