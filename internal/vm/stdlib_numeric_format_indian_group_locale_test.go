package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The retained Summer '26 API 67.0 catalog SHA-256 is
// 9d96c8f2472acb64d9d2860ab22ebb1e526edccf137b659c2bca441b1ce86a7b.
// Decimal.format (/documents/2197/members/12), Integer.format
// (/documents/2203/members/0), and Long.format (/documents/2206/members/0)
// use the context user's locale. The official ICU number-format table lists
// en_IN, gu_IN, hi_IN, ml_IN, pa_IN, ta_IN, ta_LK, and te_IN with the exact
// positive/negative Decimal examples 12,34,567.567 and -12,34,567.567:
// https://help.salesforce.com/s/articleView?id=admin_supported_locales.htm&language=en_US&type=5
// Salesforce specifies ICU 71.1 / CLDR 41 and API 45.0+ for these formats:
// https://help.salesforce.com/s/articleView?id=xcloud.admin_locales_icu.htm&language=en_US&type=5
// Integral expectations infer only the same grouping and ASCII minus sign;
// they make no fractional, scale, precision, or rounding claim. This test is
// limited to the exact locale keys and values above at API 67.0.
func TestExecNumericFormatIndianGroupingLocalesAPI67(t *testing.T) {
	numberTypes := []struct {
		name         string
		declarations string
		indianPos    string
		indianNeg    string
		usPos        string
		usNeg        string
	}{
		{
			name: "Decimal",
			declarations: `
Decimal positive = Decimal.valueOf('1234567.567');
Decimal negative = Decimal.valueOf('-1234567.567');
`,
			indianPos: "12,34,567.567", indianNeg: "-12,34,567.567",
			usPos: "1,234,567.567", usNeg: "-1,234,567.567",
		},
		{
			name: "Integer",
			declarations: `
Integer positive = 1234567;
Integer negative = -1234567;
`,
			indianPos: "12,34,567", indianNeg: "-12,34,567",
			usPos: "1,234,567", usNeg: "-1,234,567",
		},
		{
			name: "Long",
			declarations: `
Long positive = 1234567L;
Long negative = -1234567L;
`,
			indianPos: "12,34,567", indianNeg: "-12,34,567",
			usPos: "1,234,567", usNeg: "-1,234,567",
		},
	}
	locales := []string{"en_IN", "gu_IN", "hi_IN", "ml_IN", "pa_IN", "ta_IN", "ta_LK", "te_IN"}
	for _, numberType := range numberTypes {
		for _, locale := range locales {
			cases := []struct {
				name              string
				source            string
				currentLocale     string
				currentLanguage   string
				enableTestContext bool
			}{
				{
					name: "localeWinsOverEnglishLanguage",
					source: fmt.Sprintf(`
System.assertEquals('%s', UserInfo.getLocale());
System.assertEquals('en_US', UserInfo.getLanguage());
System.assertEquals('%s', positive.format());
System.assertEquals('%s', negative.format());
`, locale, numberType.indianPos, numberType.indianNeg),
					currentLocale: locale, currentLanguage: "en_US",
				},
				{
					name: "runAsUsesLocale",
					source: fmt.Sprintf(`
System.assertEquals('en_US', UserInfo.getLocale());
System.assertEquals('de', UserInfo.getLanguage());
System.runAs(new User(Id = '005-indian-inner-%s', LocaleSidKey = '%s', LanguageLocaleKey = 'en_US')) {
    System.assertEquals('%s', UserInfo.getLocale());
    System.assertEquals('en_US', UserInfo.getLanguage());
    System.assertEquals('%s', positive.format());
    System.assertEquals('%s', negative.format());
}
`, locale, locale, locale, numberType.indianPos, numberType.indianNeg),
					currentLocale: "en_US", currentLanguage: "de", enableTestContext: true,
				},
				{
					name: "runAsRestoresOuterUS",
					source: fmt.Sprintf(`
System.assertEquals('en_US', UserInfo.getLocale());
System.assertEquals('de', UserInfo.getLanguage());
System.runAs(new User(Id = '005-indian-restore-%s', LocaleSidKey = '%s', LanguageLocaleKey = 'en_US')) {
    System.assertEquals('%s', UserInfo.getLocale());
    System.assertEquals('en_US', UserInfo.getLanguage());
}
System.assertEquals('en_US', UserInfo.getLocale());
System.assertEquals('de', UserInfo.getLanguage());
System.assertEquals('%s', positive.format());
System.assertEquals('%s', negative.format());
`, locale, locale, locale, numberType.usPos, numberType.usNeg),
					currentLocale: "en_US", currentLanguage: "de", enableTestContext: true,
				},
			}
			for _, tc := range cases {
				t.Run(numberType.name+"/"+locale+"/"+tc.name, func(t *testing.T) {
					program, err := CompileAnonymousWithOptions(numberType.declarations+tc.source, CompileOptions{APIVersion: "67.0"})
					if err != nil {
						t.Fatal(err)
					}
					if program.APIVersion != "67.0" {
						t.Fatalf("compiled API version = %q, want 67.0", program.APIVersion)
					}
					machine := New(nil)
					machine.SetCurrentUser(storage.Record{
						ID: storage.ID("005-indian-outer-" + locale), Object: "User",
						Fields: map[string]storage.Value{
							"LocaleSidKey":      storage.StringValue(tc.currentLocale),
							"LanguageLocaleKey": storage.StringValue(tc.currentLanguage),
						},
					})
					if tc.enableTestContext {
						machine.EnableTestContext()
					}
					if _, err := machine.Execute(program); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}
