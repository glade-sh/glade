package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
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
			org := storage.NewOrgState()
			org.OrgID = storage.DefaultOrgID
			storage.EnsureDeterministicPlatformData(&org)
			features := []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, project.PlainDeveloperDescribeShape}
			if tc.name == "optional" {
				features = []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, "EnableSetPasswordInApi"}
			}
			storage.ApplyProjectOrgShape(&org, features)
			org.EnableLazyStandardSpread()
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

func TestPlainDeveloperSpreadPreservesVirtualAccessAfterSerialization(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"68.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	index := loadTestIndex(t, root)
	org := storage.NewOrgState()
	org.OrgID = storage.DefaultOrgID
	storage.EnsureDeterministicPlatformData(&org)
	storage.ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, project.PlainDeveloperDescribeShape})
	org.EnableLazyStandardSpread()
	storage.MaterializeStandardObjectSpread(&org)
	data, err := json.Marshal(org)
	if err != nil {
		t.Fatal(err)
	}
	var restored storage.OrgState
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &restored})
	const source = `List<String> ns=new List<String>{'AIFeatureExtractor','AIResearchPromptResult','AIScoringStatusEvent'}; List<String> tables=new List<String>{'UserEntityAccess','UserFieldAccess'}; for(String tableName:tables){ for(String n:ns){ String q='SELECT count() FROM '+tableName+' WHERE EntityDefinitionId = \''+n+'\''; System.assert(Database.countQuery(q)>0,tableName+':'+n); } }`
	program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: "68.0"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runner.newMachine().Execute(program); err != nil {
		t.Fatalf("origin/main serialized virtual-access outcome changed: %v", err)
	}
}

// These source-backed virtual rows are a local origin/main contract. Hydrating
// an object for a different route must not expose it through an unproven view.
func TestSourceSpreadPreservesVirtualNeighbor(t *testing.T) {
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
			const source = `List<String> tables=new List<String>{'EntityDefinition','FieldDefinition','EntityParticle','RelationshipDomain','UserEntityAccess','UserFieldAccess'}; List<String> fields=new List<String>{'QualifiedApiName','EntityDefinitionId','EntityDefinitionId','ChildSobjectId','EntityDefinitionId','EntityDefinitionId'}; for(Integer i=0;i<tables.size();i++){String q='SELECT count() FROM '+tables[i]+' WHERE '+fields[i]+' = \'CareProgram\''; System.assertEquals(0,Database.countQuery(q),tables[i]);}`
			program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: "68.0"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := runner.newMachine().Execute(program); err != nil {
				t.Fatalf("origin/main virtual neighbor outcome changed: %v", err)
			}
		})
	}
}
