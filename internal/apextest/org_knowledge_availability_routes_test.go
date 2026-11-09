package apextest

import (
	"fmt"
	"path/filepath"
	"testing"
)

// Native anonymous rows and named rows are in org_knowledge_availability.
func TestOrgKnowledgeAvailabilityProjectRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range []struct {
			name, shape string
			enabled     bool
		}{
			{"plain", `{"edition":"Developer"}`, false},
			{"feature-only", `{"edition":"Developer","features":["Knowledge"]}`, false},
			{"setting-only", `{"edition":"Developer","settings":{"knowledgeSettings":{"enableKnowledge":true}}}`, true},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
				writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), tc.shape)
				body := fmt.Sprintf(`for (String n : 'Knowledge__ka,Knowledge__kav'.split(',')) { System.assertEquals(%t,Schema.getGlobalDescribe().containsKey(n),n); System.assertEquals(%t,Type.forName('Schema.'+n)!=null,n); Boolean ok=true; try { Database.countQuery('SELECT COUNT() FROM '+n); } catch(System.QueryException e) {ok=false;} System.assertEquals(%t,ok,n); } System.assert(Schema.getGlobalDescribe().containsKey('Account'));`, tc.enabled, tc.enabled, tc.enabled)
				runStandardFieldEvidenceRoutes(t, root, api, body)
			})
		}
	}
}
