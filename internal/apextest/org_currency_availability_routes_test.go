package apextest

import (
	"fmt"
	"path/filepath"
	"testing"
)

// Plain org_object_availability/org.tsv:2-5 and MultiCurrency
// org_currency_availability/org.tsv:2-3, APIs 62 and 67.
func TestOrgCurrencyAvailabilityProjectRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range []struct {
			name, config string
			available    bool
		}{
			{"plain", "org:\n  features: []\n", false},
			{"communities", "org:\n  features: [Communities]\n", false},
			{"multi", "org:\n  features: [MultiCurrency]\n", true},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
				writeFile(t, filepath.Join(root, "glade.yml"), tc.config)
				body := fmt.Sprintf(`
for (String name : new List<String>{'CurrencyType','DatedConversionRate'}) {
 System.assertEquals(%t, Schema.getGlobalDescribe().containsKey(name), name);
 System.assertEquals(%t, Type.forName('Schema.'+name) != null, name);
 Boolean ok = true;
 try { Database.countQuery('SELECT COUNT() FROM '+name); } catch (System.QueryException e) { ok = false; }
 System.assertEquals(%t, ok, name);
}
System.assert(Schema.getGlobalDescribe().containsKey('Account'));
`, tc.available, tc.available, tc.available)
				runStandardFieldEvidenceRoutes(t, root, api, body)
			})
		}
	}
}
