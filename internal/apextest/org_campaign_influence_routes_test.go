package apextest

import (
	"path/filepath"
	"testing"
)

func TestOrgCampaignInfluenceProjectRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range []struct {
			name, shape string
			typeOn      bool
		}{
			{"plain", `{"edition":"Enterprise"}`, false},
			{"feature-only", `{"edition":"Enterprise","features":["CampaignInfluence2"]}`, false},
			{"setting-only", `{"edition":"Enterprise","settings":{"campaignSettings":{"enableCampaignInfluence2":true}}}`, true},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
				writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), tc.shape)
				wantType := "false"
				if tc.typeOn {
					wantType = "true"
				}
				testQuery := "ok"
				if api == "67.0" {
					testQuery = "System.SecurityException"
				}
				body := `String n='CampaignInfluence';Boolean enabled=` + wantType + `;Boolean isTest=Test.isRunningTest();System.assertEquals(enabled && isTest,Schema.getGlobalDescribe().containsKey(n));System.assertEquals(enabled,Type.forName('Schema.'+n)!=null);String result='ok';try{Database.countQuery('SELECT COUNT() FROM '+n);}catch(Exception e){result=e.getTypeName();}String expected='System.QueryException';if(enabled && isTest){expected='` + testQuery + `';}System.assertEquals(expected,result);System.assert(Schema.getGlobalDescribe().containsKey('Account'));`
				runStandardFieldEvidenceRoutes(t, root, api, body)
			})
		}
	}
}
