package apextest

import (
	"fmt"
	"path/filepath"
	"testing"
)

// Native rows: org_quote_family/org.tsv at APIs 62 and 67.
func TestOrgQuoteFamilyAvailabilityProjectRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, enabled := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%t", api, enabled), func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
				shape := `{"edition":"Developer"}`
				if enabled {
					shape = `{"edition":"Developer","settings":{"quoteSettings":{"enableQuote":true}}}`
				}
				writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), shape)
				body := fmt.Sprintf(`for(String n:'QuoteDocument,QuoteFeed,QuoteHistory,QuoteLineItemHistory,QuoteShare'.split(',')){System.assertEquals(%t,Schema.getGlobalDescribe().containsKey(n),n);System.assertEquals(%t,Type.forName('Schema.'+n)!=null,n);Boolean ok=true;try{Database.countQuery('SELECT COUNT() FROM '+n);}catch(System.QueryException e){ok=false;}System.assertEquals(%t,ok,n);}System.assert(Schema.getGlobalDescribe().containsKey('Account'));`, enabled, enabled, enabled)
				runStandardFieldEvidenceRoutes(t, root, api, body)
			})
		}
	}
}
