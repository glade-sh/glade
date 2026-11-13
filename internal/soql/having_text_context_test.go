package soql

import (
	"fmt"
	"testing"
)

// Local contract: HAVING evaluates text ranges in the same execution context
// as WHERE. The native range rows establish the English order; no native
// HAVING parity is claimed by this route-consistency regression.
func TestHavingTextRangesKeepExecutionLocale(t *testing.T) {
	org := collationOrg([]string{"_"}, "Account", map[string]bool{"Name": true})
	for _, op := range []string{">", "<", ">=", "<="} {
		for _, locale := range []string{"en_US", "en_GB", "", "de_DE", "en_US"} {
			t.Run(fmt.Sprintf("%s/%s", op, locale), func(t *testing.T) {
				want := len(collationQuery(t, org, locale, "SELECT Name FROM Account WHERE Name "+op+" '9'").Records)
				for _, query := range []string{
					"SELECT Name, COUNT(Id) c FROM Account GROUP BY Name HAVING Name " + op + " '9'",
					"SELECT MIN(Name) mn FROM Account HAVING MIN(Name) " + op + " '9'",
				} {
					if got := len(collationQuery(t, org, locale, query).Records); got != want {
						t.Errorf("%s: got %d rows, WHERE got %d", query, got, want)
					}
				}
			})
		}
	}
}
