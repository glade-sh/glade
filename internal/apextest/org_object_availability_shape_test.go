package apextest

import (
	"path/filepath"
	"testing"
)

// No scratch definition is an unspecified target, not the captured plain
// Developer shape. The enabled twin is backed by standard_describe_profiles/
// P10/ORG-RUN.txt and org.tsv (APIs 62/67); its license-only rejection twin is
// retained by TestStandardFieldEvidenceCommerceShapeCertainty in storage.
func TestOrgObjectAvailabilityCommerceDMLProjectRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, shape := range []string{"no-scratch-definition", "commerce-enabled"} {
			t.Run(api+"/"+shape, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
				if shape == "commerce-enabled" {
					writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), `{"edition":"Developer","features":["B2BCommerce","Communities"],"settings":{"commerceSettings":{"commerceEnabled":true},"communitiesSettings":{"enableNetworksEnabled":true}}}`)
				}
				runStandardFieldEvidenceRoutes(t, root, api, `
WebStore store = new WebStore(Name='Owned store');
insert store;
System.assertNotEquals(null, store.Id);
System.assertEquals('Owned store', [SELECT Name FROM WebStore WHERE Id=:store.Id].Name);
System.assertEquals(true, Schema.getGlobalDescribe().containsKey('WebStore'));
System.assertNotEquals(null, Type.forName('Schema.WebStore'));
`)
			})
		}
	}
}
