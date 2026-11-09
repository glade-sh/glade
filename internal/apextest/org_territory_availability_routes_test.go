package apextest

import (
	"fmt"
	"path/filepath"
	"testing"
)

func TestOrgTerritoryAvailabilityProjectRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range []struct {
			name, shape string
			want        bool
		}{
			{"plain", `{"edition":"Enterprise"}`, false},
			{"enabled", `{"edition":"Enterprise","settings":{"territory2Settings":{"enableTerritoryManagement2":true}}}`, true},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
				writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), tc.shape)
				body := fmt.Sprintf(`for (String n : new List<String>{'Territory2','Territory2Model','Territory2Type','UserTerritory2Association','ObjectTerritory2Association'}) { System.assertEquals(%t, Schema.getGlobalDescribe().containsKey(n), n); System.assertEquals(%t, Type.forName('Schema.'+n)!=null, n); Boolean ok=true; try { Database.countQuery('SELECT COUNT() FROM '+n); } catch (System.QueryException e) { ok=false; } System.assertEquals(%t,ok,n); } System.assert(Schema.getGlobalDescribe().containsKey('Account'));`, tc.want, tc.want, tc.want)
				runStandardFieldEvidenceRoutes(t, root, api, body)
			})
		}
	}
}
