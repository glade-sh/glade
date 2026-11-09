package apextest

import (
	"fmt"
	"path/filepath"
	"testing"
)

// Native rows: org_commerce_family/org.tsv, small/org.tsv, org_commerce_version/org.tsv.
func TestOrgCommerceAvailabilityProjectRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range []struct {
			name, shape      string
			feature, setting bool
		}{
			{"plain", `{"edition":"Developer"}`, false, false},
			{"feature-only", `{"edition":"Developer","features":["OrderManagement"]}`, true, false},
			{"setting-only", `{"edition":"Developer","settings":{"orderManagementSettings":{"enableOrderManagement":true}}}`, false, true},
			{"enabled", `{"edition":"Developer","features":["OrderManagement"],"settings":{"orderManagementSettings":{"enableOrderManagement":true}}}`, true, true},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
				writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), tc.shape)
				body := fmt.Sprintf(`Map<String,Boolean> expected=new Map<String,Boolean>{'BuyerGroup'=>%t,'CartItem'=>%t,'CartTax'=>%t,'PickTicket'=>%t,'PickTicketProduct'=>%t,'UserLocationAssignment'=>%t,'PaymentCredit'=>%t,'WebCartCredit'=>%t};for(String n:expected.keySet()){Boolean want=expected.get(n);System.assertEquals(want,Schema.getGlobalDescribe().containsKey(n),n);System.assertEquals(want,Type.forName('Schema.'+n)!=null,n);Boolean ok=true;try{Database.countQuery('SELECT COUNT() FROM '+n);}catch(System.QueryException e){ok=false;}System.assertEquals(want,ok,n);}System.assert(Schema.getGlobalDescribe().containsKey('Account'));`, tc.feature, tc.feature, tc.feature, tc.feature && tc.setting, tc.feature && tc.setting, tc.feature && tc.setting, tc.feature && api == "67.0", tc.feature && api == "67.0")
				runStandardFieldEvidenceRoutes(t, root, api, body)
			})
		}
	}
}

// Additional project routes for the captured alternate B2B and network shapes.
func TestOrgB2BAndCommunityAvailabilityProjectRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range []struct{ name, shape, body string }{
			{"b2b", `{"edition":"Developer","features":["B2BCommerce","Communities"],"settings":{"communitiesSettings":{"enableNetworksEnabled":true},"commerceSettings":{"commerceEnabled":true}}}`, fmt.Sprintf(`Map<String,Boolean> expected=new Map<String,Boolean>{'BuyerGroup'=>true,'CartItem'=>true,'CartTax'=>true,'PickTicket'=>false,'PaymentCredit'=>false,'WebCartCredit'=>%t};for(String n:expected.keySet()){Boolean want=expected.get(n);System.assertEquals(want,Schema.getGlobalDescribe().containsKey(n),n);System.assertEquals(want,Type.forName('Schema.'+n)!=null,n);}`, api == "67.0")},
			{"community", `{"edition":"Developer","settings":{"communitiesSettings":{"enableNetworksEnabled":true}}}`, `for(String n:new List<String>{'AccountBrand','DelegatedAccount','NetworkActivityAudit','NetworkUserHistoryRecent'}){System.assert(Schema.getGlobalDescribe().containsKey(n),n);System.assert(Type.forName('Schema.'+n)!=null,n);}`},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
				writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), tc.shape)
				runStandardFieldEvidenceRoutes(t, root, api, tc.body)
			})
		}
	}
}
