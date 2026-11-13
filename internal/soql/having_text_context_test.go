package soql

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
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
				} {
					if got := len(collationQuery(t, org, locale, query).Records); got != want {
						t.Errorf("%s: got %d rows, WHERE got %d", query, got, want)
					}
				}

				// Scalar HAVING is rejected by the public query validator.
				// Exercise the scalar aggregate evaluator directly, without
				// widening the accepted SOQL surface.
				scalar, err := Parse("SELECT MIN(Name) mn FROM Account GROUP BY BillingCity HAVING MIN(Name) " + op + " '9'")
				if err != nil {
					t.Fatal(err)
				}
				var records []storage.Record
				for _, record := range org.Objects["Account"].Records {
					records = append(records, record)
				}
				scalar.GroupBy = nil
				scalar = withTextCollation(scalar, englishTextLocale(locale))
				got, err := aggregateRecords(org, org.Objects["Account"].Definition, records, scalar)
				if err != nil {
					t.Fatal(err)
				}
				if len(got) != want {
					t.Errorf("scalar evaluator: got %d rows, WHERE got %d", len(got), want)
				}

			})
		}
	}
}
