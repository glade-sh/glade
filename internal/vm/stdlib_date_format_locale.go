package vm

import (
	"fmt"
	"time"
)

// formatApexDateForLocale covers the explicit API 67.0 en_US/de_DE profiles,
// the en_GB/fr_FR day-first slash group, and the ja_JP/zh_CN/sv_SE year-first
// group exercised by Date.format(). The retained Apex Date member says
// format() uses the context user's locale; Salesforce's supported ICU
// date/time table supplies the examples whose date portions this function
// renders. For 2008-01-28, the table shows en_GB 28/01/2008, 16:30 and fr_FR
// 28/01/2008 16:30; ja_JP 2008/01/28 16:30; zh_CN 2008/1/28 16:30; and sv_SE
// 2008-01-28 16:30. Extracting each date-only result from its date/time row is
// a bounded inference. These rows qualify only the named locale codes and
// this input, not other dates, locale keys, or Salesforce parity generally.
// Source profile: API 67.0 Summer '26, catalog
// 9d96c8f2472acb64d9d2860ab22ebb1e526edccf137b659c2bca441b1ce86a7b,
// apex/apex_methods_system_date.md /documents/2195/members/7; ICU 71.1 / CLDR
// 41 at https://help.salesforce.com/s/articleView?id=sf.admin_locales_icu.htm&language=en_US&type=5.
// Supported date/time table: https://help.salesforce.com/s/articleView?id=xcloud.admin_supported_date_time_format.htm&language=en_US&type=5.
func formatApexDateForLocale(date time.Time, locale string) string {
	switch locale {
	case "de_DE":
		return date.Format("02.01.2006")
	case "en_GB", "fr_FR":
		return date.Format("02/01/2006")
	case "ja_JP":
		return date.Format("2006/01/02")
	case "zh_CN":
		return fmt.Sprintf("%d/%d/%d", date.Year(), int(date.Month()), date.Day())
	case "sv_SE":
		return date.Format("2006-01-02")
	default:
		// Preserve the historical output for all profiles not qualified here.
		return fmt.Sprintf("%d/%d/%d", int(date.Month()), date.Day(), date.Year())
	}
}
