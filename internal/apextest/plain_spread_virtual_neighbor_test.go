package apextest

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// The captured Salesforce queries throw for these user-access tables, so the
// current local result remains the contract until a row proves a change.
func TestPlainDeveloperSpreadPreservesUnprobedVirtualAccess(t *testing.T) {
	for _, tc := range []struct{ name, scratch string }{
		{"default", `{"edition":"Developer","features":[],"settings":{}}`},
		{"optional", `{"edition":"Developer","features":["EnableSetPasswordInApi"],"settings":{}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"68.0","packageDirectories":[{"path":"force-app","default":true}]}`)
			writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), tc.scratch)
			index := loadTestIndex(t, root)
			org := orgFromIndex(index)
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			const source = `List<String> ns=new List<String>{'AIFeatureExtractor','AIResearchPromptResult','AIScoringStatusEvent'}; List<String> tables=new List<String>{'UserEntityAccess','UserFieldAccess'}; for(String tableName:tables){ for(String n:ns){ String q='SELECT count() FROM '+tableName+' WHERE EntityDefinitionId = \''+n+'\''; System.assert(Database.countQuery(q)>0,tableName+':'+n); } }`
			program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: "68.0"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := runner.newMachine().Execute(program); err != nil {
				t.Fatalf("origin/main virtual-access outcome changed: %v", err)
			}
		})
	}
}
