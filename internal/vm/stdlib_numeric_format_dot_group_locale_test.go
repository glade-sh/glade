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
// use the context user's locale. The official ICU number-format table gives
// it_IT, es_ES, and pt_BR the exact positive/negative examples
// 1.234.567,567 and -1.234.567,567:
// https://help.salesforce.com/s/articleView?id=xcloud.admin_supported_locales.htm&language=en_US&type=5
// Integral expectations infer only the same grouping and ASCII minus sign;
// they make no fractional, scale, precision, or rounding claim.
// The ICU 71.1 / CLDR 41 policy is specified at
// https://help.salesforce.com/s/articleView?id=sf.admin_locales_icu.htm&language=en_US&type=5 .
// This regression is limited to those three exact locale keys at API 67.0.
func TestExecNumericFormatDotGroupingLocalesAPI67(t *testing.T) {
	numberTypes := []struct {
		name         string
		declarations string
		dotPositive  string
		dotNegative  string
		usPositive   string
		usNegative   string
	}{
		{
			name: "Decimal",
			declarations: `
Decimal positive = Decimal.valueOf('1234567.567');
Decimal negative = Decimal.valueOf('-1234567.567');
`,
			dotPositive: "1.234.567,567", dotNegative: "-1.234.567,567",
			usPositive: "1,234,567.567", usNegative: "-1,234,567.567",
		},
		{
			name: "Integer",
			declarations: `
Integer positive = 1234567;
Integer negative = -1234567;
`,
			dotPositive: "1.234.567", dotNegative: "-1.234.567",
			usPositive: "1,234,567", usNegative: "-1,234,567",
		},
		{
			name: "Long",
			declarations: `
Long positive = 1234567L;
Long negative = -1234567L;
`,
			dotPositive: "1.234.567", dotNegative: "-1.234.567",
			usPositive: "1,234,567", usNegative: "-1,234,567",
		},
	}
	for _, numberType := range numberTypes {
		for _, locale := range []string{"it_IT", "es_ES", "pt_BR"} {
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
`, locale, numberType.dotPositive, numberType.dotNegative),
					currentLocale: locale, currentLanguage: "en_US",
				},
				{
					name: "runAsUsesLocale",
					source: fmt.Sprintf(`
System.assertEquals('en_US', UserInfo.getLocale());
System.assertEquals('de', UserInfo.getLanguage());
System.runAs(new User(Id = '005-group-inner-%s', LocaleSidKey = '%s', LanguageLocaleKey = 'en_US')) {
    System.assertEquals('%s', UserInfo.getLocale());
    System.assertEquals('en_US', UserInfo.getLanguage());
    System.assertEquals('%s', positive.format());
    System.assertEquals('%s', negative.format());
}
`, locale, locale, locale, numberType.dotPositive, numberType.dotNegative),
					currentLocale: "en_US", currentLanguage: "de", enableTestContext: true,
				},
				{
					name: "runAsRestoresOuterUS",
					source: fmt.Sprintf(`
System.assertEquals('en_US', UserInfo.getLocale());
System.assertEquals('de', UserInfo.getLanguage());
System.runAs(new User(Id = '005-group-restore-%s', LocaleSidKey = '%s', LanguageLocaleKey = 'en_US')) {
    System.assertEquals('%s', UserInfo.getLocale());
    System.assertEquals('en_US', UserInfo.getLanguage());
}
System.assertEquals('en_US', UserInfo.getLocale());
System.assertEquals('de', UserInfo.getLanguage());
System.assertEquals('%s', positive.format());
System.assertEquals('%s', negative.format());
`, locale, locale, locale, numberType.usPositive, numberType.usNegative),
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
						ID: storage.ID("005-group-outer-" + locale), Object: "User",
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
