package apextest

import (
	"path/filepath"
	"testing"
)

func TestOrgCampaignInfluenceOverloadRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range []struct{ name, shape, want string }{
			{"off", `{"edition":"Enterprise"}`, "false"},
			{"on", `{"edition":"Enterprise","settings":{"campaignSettings":{"enableCampaignInfluence2":true}}}`, "true"},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
				writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), tc.shape)
				body := `System.assertEquals(` + tc.want + `,Type.forName('Schema.CampaignInfluence')!=null);System.assertEquals(` + tc.want + `,Type.forName('CampaignInfluence')!=null);System.assertEquals(` + tc.want + `,Type.forName(null,'Schema.CampaignInfluence')!=null);System.assertEquals(` + tc.want + `,Type.forName('','Schema.CampaignInfluence')!=null);System.assertEquals(null,Type.forName('Schema','CampaignInfluence'));System.assertNotEquals(null,Type.forName('Schema.Account'));System.assertEquals(null,Type.forName('Schema.AbsentObject__c'));`
				runStandardFieldEvidenceRoutes(t, root, api, body)
			})
		}
	}
}
