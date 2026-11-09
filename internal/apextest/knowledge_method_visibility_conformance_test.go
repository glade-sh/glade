package apextest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Each source is the call shape observed in the Knowledge-enabled org.
func TestKnowledgeB4MethodVisibility(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for name, body := range map[string]string{
			"edit":   `Id parent=null; Id edited=KbManagement.PublishingService.editOnlineArticle(parent,false);`,
			"assign": `Id parent=null; KbManagement.PublishingService.assignDraftArticleTask(parent,UserInfo.getUserId(),'Synthetic task',Datetime.now().addDays(1),true);`,
		} {
			for _, route := range []string{"anonymous", "isTest"} {
				t.Run(api+"/"+name+"/"+route, func(t *testing.T) {
					root := t.TempDir()
					data, err := os.ReadFile(filepath.Join("testdata", "conformance", "knowledge_publishing", "project", "config", "project-scratch-def.json"))
					if err != nil {
						t.Fatal(err)
					}
					writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), string(data))
					p := project.Project{Root: root, SourceAPIVersion: api}
					if route == "isTest" {
						path := filepath.Join(root, "SyntheticMethod.cls")
						writeFile(t, path, "@IsTest private class SyntheticMethod { @IsTest static void observe() { "+body+" } }")
						writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
						p.ApexFiles = []string{path}
					}
					index := typesys.Build(p, gladeschema.Schema{})
					if index.HasErrors() {
						t.Fatal(index.Diagnostics)
					}
					var analysis sema.Result
					if route == "anonymous" {
						analysis = sema.AnalyzeAnonymous(index, body, api)
					} else {
						analysis = sema.Analyze(index)
					}
					if analysis.HasErrors() {
						t.Fatalf("native compiled: %v", analysis.Diagnostics)
					}
				})
			}
		}
	}
}
