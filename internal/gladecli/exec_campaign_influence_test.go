package gladecli

import (
	"context"
	"io"
	"path/filepath"
	"testing"
)

func TestRunExecCampaignInfluence(t *testing.T) {
	for _, tc := range []struct {
		name, shape string
		typeOn      bool
	}{
		{"plain", `{"edition":"Enterprise"}`, false},
		{"feature-only", `{"edition":"Enterprise","features":["CampaignInfluence2"]}`, false},
		{"setting-only", `{"edition":"Enterprise","settings":{"campaignSettings":{"enableCampaignInfluence2":true}}}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := execStandardOrgProject(t, "")
			writeTestFile(t, filepath.Join(root, "config/project-scratch-def.json"), tc.shape)
			body := `String n='CampaignInfluence';System.assertEquals(false,Schema.getGlobalDescribe().containsKey(n));System.assertEquals(`
			if tc.typeOn {
				body += "true"
			} else {
				body += "false"
			}
			body += `,Type.forName('Schema.'+n)!=null);Boolean rejected=false;try{Database.countQuery('SELECT COUNT() FROM '+n);}catch(System.QueryException e){rejected=true;}System.assert(rejected);`
			if err := runExec(context.Background(), []string{"--project", root, "--json", body}, io.Discard); err != nil {
				t.Fatal(err)
			}
		})
	}
}
