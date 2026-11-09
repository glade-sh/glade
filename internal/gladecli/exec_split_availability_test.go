package gladecli

import (
	"context"
	"fmt"
	"io"
	"testing"
)

func TestRunExecSplitAvailability(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprint(enabled), func(t *testing.T) {
			config := "org:\n  features: [OpportunityTeam]\n"
			if enabled {
				config = "org:\n  features: [OpportunityTeam, OpportunitySplits]\n"
			}
			root := execStandardOrgProject(t, config)
			body := fmt.Sprintf(`String n='OpportunitySplit';System.assertEquals(%t,Schema.getGlobalDescribe().containsKey(n));Boolean ok=true;try{Database.countQuery('SELECT COUNT() FROM '+n);}catch(System.QueryException e){ok=false;}System.assertEquals(%t,ok);`, enabled, enabled)
			if err := runExec(context.Background(), []string{"--project", root, "--json", body}, io.Discard); err != nil {
				t.Fatal(err)
			}
		})
	}
}
