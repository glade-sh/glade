package apextest

import (
	"fmt"
	"path/filepath"
	"testing"
)

func TestOrgSplitAvailabilityProjectRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, enabled := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%t", api, enabled), func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
				config := "org:\n  features: [OpportunityTeam]\n"
				if enabled {
					config = "org:\n  features: [OpportunityTeam, OpportunitySplits]\n"
				}
				writeFile(t, filepath.Join(root, "glade.yml"), config)
				body := fmt.Sprintf(`String n='OpportunitySplit';System.assertEquals(%t,Schema.getGlobalDescribe().containsKey(n));System.assertEquals(%t,Type.forName('Schema.'+n)!=null);Boolean ok=true;try{Database.countQuery('SELECT COUNT() FROM '+n);}catch(System.QueryException e){ok=false;}System.assertEquals(%t,ok);System.assert(Schema.getGlobalDescribe().containsKey('Account'));`, enabled, enabled, enabled)
				runStandardFieldEvidenceRoutes(t, root, api, body)
			})
		}
	}
}
