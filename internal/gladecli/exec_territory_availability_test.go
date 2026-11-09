package gladecli

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"testing"
)

func TestRunExecTerritoryAvailability(t *testing.T) {
	for _, tc := range []struct {
		name, shape string
		want        bool
	}{
		{"plain", `{"edition":"Enterprise"}`, false},
		{"enabled", `{"edition":"Enterprise","settings":{"territory2Settings":{"enableTerritoryManagement2":true}}}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := execStandardOrgProject(t, "")
			writeTestFile(t, filepath.Join(root, "config/project-scratch-def.json"), tc.shape)
			body := fmt.Sprintf(`for (String n : new List<String>{'Territory2','Territory2Model','Territory2Type','UserTerritory2Association','ObjectTerritory2Association'}) { System.assertEquals(%t, Schema.getGlobalDescribe().containsKey(n), n); Boolean ok=true; try { Database.countQuery('SELECT COUNT() FROM '+n); } catch (System.QueryException e) { ok=false; } System.assertEquals(%t,ok,n); }`, tc.want, tc.want)
			if err := runExec(context.Background(), []string{"--project", root, "--json", body}, io.Discard); err != nil {
				t.Fatal(err)
			}
		})
	}
}
