package gladecli

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"testing"
)

// Covers the CLI entrypoint at both project API versions. Source-only fallback
// has a separate test; these use explicitly declared off and on profiles.
func TestRunExecAvailabilityAPIRoutes(t *testing.T) {
	names := "CurrencyType,DatedConversionRate,Territory2,Territory2Model,Territory2Type,UserTerritory2Association,ObjectTerritory2Association,AccountTeamMember,ForecastingItem,ForecastingQuota,ForecastingType,OpportunityTeamMember,OpportunitySplit,ServiceChannel,Survey"
	for _, api := range []string{"62.0", "67.0"} {
		for _, enabled := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%t", api, enabled), func(t *testing.T) {
				root := t.TempDir()
				writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
				features := "[]"
				if enabled {
					features = "[MultiCurrency, Territory2, AccountTeams, Forecasts, OpportunityTeam, OpportunitySplits, OmniChannel, Surveys]"
				}
				writeTestFile(t, filepath.Join(root, "glade.yml"), "org:\n  features: "+features+"\n")
				body := fmt.Sprintf(`for(String n : '%s'.split(',')){System.assertEquals(%t,Schema.getGlobalDescribe().containsKey(n),n);Boolean ok=true;try{Database.countQuery('SELECT COUNT() FROM '+n);}catch(System.QueryException e){ok=false;}System.assertEquals(%t,ok,n);}`, names, enabled, enabled)
				if err := runExec(context.Background(), []string{"--project", root, "--json", body}, io.Discard); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
