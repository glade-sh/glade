package sema

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// platform_batch_superclass/org.tsv:4,11 reject a second Batchable only below
// the platform UserProvisioning.PluginBatchable. No row covers a project
// superclass, so a project hierarchy with two Batchable parameterizations
// keeps its earlier diagnostics on the named and anonymous routes.
func TestBatchableProjectHierarchySecondGenericKeepsBaseDiagnostics(t *testing.T) {
	files := map[string]string{
		"TwoGen.cls":    "public virtual class TwoGen implements Database.Batchable<SObject> { public virtual Database.QueryLocator start(Database.BatchableContext context) { return Database.getQueryLocator('SELECT Id FROM Account'); } public virtual void execute(Database.BatchableContext context, List<SObject> scope) {} public virtual void finish(Database.BatchableContext context) {} }",
		"TwoGenSub.cls": "public class TwoGenSub extends TwoGen implements Database.Batchable<Account> { public void execute(Database.BatchableContext context, List<Account> scope) {} }",
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			t.Run("named", func(t *testing.T) {
				result := analyzeDeclarationProjectWithAPIVersion(t, files, api)
				for _, d := range result.Diagnostics {
					if strings.Contains(d.Message, "Generic Interface already implemented") {
						t.Fatalf("project superclass hierarchy has no native duplicate-generic row; diagnostics=%+v", result.Diagnostics)
					}
				}
			})
			t.Run("anonymous", func(t *testing.T) {
				root := t.TempDir()
				paths := make([]string, 0, len(files))
				for name, contents := range files {
					path := filepath.Join(root, name)
					writeSemaFile(t, path, contents)
					paths = append(paths, path)
				}
				index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: paths}, schema.Schema{})
				result := AnalyzeAnonymous(index, "Database.executeBatch(new TwoGenSub());", api)
				if result.HasErrors() {
					t.Fatalf("anonymous use of the project hierarchy compiles; diagnostics=%+v", result.Diagnostics)
				}
			})
		})
	}
}
