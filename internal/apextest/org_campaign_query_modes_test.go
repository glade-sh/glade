package apextest

import (
	"fmt"
	"path/filepath"
	"testing"
)

// Native default/user/system dynamic query rows: org_influence_query_modes/org.tsv.
func TestOrgCampaignInfluenceQueryModesProject(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), `{"edition":"Enterprise","settings":{"campaignSettings":{"enableCampaignInfluence2":true}}}`)
			defaultCount, defaultQuery := "ok", "ok"
			if api == "67.0" {
				defaultCount, defaultQuery = "System.SecurityException", "System.QueryException"
			}
			body := fmt.Sprintf(`Map<String,String> got=new Map<String,String>(); for(String route : new List<String>{'queryDefault','queryUser','querySystem','countDefault','countUser','countSystem'}) { try { if(route=='queryDefault') Database.query('SELECT Id FROM CampaignInfluence'); if(route=='queryUser') Database.query('SELECT Id FROM CampaignInfluence',AccessLevel.USER_MODE); if(route=='querySystem') Database.query('SELECT Id FROM CampaignInfluence',AccessLevel.SYSTEM_MODE); if(route=='countDefault') Database.countQuery('SELECT COUNT() FROM CampaignInfluence'); if(route=='countUser') Database.countQuery('SELECT COUNT() FROM CampaignInfluence',AccessLevel.USER_MODE); if(route=='countSystem') Database.countQuery('SELECT COUNT() FROM CampaignInfluence',AccessLevel.SYSTEM_MODE); got.put(route,'ok'); } catch(Exception e) {got.put(route,e.getTypeName());} } System.assertEquals('%s',got.get('countDefault'));System.assertEquals('%s',got.get('queryDefault'));System.assertEquals('System.QueryException',got.get('countUser'));System.assertEquals('System.QueryException',got.get('queryUser'));System.assertEquals('ok',got.get('countSystem'));System.assertEquals('ok',got.get('querySystem'));`, defaultCount, defaultQuery)
			path := filepath.Join(root, "force-app/main/default/classes/InfluenceModesTest.cls")
			writeFile(t, path, "@IsTest private class InfluenceModesTest { @IsTest static void sample(){"+body+"} }")
			writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			index := loadTestIndex(t, root)
			for _, state := range []string{"cold", "warm"} {
				t.Run(state, func(t *testing.T) {
					run := Run(index, Options{SelectedClasses: []string{"InfluenceModesTest"}, SelectedMethod: "sample", NoDiskCache: true, Parallelism: 1})
					if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem != nil {
						t.Fatalf("project runner: %+v", run.Summary())
					}
				})
			}
		})
	}
}
