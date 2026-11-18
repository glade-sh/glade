package soql

import (
	"os"
	"strings"
	"testing"
	"time"
)

// The same file and frozen values can be applied to the parent as a cost
// control. Foreign-locale outputs differ there; only English is an unchanged
// behavior comparison. No timing budget is imposed by this benchmark.
func BenchmarkLocaleCollationQuery(b *testing.B) {
	path := os.Getenv("GLADE_COLLATION_BENCH_VALUES")
	if path == "" {
		path = "testdata/locale_collation/maps/V-values.tsv"
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		b.Fatal(err)
	}
	var values []string
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
		c := strings.Split(line, "\t")
		if len(c) != 3 {
			b.Fatal("invalid values map")
		}
		values = append(values, c[2])
	}
	org := collationOrg(values, "Account", map[string]bool{"Name": true})
	queries := []struct{ name, text string }{{"order", "SELECT BillingCity FROM Account ORDER BY Name, BillingCity"}, {"range", "SELECT BillingCity FROM Account WHERE Name > 'z'"}, {"group", "SELECT Name, COUNT(Id) c FROM Account GROUP BY Name ORDER BY Name"}, {"extremes", "SELECT MIN(Name) mn, MAX(Name) mx FROM Account"}}
	for _, locale := range []string{"en_US", "de_DE", "sv_SE", "ja_JP", "zh_CN"} {
		b.Run(locale, func(b *testing.B) {
			for _, q := range queries {
				b.Run(q.name, func(b *testing.B) {
					query, err := Parse(q.text)
					if err != nil {
						b.Fatal(err)
					}
					start := time.Now()
					if _, err := ExecuteWithCacheAndContext(org, query, nil, ExecutionContext{Locale: locale}); err != nil {
						b.Fatal(err)
					}
					b.Logf("first execution including any lazy initialization: %s", time.Since(start))
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						if _, err := ExecuteWithCacheAndContext(org, query, nil, ExecutionContext{Locale: locale}); err != nil {
							b.Fatal(err)
						}
					}
				})
			}
		})
	}
}
