package apextest

import (
	"fmt"
	"path/filepath"
	"testing"
)

func TestOrgSetupObjectAvailabilityProjectRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range []struct{ name, setting, object string }{
			{"accountTeams", `"accountSettings":{"enableAccountTeams":true}`, "AccountTeamMember"},
			{"forecasting", `"forecastingSettings":{"enableForecasts":true}`, "ForecastingItem,ForecastingQuota,ForecastingType"},
			{"opportunityTeam", `"opportunitySettings":{"enableOpportunityTeam":true}`, "OpportunityTeamMember"},
			{"omnichannel", `"omniChannelSettings":{"enableOmniChannel":true}`, "ServiceChannel"},
			{"survey", `"surveySettings":{"enableSurvey":true}`, "Survey"},
		} {
			for _, enabled := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/%t", api, tc.name, enabled), func(t *testing.T) {
					root := t.TempDir()
					writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
					shape := `{"edition":"Enterprise"}`
					if enabled {
						shape = `{"edition":"Enterprise","settings":{` + tc.setting + `}}`
					}
					writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), shape)
					body := fmt.Sprintf(`for (String n : '%s'.split(',')) { System.assertEquals(%t, Schema.getGlobalDescribe().containsKey(n), n); System.assertEquals(%t, Type.forName('Schema.'+n)!=null, n); Boolean ok=true; try { Database.countQuery('SELECT COUNT() FROM '+n); } catch (System.QueryException e) {ok=false;} System.assertEquals(%t,ok,n); } System.assert(Schema.getGlobalDescribe().containsKey('Account'));`, tc.object, enabled, enabled, enabled)
					runStandardFieldEvidenceRoutes(t, root, api, body)
				})
			}
		}
	}
}
