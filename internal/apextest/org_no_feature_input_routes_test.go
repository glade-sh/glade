package apextest

import (
	"path/filepath"
	"testing"
)

// Unspecified source-only projects retain Glade's compatibility fallback; the
// plain Developer rows in org_object_availability/org.tsv:2-5 are distinct.
func TestOrgNoFeatureInputAvailabilityProjectRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			runStandardFieldEvidenceRoutes(t, root, api, `for(String n : new List<String>{'Quote','QuoteLineItem','AccountContactRelation','WebStore'}) {System.assert(Schema.getGlobalDescribe().containsKey(n),n);System.assertNotEquals(null,Type.forName('Schema.'+n),n);Database.countQuery('SELECT COUNT() FROM '+n);}`)
		})
	}
}
