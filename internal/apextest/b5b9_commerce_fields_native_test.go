package apextest

import (
	"path/filepath"
	"testing"
)

// P10/enablement-differences.tsv:239-240,544-545 records both fields
// absent with licenses alone and present after Commerce enablement at 62/67.
// b5b9_probe/org.tsv:20,46 independently captures ProductSellingModelId.
func TestB5B9CommercePricebookNativeRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, enabled := range []bool{false, true} {
			label, setting, want := "license-only", "false", "false"
			if enabled {
				label, setting, want = "enabled", "true", "true"
			}
			t.Run(api+"/"+label, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
				writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), `{"edition":"Developer","features":["B2BCommerce","Communities"],"settings":{"communitiesSettings":{"enableNetworksEnabled":true},"commerceSettings":{"commerceEnabled":`+setting+`}}}`)
				body := `Map<String,Schema.SObjectField> f=PricebookEntry.SObjectType.getDescribe().fields.getMap();` +
					`System.assertEquals(` + want + `,f.containsKey('ProductSellingModelId'));` +
					`System.assertEquals(` + want + `,f.containsKey('ActivePriceAdjustmentQuantity'));`
				runStandardFieldEvidenceRoutes(t, root, api, body)
			})
		}
	}
}
