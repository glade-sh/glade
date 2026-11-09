package gladecli

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"testing"
)

func TestRunExecSetupObjectAvailability(t *testing.T) {
	for _, tc := range []struct {
		name, shape string
		want        bool
	}{
		{"plain", `{"edition":"Enterprise"}`, false},
		{"enabled", `{"edition":"Enterprise","settings":{"accountSettings":{"enableAccountTeams":true},"forecastingSettings":{"enableForecasts":true},"opportunitySettings":{"enableOpportunityTeam":true},"omniChannelSettings":{"enableOmniChannel":true},"surveySettings":{"enableSurvey":true}}}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := execStandardOrgProject(t, "")
			writeTestFile(t, filepath.Join(root, "config/project-scratch-def.json"), tc.shape)
			body := fmt.Sprintf(`for (String n : new List<String>{'AccountTeamMember','ForecastingItem','ForecastingQuota','ForecastingType','OpportunityTeamMember','ServiceChannel','Survey'}) {System.assertEquals(%t, Schema.getGlobalDescribe().containsKey(n), n); Boolean ok=true; try {Database.countQuery('SELECT COUNT() FROM '+n);} catch (System.QueryException e) {ok=false;} System.assertEquals(%t,ok,n);}`, tc.want, tc.want)
			if err := runExec(context.Background(), []string{"--project", root, "--json", body}, io.Discard); err != nil {
				t.Fatal(err)
			}
		})
	}
}
